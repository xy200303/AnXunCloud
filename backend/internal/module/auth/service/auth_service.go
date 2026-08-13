// Package service 认证业务逻辑：登录、登出、刷新、用户信息、动态路由。
package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/auth/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/storage"
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
	store  *storage.Storage                // 签名图 file_key → URL
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
		s.writeLoginLog(nil, req.Username, ip, ua, "fail", "登录过于频繁已锁定", channel)
		return nil, be
	}

	var user model.SysUser
	err := s.db.Where("username = ?", req.Username).First(&user).Error
	if err != nil || !password.Compare(user.Password, req.Password) {
		s.incrLoginFail(ctx, ip)
		s.writeLoginLog(nilIfErr(err, &user), req.Username, ip, ua, "fail", "用户名或密码错误", channel)
		return nil, errs.ErrBadCredentials
	}
	if user.Status != model.StatusEnabled {
		s.writeLoginLog(&user.ID, req.Username, ip, ua, "fail", "账号已停用", channel)
		return nil, errs.ErrAccountDisabled
	}

	resp, be := s.issueTokens(ctx, &user, channel)
	if be != nil {
		return nil, be
	}
	// 登录成功：清除失败计数、更新最近登录时间
	s.rdb.Del(ctx, "limit:login:"+ip)
	now := time.Now()
	s.db.Model(&model.SysUser{}).Where("id = ?", user.ID).Update("last_login_at", now)
	s.writeLoginLog(&user.ID, req.Username, ip, ua, "success", "登录成功", channel)
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
	var count int64
	s.db.Model(&model.SysUser{}).Where("username = ?", req.Username).Count(&count)
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
		Username: req.Username,
		Password: hash,
		Name:     req.Name,
		Phone:    req.Phone,
		UserType: "admin", // 后台用户
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
	if err := s.db.Select("id", "username", "name", "phone", "avatar", "openid", "is_builtin", "role_ids", "community_ids", "last_login_at", "created_at").
		First(&user, "id = ?", identity.UserID).Error; err != nil {
		return nil, errs.ErrUnauthorized
	}
	resp := &dto.InfoResp{
		ID:           user.ID,
		Username:     user.Username,
		Name:         user.Name,
		Phone:        user.Phone,
		Avatar:       user.Avatar,
		CommunityIDs: user.CommunityIDs,
		DataScope:    model.ScopeCustom,
		Roles:        []dto.RoleBrief{},
		Perms:        []string{},
	}
	// 签名取当前 active 签章资产（sign_asset 表）
	var sigAsset model.SignAsset
	if err := s.db.Select("file_key").
		Where("asset_type = ? AND owner_id = ? AND status = ?",
			model.SignAssetTypeUserSignature, user.ID, model.SignAssetStatusActive).
		First(&sigAsset).Error; err == nil && sigAsset.FileKey != "" && s.store != nil {
		resp.SignatureURL = s.store.URL(sigAsset.FileKey)
	}
	if identity.DataScopeAll {
		resp.DataScope = model.ScopeAll
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
func (s *AuthService) writeLoginLog(userID *string, username, ip, ua, status, msg, channel string) {
	rec := model.SysLoginLog{
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

// nilIfErr 用户查询失败时返回 nil（登录日志 user_id 可空）。
func nilIfErr(err error, user *model.SysUser) *string {
	if err != nil {
		return nil
	}
	return &user.ID
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
