// Package service 认证业务逻辑：登录、登出、刷新、用户信息、动态路由。
package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/auth/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/uploadfile"
)

// ChannelAdmin 后台会话渠道。
const ChannelAdmin = "admin"

// ChannelApp 移动 APP（Android/iOS/鸿蒙）会话渠道。
const ChannelApp = "app"

// AuthService 认证服务。
type AuthService struct {
	db     *gorm.DB
	rdb    *redis.Client
	sess   *session.Store
	jwtm   *jwtutil.Manager
	getCfg func(key string) (string, bool) // 读取系统参数（config:all 缓存）
	store  *storage.Storage                // 签名图存储键 → URL
}

// NewAuthService 构造认证服务。
func NewAuthService(db *gorm.DB, rdb *redis.Client, sess *session.Store, jwtm *jwtutil.Manager, getCfg func(string) (string, bool), store *storage.Storage) *AuthService {
	return &AuthService{db: db, rdb: rdb, sess: sess, jwtm: jwtm, getCfg: getCfg, store: store}
}

// Login 后台账号密码登录，签发双令牌并建立 Redis 会话。
func (s *AuthService) Login(ctx context.Context, req *dto.LoginReq, ip, ua string) (*dto.TokenResp, *errs.Error) {
	return s.LoginChannel(ctx, req, ChannelAdmin, ip, ua)
}

// LoginChannel 账号密码登录（按渠道建立会话/写日志；APP 端走 ChannelApp）。
func (s *AuthService) LoginChannel(ctx context.Context, req *dto.LoginReq, channel, ip, ua string) (*dto.TokenResp, *errs.Error) {
	// 登录限流：连续失败达 security.login_fail_limit 锁定 10 分钟
	if be := s.checkLoginLimit(ctx, ip); be != nil {
		s.writeLoginLog(nil, nil, req.Username, ip, ua, "fail", "登录过于频繁已锁定", channel)
		return nil, be
	}

	var users []model.SysUser
	s.db.Where("username = ?", req.Username).Find(&users)
	user, be := s.resolveLoginUser(users, req)
	if be != nil {
		// 40109：密码已校验通过、仅需选择公司——不计失败、不写失败日志，候选列表随错误下发
		if be.Code == errs.ErrTenantCodeRequired.Code {
			return nil, be
		}
		s.incrLoginFail(ctx, ip)
		s.writeLoginLog(nil, nil, req.Username, ip, ua, "fail", be.Msg, channel)
		return nil, be
	}
	if user.Status != model.StatusEnabled {
		s.writeLoginLog(&user.ID, &user.TenantID, req.Username, ip, ua, "fail", "账号已停用", channel)
		return nil, errs.ErrAccountDisabled
	}
	// 租户停用拒绝登录（每请求校验在 buildIdentity，登录侧先给明确提示）
	enabled, err := middleware.TenantEnabled(s.db, user.TenantID)
	if err != nil {
		return nil, errs.ErrInternal
	}
	if !enabled {
		s.writeLoginLog(&user.ID, &user.TenantID, req.Username, ip, ua, "fail", "租户已停用", channel)
		return nil, errs.ErrTenantDisabled
	}

	resp, be := s.issueTokens(ctx, user, channel)
	if be != nil {
		return nil, be
	}
	// 登录成功：清除失败计数、更新最近登录时间
	s.rdb.Del(ctx, "limit:login:"+ip)
	now := time.Now()
	s.db.Model(&model.SysUser{}).Where("id = ?", user.ID).Update("last_login_at", now)
	s.writeLoginLog(&user.ID, &user.TenantID, req.Username, ip, ua, "success", "登录成功", channel)
	return resp, nil
}

// RegisterConfig 开放注册开关状态（免登录）。
func (s *AuthService) RegisterConfig() gin.H {
	enabled := false
	if s.getCfg != nil {
		if v, ok := s.getCfg("auth.register_enabled"); ok {
			enabled = v == "true"
		}
	}
	return gin.H{"enabled": enabled}
}

// RegisterTenants 注册下拉公司列表（免登录）：仅注册开启时返回启用租户（code+name）。
// 注册场景租户名单本就对注册者公开；关闭注册时不暴露（40303）。
func (s *AuthService) RegisterTenants() ([]gin.H, *errs.Error) {
	if !s.RegisterConfig()["enabled"].(bool) {
		return nil, errs.ErrRegisterDisabled
	}
	var tenants []model.Tenant
	s.db.Select("code", "name").Where("status = ?", model.StatusEnabled).Order("created_at ASC").Find(&tenants)
	items := make([]gin.H, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, gin.H{"code": t.Code, "name": t.Name})
	}
	return items, nil
}

// Register 开放注册（免登录，受 auth.register_enabled 开关控制）。
// 注册成功的用户不绑定任何角色（登录后无菜单，仅可访问登录即可的接口）。
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterReq, ip string) *errs.Error {
	if be := s.checkLoginLimit(ctx, ip); be != nil {
		return be
	}
	if !s.RegisterConfig()["enabled"].(bool) {
		return errs.ErrRegisterDisabled
	}
	if !password.ValidUsernameRange(req.Username, 4, 20) {
		s.incrLoginFail(ctx, ip)
		return errs.ErrParam.WithMsg("username 须为 4–20 位字母数字下划线")
	}
	if !password.ValidPassword(req.Password) {
		s.incrLoginFail(ctx, ip)
		return errs.ErrParam.WithMsg("password 须为 8–32 位且含字母与数字")
	}
	if !password.ValidPhone(req.Phone) {
		s.incrLoginFail(ctx, ip)
		return errs.ErrParam.WithMsg("phone 手机号格式错误")
	}
	// 注册目标租户：下拉选择的公司（tenant_code），缺省 = 默认租户（私有化单租户场景）
	tenantID, be := s.defaultTenantID()
	if be != nil {
		return be
	}
	if code := strings.TrimSpace(req.TenantCode); code != "" {
		var t model.Tenant
		if err := s.db.Where("code = ? AND status = ?", code, model.StatusEnabled).First(&t).Error; err != nil {
			s.incrLoginFail(ctx, ip)
			return errs.ErrParam.WithMsg("所选公司不存在或已停用")
		}
		tenantID = t.ID
	}
	var count int64
	// 用户名租户内唯一（P3）；手机号保持全局唯一（小程序按手机号绑定的消歧前提）
	s.db.Model(&model.SysUser{}).Where("tenant_id = ? AND username = ?", tenantID, req.Username).Count(&count)
	if count > 0 {
		s.incrLoginFail(ctx, ip)
		return errs.ErrUsernameExists
	}
	s.db.Model(&model.SysUser{}).Where("phone = ?", req.Phone).Count(&count)
	if count > 0 {
		s.incrLoginFail(ctx, ip)
		return errs.ErrPhoneExists
	}
	hash, err := password.Hash(req.Password)
	if err != nil {
		return errs.ErrInternal
	}
	user := model.SysUser{
		TenantID: tenantID,
		Username: req.Username,
		Password: hash,
		Name:     req.Name,
		Phone:    req.Phone,
		Status:   model.StatusEnabled,
		Remark:   "开放注册",
	}
	if err := s.db.Create(&user).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Refresh 用 refresh token 滚动换新双令牌（后台渠道）。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*dto.TokenResp, *errs.Error) {
	return s.RefreshChannel(ctx, ChannelAdmin, refreshToken)
}

// RefreshChannel 按渠道滚动换新双令牌。
func (s *AuthService) RefreshChannel(ctx context.Context, channel, refreshToken string) (*dto.TokenResp, *errs.Error) {
	claims, err := s.jwtm.Parse(refreshToken)
	if err != nil || claims.Type != jwtutil.TypeRefresh {
		return nil, errs.ErrRefreshInvalid
	}
	black, _ := s.sess.IsBlacklisted(ctx, claims.ID)
	if black {
		return nil, errs.ErrRefreshInvalid
	}
	sessInfo, err := s.sess.GetByRefresh(ctx, channel, claims.UserID, claims.ID)
	if err != nil || sessInfo == nil {
		return nil, errs.ErrRefreshInvalid
	}
	var user model.SysUser
	if err := s.db.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil, errs.ErrRefreshInvalid
	}
	if user.Status != model.StatusEnabled {
		return nil, errs.ErrAccountDisabled
	}
	// 旧 refresh 作废（滚动刷新）
	s.sess.Blacklist(ctx, claims.ID, time.Until(claims.ExpiresAt.Time))
	return s.issueTokens(ctx, &user, channel)
}

// Logout 注销会话：双 token 加入黑名单并删除会话（后台渠道）。
func (s *AuthService) Logout(ctx context.Context, identity *middleware.Identity, accessExp time.Time) *errs.Error {
	return s.LogoutChannel(ctx, identity, accessExp, ChannelAdmin)
}

// LogoutChannel 按渠道注销会话（只退出当前登录点，其他端会话不受影响）。
func (s *AuthService) LogoutChannel(ctx context.Context, identity *middleware.Identity, accessExp time.Time, channel string) *errs.Error {
	sessInfo, err := s.sess.Get(ctx, channel, identity.UserID, identity.JTI)
	if err == nil && sessInfo != nil {
		s.sess.Blacklist(ctx, sessInfo.RefreshID, s.jwtm.RefreshTTL())
	}
	s.sess.Blacklist(ctx, identity.JTI, time.Until(accessExp))
	return wrapErr(s.sess.Delete(ctx, channel, identity.UserID, identity.JTI))
}

// KillUserSessions 使用户全部端全部登录点会话失效（重置密码/停用/删除时调用）。
func (s *AuthService) KillUserSessions(ctx context.Context, userID string) {
	for _, channel := range []string{ChannelAdmin, ChannelApp, "mp"} {
		infos, err := s.sess.List(ctx, channel, userID)
		if err != nil {
			continue
		}
		for _, info := range infos {
			s.sess.Blacklist(ctx, info.TokenID, s.jwtm.AccessTTL())
			s.sess.Blacklist(ctx, info.RefreshID, s.jwtm.RefreshTTL())
		}
		s.sess.DeleteAll(ctx, channel, userID)
	}
}

// Info 当前用户信息 + 权限点集合。
func (s *AuthService) Info(identity *middleware.Identity) (*dto.InfoResp, *errs.Error) {
	var user model.SysUser
	if err := s.db.Select("id", "username", "name", "phone", "avatar", "openid", "is_builtin", "role_ids", "tenant_id", "last_login_at", "created_at").
		First(&user, "id = ?", identity.UserID).Error; err != nil {
		return nil, errs.ErrUnauthorized
	}
	// 所属项目由 project_staff 在职编制推导
	projectIDs, err := middleware.StaffProjectIDs(s.db, user.ID)
	if err != nil {
		return nil, errs.ErrInternal
	}
	projects := make([]dto.ProjectBrief, 0, len(projectIDs))
	for _, pid := range projectIDs {
		projects = append(projects, dto.ProjectBrief{ID: pid})
	}
	resp := &dto.InfoResp{
		ID:        user.ID,
		Username:  user.Username,
		Name:      user.Name,
		Phone:     user.Phone,
		Avatar:    user.Avatar,
		Projects:  projects,
		DataScope: model.ScopeProject,
		Roles:     []dto.RoleBrief{},
		Perms:     []string{},
	}
	// 所属公司（租户名）与在职编制（小区 + 岗位名），个人中心展示用
	if user.TenantID != "" {
		var t model.Tenant
		if err := s.db.Select("name").First(&t, "id = ?", user.TenantID).Error; err == nil {
			resp.TenantName = t.Name
		}
	}
	resp.Staffs = s.staffBriefs(user)
	// 签名取当前 active 签章资产（sign_asset 表，file_id → 存储路径解析 URL）
	var sigAsset model.SignAsset
	if err := s.db.Select("file_id").
		Where("asset_type = ? AND owner_id = ? AND status = ?",
			model.SignAssetTypeUserSignature, user.ID, model.SignAssetStatusActive).
		First(&sigAsset).Error; err == nil && sigAsset.FileID != "" && s.store != nil {
		if f, ferr := uploadfile.ByID(s.db, sigAsset.FileID); ferr == nil {
			resp.SignatureURL = s.store.URL(f.StorageKey)
		}
	}
	if identity.DataScopeAll {
		resp.DataScope = model.ScopeAll
	} else if identity.ScopeSelf {
		resp.DataScope = model.ScopeSelf
	}
	if user.Openid != nil {
		resp.Openid = *user.Openid
	}
	resp.IsBuiltin = user.IsBuiltin
	resp.CreatedAt = user.CreatedAt.Format("2006-01-02 15:04:05")
	if user.LastLoginAt != nil {
		resp.LastLoginAt = user.LastLoginAt.Format("2006-01-02 15:04:05")
	}
	// 最近一次成功登录的 IP（个人中心安全信息展示）
	var lastLog model.SysLoginLog
	if err := s.db.Select("ip").Where("user_id = ? AND status = ?", user.ID, "success").
		Order("created_at DESC").First(&lastLog).Error; err == nil {
		resp.LastLoginIP = lastLog.IP
	}
	if len(user.RoleIDs) > 0 {
		var roles []model.SysRole
		s.db.Select("id", "code", "name").Where("id IN ? AND status = ?", []string(user.RoleIDs), model.StatusEnabled).Find(&roles)
		for _, r := range roles {
			resp.Roles = append(resp.Roles, dto.RoleBrief{ID: r.ID, Code: r.Code, Name: r.Name})
		}
	}
	for p := range identity.Perms {
		resp.Perms = append(resp.Perms, p)
	}
	sort.Strings(resp.Perms)
	return resp, nil
}

// Routes 按角色下发菜单树（仅 dir/menu 类型）。
func (s *AuthService) Routes(identity *middleware.Identity) ([]dto.RouteNode, *errs.Error) {
	q := s.db.Model(&model.SysMenu{}).
		Where("type IN ? AND status = ? AND visible = ?", []string{model.MenuTypeDir, model.MenuTypeMenu}, model.StatusEnabled, true).
		Order("sort ASC")
	if !identity.SuperAdmin {
		if len(identity.Perms) == 0 && len(identity.RoleCodes) == 0 {
			return []dto.RouteNode{}, nil
		}
		// 目录无 perms，需按角色关联的菜单 id 反查父链；这里直接取角色可见菜单全集再按类型过滤
		var roleIDs []string
		s.db.Model(&model.SysRole{}).Where("code IN ? AND status = ?", identity.RoleCodes, model.StatusEnabled).Pluck("id", &roleIDs)
		if len(roleIDs) == 0 {
			return []dto.RouteNode{}, nil
		}
		q = q.Where("id IN (?)",
			s.db.Model(&model.SysRoleMenu{}).Select("menu_id").Where("role_id IN ?", roleIDs))
	}
	var menus []model.SysMenu
	if err := q.Find(&menus).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 目录可能未被直接分配，补全已选菜单的祖先目录
	menus = s.withAncestors(menus)
	if !identity.SuperAdmin {
		// 平台级菜单（is_platform：平台管理目录整棵子树）不下发非超管，
		// 兜底存量 role_menu 中可能残留的平台菜单绑定
		kept := make([]model.SysMenu, 0, len(menus))
		for _, m := range menus {
			if !m.IsPlatform {
				kept = append(kept, m)
			}
		}
		menus = kept
	}
	return buildRouteTree(menus, ""), nil
}

// withAncestors 补齐菜单的祖先节点（保证树完整）。
func (s *AuthService) withAncestors(menus []model.SysMenu) []model.SysMenu {
	seen := map[string]bool{}
	for _, m := range menus {
		seen[m.ID] = true
	}
	missing := []string{}
	var byID []model.SysMenu
	s.db.Select("id", "parent_id").Where("status = ?", model.StatusEnabled).Find(&byID)
	parent := map[string]string{}
	for _, m := range byID {
		parent[m.ID] = m.ParentIDStr()
	}
	for _, m := range menus {
		for pid := m.ParentIDStr(); pid != "" && !seen[pid]; pid = parent[pid] {
			seen[pid] = true
			missing = append(missing, pid)
		}
	}
	if len(missing) > 0 {
		var ancestors []model.SysMenu
		s.db.Where("id IN ? AND type IN ? AND status = ? AND visible = ?",
			missing, []string{model.MenuTypeDir, model.MenuTypeMenu}, model.StatusEnabled, true).Find(&ancestors)
		menus = append(menus, ancestors...)
	}
	// 按 sort 重排
	sort.Slice(menus, func(i, j int) bool { return menus[i].Sort < menus[j].Sort })
	return menus
}

// buildRouteTree 按 parent_id 分组后递归组装菜单树，整体 O(n)。
func buildRouteTree(menus []model.SysMenu, parentID string) []dto.RouteNode {
	byParent := make(map[string][]model.SysMenu, len(menus))
	for _, m := range menus {
		byParent[m.ParentIDStr()] = append(byParent[m.ParentIDStr()], m)
	}
	var build func(pid string) []dto.RouteNode
	build = func(pid string) []dto.RouteNode {
		nodes := []dto.RouteNode{}
		for _, m := range byParent[pid] {
			nodes = append(nodes, dto.RouteNode{
				ID:       m.ID,
				ParentID: m.ParentIDStr(),
				Title:    m.Title,
				Path:     m.Path,
				Icon:     m.Icon,
				Type:     m.Type,
				Sort:     m.Sort,
				Children: build(m.ID),
			})
		}
		return nodes
	}
	return build(parentID)
}

// issueTokens 签发双令牌并刷新会话。
func (s *AuthService) issueTokens(ctx context.Context, user *model.SysUser, channel string) (*dto.TokenResp, *errs.Error) {
	access, accessJTI, err := s.jwtm.Generate(user.ID, user.Username, jwtutil.TypeAccess)
	if err != nil {
		return nil, errs.ErrInternal
	}
	refresh, refreshJTI, err := s.jwtm.Generate(user.ID, user.Username, jwtutil.TypeRefresh)
	if err != nil {
		return nil, errs.ErrInternal
	}
	var roles []string
	if len(user.RoleIDs) > 0 {
		s.db.Model(&model.SysRole{}).Where("id IN ?", []string(user.RoleIDs)).Pluck("code", &roles)
	}
	err = s.sess.Save(ctx, channel, user.ID, session.Info{
		TokenID:   accessJTI,
		RefreshID: refreshJTI,
		Name:      user.Name,
		Roles:     strings.Join(roles, ","),
		LoginAt:   time.Now().Format("2006-01-02 15:04:05"),
	}, s.jwtm.RefreshTTL())
	if err != nil {
		return nil, errs.ErrInternal
	}
	return &dto.TokenResp{
		TokenType:    "Bearer",
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwtm.AccessTTL().Seconds()),
		User:         &dto.UserBrief{MustChangePassword: user.MustChangePassword},
	}, nil
}

// checkLoginLimit 判断 IP 是否因连续失败被锁定。
func (s *AuthService) checkLoginLimit(ctx context.Context, ip string) *errs.Error {
	limit := s.loginFailLimit()
	n, err := s.rdb.Get(ctx, "limit:login:"+ip).Int()
	if err == nil && n >= limit {
		return errs.ErrTooMany.WithMsg("登录失败次数过多，请 10 分钟后再试")
	}
	return nil
}

// incrLoginFail 累计失败次数（10 分钟窗口）。
func (s *AuthService) incrLoginFail(ctx context.Context, ip string) {
	key := "limit:login:" + ip
	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 10*time.Minute)
	pipe.Exec(ctx)
}

// loginFailLimit 读取锁定阈值（默认 5）。
func (s *AuthService) loginFailLimit() int {
	if s.getCfg != nil {
		if v, ok := s.getCfg("security.login_fail_limit"); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return n
			}
		}
	}
	return 5
}

// writeLoginLog 写登录日志（channel 区分后台/APP/小程序）。
// tenantID 取登录用户所属租户（日志管理按租户上下文过滤）；
// 无法识别用户（账号不存在/公司代码错误等）时归默认租户，与迁移 00023 存量回填口径一致。
func (s *AuthService) writeLoginLog(userID *string, tenantID *string, username, ip, ua, status, msg, channel string) {
	if tenantID == nil {
		if id, be := s.defaultTenantID(); be == nil {
			tenantID = &id
		}
	}
	rec := model.SysLoginLog{
		TenantID: tenantID,
		UserID:   userID,
		Username: username,
		Channel:  channel,
		IP:       ip,
		UA:       truncate(ua, 500),
		Status:   status,
		Msg:      msg,
	}
	s.db.Create(&rec)
}

// resolveLoginUser 登录用户解析（多租户，密码优先消歧）：
// username 全局查出候选账号后先用密码过滤——只有一个账号密码匹配则直接命中（用户无感）；
// 多个账号密码都匹配（同名同密码）时返回 40109 并携带候选公司列表（此时密码已验证通过，不泄露存在性），
// 前端下拉选择后带 tenant_code 重提；零匹配报通用"用户名或密码错误"。
func (s *AuthService) resolveLoginUser(users []model.SysUser, req *dto.LoginReq) (*model.SysUser, *errs.Error) {
	matched := make([]model.SysUser, 0, 1)
	for _, u := range users {
		if password.Compare(u.Password, req.Password) {
			matched = append(matched, u)
		}
	}
	if len(matched) == 0 {
		return nil, errs.ErrBadCredentials
	}
	if len(matched) == 1 {
		u := matched[0]
		return &u, nil
	}
	// 多账号同密码：tenant_code 选择或下发候选公司列表
	tenantCode := strings.TrimSpace(req.TenantCode)
	if tenantCode != "" {
		var t model.Tenant
		if err := s.db.Where("code = ?", tenantCode).First(&t).Error; err != nil {
			return nil, errs.ErrParam.WithMsg("所选公司不存在")
		}
		for _, u := range matched {
			if u.TenantID == t.ID {
				return &u, nil
			}
		}
		return nil, errs.ErrBadCredentials
	}
	tenants := s.tenantBriefsOf(matched)
	return nil, errs.ErrTenantCodeRequired.WithData(gin.H{"tenants": tenants})
}

// tenantBriefsOf 候选用户涉及租户的名称/代码（登录公司选择列表）。
func (s *AuthService) tenantBriefsOf(users []model.SysUser) []gin.H {
	ids := distinctTenantIDs(users)
	var tenants []model.Tenant
	s.db.Select("id", "code", "name").Where("id IN ? AND status = ?", ids, model.StatusEnabled).Find(&tenants)
	items := make([]gin.H, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, gin.H{"code": t.Code, "name": t.Name})
	}
	return items
}

// distinctTenantIDs 候选用户去重后的租户集合（纯函数，登录租户歧义判定用）。
func distinctTenantIDs(users []model.SysUser) []string {
	seen := map[string]bool{}
	var ids []string
	for _, u := range users {
		if !seen[u.TenantID] {
			seen[u.TenantID] = true
			ids = append(ids, u.TenantID)
		}
	}
	return ids
}

// filterUsersByTenant 按租户过滤候选用户（纯函数）。
func filterUsersByTenant(users []model.SysUser, tenantID string) []model.SysUser {
	out := make([]model.SysUser, 0, 1)
	for _, u := range users {
		if u.TenantID == tenantID {
			out = append(out, u)
		}
	}
	return out
}

// defaultTenantID 默认租户 ID（开放注册等无登录态场景归入默认租户）。
func (s *AuthService) defaultTenantID() (string, *errs.Error) {
	var id string
	if err := s.db.Model(&model.Tenant{}).Select("id").Where("code = ?", model.DefaultTenantCode).
		Limit(1).Pluck("id", &id).Error; err != nil || id == "" {
		return "", errs.ErrInternal
	}
	return id, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func wrapErr(err error) *errs.Error {
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// staffBriefs 在职编制明细：小区名 + 岗位名列表（个人中心展示用；岗位名按租户岗位字典优先、平台模板回落）。
func (s *AuthService) staffBriefs(user model.SysUser) []dto.StaffBrief {
	var rows []model.ProjectStaff
	if err := s.db.Where("user_id = ? AND status = ?", user.ID, model.StatusEnabled).
		Order("created_at ASC").Find(&rows).Error; err != nil || len(rows) == 0 {
		return []dto.StaffBrief{}
	}
	// 小区名
	commIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		commIDs = append(commIDs, r.ProjectID)
	}
	commNames := map[string]string{}
	var comms []model.Community
	if err := s.db.Select("id", "name").Where("id IN ?", commIDs).Find(&comms).Error; err == nil {
		for _, cm := range comms {
			commNames[cm.ID] = cm.Name
		}
	}
	// 岗位 code → 名称（租户字典优先，平台模板回落）
	codes := map[string]bool{}
	for _, r := range rows {
		for _, c := range r.Posts {
			codes[c] = true
		}
	}
	postNames := map[string]string{}
	if len(codes) > 0 {
		codeList := make([]string, 0, len(codes))
		for c := range codes {
			codeList = append(codeList, c)
		}
		var posts []model.PostDict
		q := s.db.Select("code", "name").Where("code IN ?", codeList)
		if user.TenantID != "" {
			q = q.Where("tenant_id = ? OR tenant_id IS NULL", user.TenantID)
		}
		if err := q.Order("tenant_id ASC").Find(&posts).Error; err == nil {
			for _, p := range posts { // 租户行（tenant_id 非空）后扫到则覆盖平台模板名
				postNames[p.Code] = p.Name
			}
		}
	}
	out := make([]dto.StaffBrief, 0, len(rows))
	for _, r := range rows {
		names := make([]string, 0, len(r.Posts))
		for _, c := range r.Posts {
			if n, ok := postNames[c]; ok {
				names = append(names, n)
			} else {
				names = append(names, c)
			}
		}
		out = append(out, dto.StaffBrief{
			CommunityID:   r.ProjectID,
			CommunityName: commNames[r.ProjectID],
			PostNames:     names,
		})
	}
	return out
}
