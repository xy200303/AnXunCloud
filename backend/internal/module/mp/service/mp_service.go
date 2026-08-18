package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	authsvc "anxuncloud/internal/module/auth/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	womodel "anxuncloud/internal/module/workorder/model"
	wodto "anxuncloud/internal/module/workorder/dto"
	wosvc "anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// ChannelMP 小程序端标记（仅用于登录日志/审计的客户端类型标识）。
// 会话通道自 v21 起与 App 合并为 ChannelApp：app/mp 共用一套会话体系，token 两端正通用。
const ChannelMP = "mp"

// MPService 小程序端服务（登录/任务/工单/消息/公告）。
type MPService struct {
	db     *gorm.DB
	rdb    *redis.Client
	sess   *session.Store
	jwtm   *jwtutil.Manager
	wechat config.WechatConfig
	orders *wosvc.OrderService
	httpc  *http.Client
}

func NewMPService(db *gorm.DB, rdb *redis.Client, sess *session.Store, jwtm *jwtutil.Manager, wechat config.WechatConfig, orders *wosvc.OrderService) *MPService {
	return &MPService{db: db, rdb: rdb, sess: sess, jwtm: jwtm, wechat: wechat, orders: orders, httpc: &http.Client{Timeout: 8 * time.Second}}
}

// ========== 登录 ==========

// Login 微信 code 换登录态。
// 【mock 模式】mock=true（仅 dev 可用）时，code 传 "mock:<手机号>"：
// 按手机号查找已开通账号，首次自动绑定伪 openid（mock-openid-<手机号>），仅用于开发联调。
func (s *MPService) Login(ctx context.Context, req *dto.MPLoginReq, ip, ua string) (gin.H, *errs.Error) {
	// IP 维度失败限流（与后台登录一致的安全基线，防爆破/手机号枚举）
	if be := s.checkLoginLimit(ctx, ip); be != nil {
		return nil, be
	}
	var user *sysmodel.SysUser
	if s.wechat.MockEnabled() {
		u, be := s.mockLogin(req.Code)
		if be != nil {
			s.incrLoginFail(ctx, ip)
			s.loginLog(nil, nil, "", ip, ua, "fail", be.Msg)
			return nil, be
		}
		user = u
	} else {
		u, be := s.wxLogin(ctx, req)
		if be != nil {
			s.incrLoginFail(ctx, ip)
			s.loginLog(nil, nil, "", ip, ua, "fail", be.Msg)
			return nil, be
		}
		user = u
	}
	if user.Status != sysmodel.StatusEnabled {
		s.incrLoginFail(ctx, ip)
		s.loginLog(&user.ID, &user.TenantID, user.Username, ip, ua, "fail", "账号已停用")
		return nil, errs.ErrAccountDisabled
	}
	// 租户停用拒绝登录（P3 多租户：与后台登录同一规则）
	enabled, err := middleware.TenantEnabled(s.db, user.TenantID)
	if err != nil {
		return nil, errs.ErrInternal
	}
	if !enabled {
		s.incrLoginFail(ctx, ip)
		s.loginLog(&user.ID, &user.TenantID, user.Username, ip, ua, "fail", "租户已停用")
		return nil, errs.ErrTenantDisabled
	}
	resp, be := s.issueTokens(ctx, user)
	if be != nil {
		return nil, be
	}
	s.rdb.Del(ctx, "limit:login:mp:"+ip)
	now := time.Now()
	s.db.Model(&sysmodel.SysUser{}).Where("id = ?", user.ID).Update("last_login_at", now)
	s.loginLog(&user.ID, &user.TenantID, user.Username, ip, ua, "success", "小程序登录成功")
	return resp, nil
}

// checkLoginLimit 小程序登录 IP 锁定（连续 10 次失败锁 10 分钟）。
func (s *MPService) checkLoginLimit(ctx context.Context, ip string) *errs.Error {
	n, err := s.rdb.Get(ctx, "limit:login:mp:"+ip).Int()
	if err == nil && n >= 10 {
		return errs.ErrTooMany.WithMsg("登录失败次数过多，请 10 分钟后再试")
	}
	return nil
}

// incrLoginFail 累计失败次数（10 分钟窗口）。
func (s *MPService) incrLoginFail(ctx context.Context, ip string) {
	key := "limit:login:mp:" + ip
	pipe := s.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 10*time.Minute)
	pipe.Exec(ctx)
}

// mockLogin mock 模式登录（开发联调专用）。
func (s *MPService) mockLogin(code string) (*sysmodel.SysUser, *errs.Error) {
	var phone string
	if _, err := fmt.Sscanf(code, "mock:%s", &phone); err != nil || phone == "" {
		return nil, errs.ErrWxCodeInvalid.WithMsg("mock 模式 code 格式应为 mock:<手机号>")
	}
	openid := "mock-openid-" + phone
	var user sysmodel.SysUser
	// 先按 openid 找（已绑定），再按手机号找（首次绑定）
	if err := s.db.Where("openid = ?", openid).First(&user).Error; err != nil {
		if err := s.db.Where("phone = ?", phone).First(&user).Error; err != nil {
			return nil, errs.ErrWxUnbound
		}
		if err := s.db.Model(&user).Update("openid", openid).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	return &user, nil
}

// wxLogin 真实模式：code2session 换 openid，未绑定时用 phone_code 绑定手机号账号。
func (s *MPService) wxLogin(ctx context.Context, req *dto.MPLoginReq) (*sysmodel.SysUser, *errs.Error) {
	openid, be := s.code2Session(req.Code)
	if be != nil {
		return nil, be
	}
	var user sysmodel.SysUser
	if err := s.db.Where("openid = ?", openid).First(&user).Error; err == nil {
		return &user, nil
	}
	// 未绑定：需要 phone_code 换手机号完成绑定
	if req.PhoneCode == "" {
		return nil, errs.ErrWxUnbound
	}
	phone, be := s.phoneByCode(ctx, req.PhoneCode)
	if be != nil {
		return nil, be
	}
	if err := s.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, errs.ErrWxUnbound
	}
	if err := s.db.Model(&user).Update("openid", openid).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &user, nil
}

// code2Session 调微信 jscode2session。
func (s *MPService) code2Session(code string) (string, *errs.Error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		s.wechat.AppID, s.wechat.Secret, code)
	resp, err := s.httpc.Get(url)
	if err != nil {
		return "", errs.ErrWxCodeInvalid
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var out struct {
		OpenID string `json:"openid"`
		ErrCode int   `json:"errcode"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.OpenID == "" || out.ErrCode != 0 {
		return "", errs.ErrWxCodeInvalid
	}
	return out.OpenID, nil
}

// phoneByCode getPhoneNumber 换手机号（access_token 缓存于 Redis）。
func (s *MPService) phoneByCode(ctx context.Context, phoneCode string) (string, *errs.Error) {
	token, be := s.wxAccessToken(ctx)
	if be != nil {
		return "", be
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=%s", token)
	resp, err := s.httpc.Post(url, "application/json", strings.NewReader(fmt.Sprintf(`{"code":"%s"}`, phoneCode)))
	if err != nil {
		return "", errs.ErrWxCodeInvalid
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var out struct {
		ErrCode   int `json:"errcode"`
		PhoneInfo struct {
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ErrCode != 0 || out.PhoneInfo.PurePhoneNumber == "" {
		return "", errs.ErrWxCodeInvalid.WithMsg("手机号获取失败")
	}
	return out.PhoneInfo.PurePhoneNumber, nil
}

// wxAccessToken 小程序全局 access_token（Redis 缓存 7000s）。
func (s *MPService) wxAccessToken(ctx context.Context) (string, *errs.Error) {
	const key = "cache:wx:access_token"
	if v, err := s.rdb.Get(ctx, key).Result(); err == nil && v != "" {
		return v, nil
	}
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		s.wechat.AppID, s.wechat.Secret)
	resp, err := s.httpc.Get(url)
	if err != nil {
		return "", errs.ErrInternal
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", errs.ErrInternal
	}
	ttl := time.Duration(out.ExpiresIn-200) * time.Second
	if ttl <= 0 {
		ttl = 7000 * time.Second
	}
	s.rdb.Set(ctx, key, out.AccessToken, ttl)
	return out.AccessToken, nil
}

// issueTokens 签发双令牌并写 mp 会话。
func (s *MPService) issueTokens(ctx context.Context, user *sysmodel.SysUser) (gin.H, *errs.Error) {
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
		s.db.Model(&sysmodel.SysRole{}).Where("id IN ?", []string(user.RoleIDs)).Pluck("code", &roles)
	}
	err = s.sess.Save(ctx, authsvc.ChannelApp, user.ID, session.Info{
		TokenID: accessJTI, RefreshID: refreshJTI, Name: user.Name,
		Roles: strings.Join(roles, ","), LoginAt: time.Now().Format(timefmt.Layout),
	}, s.jwtm.RefreshTTL())
	if err != nil {
		return nil, errs.ErrInternal
	}
	// 所属项目由 project_staff 在职编制推导
	projectIDs, _ := middleware.StaffProjectIDs(s.db, user.ID)
	projects := make([]gin.H, 0, len(projectIDs))
	for _, pid := range projectIDs {
		projects = append(projects, gin.H{"id": pid})
	}
	return gin.H{
		"token_type": "Bearer", "access_token": access, "refresh_token": refresh,
		"expires_in": int64(s.jwtm.AccessTTL().Seconds()),
		"user": gin.H{
			"id": user.ID, "name": user.Name, "avatar": user.Avatar,
			"roles": roles, "projects": projects,
		},
	}, nil
}

// Refresh 小程序端刷新双令牌（逻辑同后台）。
func (s *MPService) Refresh(ctx context.Context, refreshToken string) (gin.H, *errs.Error) {
	claims, err := s.jwtm.Parse(refreshToken)
	if err != nil || claims.Type != jwtutil.TypeRefresh {
		return nil, errs.ErrRefreshInvalid
	}
	black, _ := s.sess.IsBlacklisted(ctx, claims.ID)
	if black {
		return nil, errs.ErrRefreshInvalid
	}
	sessInfo, err := s.sess.GetByRefresh(ctx, authsvc.ChannelApp, claims.UserID, claims.ID)
	if err != nil || sessInfo == nil {
		return nil, errs.ErrRefreshInvalid
	}
	var user sysmodel.SysUser
	if err := s.db.First(&user, "id = ?", claims.UserID).Error; err != nil {
		return nil, errs.ErrRefreshInvalid
	}
	if user.Status != sysmodel.StatusEnabled {
		return nil, errs.ErrAccountDisabled
	}
	s.sess.Blacklist(ctx, claims.ID, time.Until(claims.ExpiresAt.Time))
	resp, be := s.issueTokens(ctx, &user)
	if be != nil {
		return nil, be
	}
	delete(resp, "user") // 刷新接口不下发 user
	return resp, nil
}

// loginLog 登录日志（channel=mp）。
// tenantID 取登录用户所属租户（日志管理按租户上下文过滤）；
// 无法识别用户时归默认租户，与后台 writeLoginLog 及迁移 00023 存量回填口径一致。
func (s *MPService) loginLog(userID *string, tenantID *string, username, ip, ua, status, msg string) {
	if tenantID == nil {
		var id string
		if err := s.db.Model(&sysmodel.Tenant{}).Select("id").Where("code = ?", sysmodel.DefaultTenantCode).
			Limit(1).Pluck("id", &id).Error; err == nil && id != "" {
			tenantID = &id
		}
	}
	rec := sysmodel.SysLoginLog{TenantID: tenantID, UserID: userID, Username: username, Channel: ChannelMP, IP: ip, Status: status, Msg: msg}
	if len(ua) > 500 {
		ua = ua[:500]
	}
	rec.UA = ua
	s.db.Create(&rec)
}

// ========== 任务 ==========

// PointByCode 扫码/NFC 定位：按二维码编号或 NFC ID 查点位，并匹配今日任务上下文（任务定位器，非凭证）。
func (s *MPService) PointByCode(inspectorID, code string) (gin.H, *errs.Error) {
	var pt insmodel.InspectionPoint
	// nfc_id 大小写不敏感（兼容存量小写录入；新写入库已统一大写）
	if err := s.db.Where("qrcode_no = ? OR UPPER(nfc_id) = ?", code, strings.ToUpper(strings.TrimSpace(code))).First(&pt).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	// 数据权限：非全量数据范围的用户须在该点位所在项目的编制内（任意岗位）
	var user sysmodel.SysUser
	if err := s.db.Select("id", "role_ids").First(&user, "id = ?", inspectorID).Error; err != nil {
		return nil, errs.ErrUnauthorized
	}
	var roles []sysmodel.SysRole
	if len(user.RoleIDs) > 0 {
		s.db.Select("id", "data_scope").Where("id IN ?", []string(user.RoleIDs)).Find(&roles)
	}
	scopeAll := false
	for _, r := range roles {
		if r.DataScope == sysmodel.ScopeAll {
			scopeAll = true
			break
		}
	}
	if !scopeAll {
		projectIDs, err := middleware.StaffProjectIDs(s.db, inspectorID)
		if err != nil {
			return nil, errs.ErrInternal
		}
		if !types.IDArray(projectIDs).Contains(pt.CommunityID) {
			return nil, errs.ErrDataScope
		}
	}
	// 今日任务上下文：我今日包含该点位的任务及打卡状态（多个任务取列表，客户端选最近未完成）
	today := time.Now().Format("2006-01-02")
	var tasks []insmodel.InspectionTask
	s.db.Where("inspector_id = ? AND task_date = ?", inspectorID, today).Find(&tasks)
	matched := make([]gin.H, 0, 1)
	for i := range tasks {
		t := &tasks[i]
		var plan insmodel.InspectionPlan
		if s.db.Unscoped().Select("id", "name", "point_ids").First(&plan, "id = ?", t.PlanID).Error != nil {
			continue
		}
		if !plan.PointIDs.Contains(pt.ID) {
			continue
		}
		checked := s.db.Where("task_id = ? AND point_id = ?", t.ID, pt.ID).
			First(&insmodel.CheckinRecord{}).Error == nil
		matched = append(matched, gin.H{"task_id": t.ID, "plan_name": plan.Name, "status": t.Status, "checked": checked})
	}
	buildingName := ""
	if pt.BuildingID != nil {
		var b insmodel.Building
		if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
			buildingName = b.Name
		}
	}
	return gin.H{
		"point": gin.H{
			"id": pt.ID, "name": pt.Name, "qrcode_no": pt.QRCodeNo, "nfc_id": pt.NfcID,
			"community_id": pt.CommunityID, "community_name": s.commName(pt.CommunityID),
			"building_name": buildingName,
			"credential": pt.Credential, "require_fence": pt.RequireFence,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
		},
		"tasks": matched,
	}, nil
}

// Points 项目启用点位列表（问题上报关联点位用）。
// 数据权限：须为该项目在职编制成员（任意岗位），与 OrderReport 同一规则。
func (s *MPService) Points(userID, communityID string) ([]gin.H, *errs.Error) {
	if communityID == "" {
		return nil, errs.ErrParam.WithMsg("community_id 不能为空")
	}
	var staffCount int64
	s.db.Model(&sysmodel.ProjectStaff{}).
		Where("project_id = ? AND user_id = ? AND status = ?", communityID, userID, sysmodel.StatusEnabled).Count(&staffCount)
	if staffCount == 0 {
		return nil, errs.ErrDataScope
	}
	var points []insmodel.InspectionPoint
	if err := s.db.Select("id", "name", "building_id").
		Where("community_id = ? AND status = ?", communityID, sysmodel.StatusEnabled).
		Order("sort ASC, created_at ASC").Find(&points).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 楼栋名称批量取，避免逐点查询
	buildingNames := map[string]string{}
	var buildings []insmodel.Building
	s.db.Select("id", "name").Where("community_id = ?", communityID).Find(&buildings)
	for _, b := range buildings {
		buildingNames[b.ID] = b.Name
	}
	items := make([]gin.H, 0, len(points))
	for _, p := range points {
		name := p.Name
		if p.BuildingID != nil {
			if bn := buildingNames[*p.BuildingID]; bn != "" {
				name = bn + " · " + p.Name
			}
		}
		items = append(items, gin.H{"id": p.ID, "name": name})
	}
	return items, nil
}

// PublicPoint 短链接公开点位摘要（免登录：NFC 贴卡/扫码打开 H5 信息页的数据源）。
// 脱敏原则：仅点位基础信息 + 巡检结果摘要，不出坐标、照片、凭证配置等敏感项。
func (s *MPService) PublicPoint(code string) (gin.H, *errs.Error) {
	var pt insmodel.InspectionPoint
	if err := s.db.Where("qrcode_no = ? OR UPPER(nfc_id) = ?", code, strings.ToUpper(strings.TrimSpace(code))).First(&pt).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	if pt.Status != "enabled" {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	buildingName := ""
	if pt.BuildingID != nil {
		var b insmodel.Building
		if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
			buildingName = b.Name
		}
	}
	// 近 30 天巡检概况
	since := time.Now().AddDate(0, 0, -30)
	var total30, abnormal30 int64
	s.db.Model(&insmodel.CheckinRecord{}).Where("point_id = ? AND checkin_time >= ?", pt.ID, since).Count(&total30)
	s.db.Model(&insmodel.CheckinRecord{}).
		Where("point_id = ? AND checkin_time >= ? AND result = ?", pt.ID, since, insmodel.ResultAbnormal).Count(&abnormal30)
	// 最近 5 条巡检记录
	var recs []insmodel.CheckinRecord
	s.db.Where("point_id = ?", pt.ID).Order("checkin_time DESC").Limit(5).Find(&recs)
	recent := make([]gin.H, 0, len(recs))
	for i := range recs {
		r := &recs[i]
		name := ""
		var u sysmodel.SysUser
		if s.db.Select("name").First(&u, "id = ?", r.InspectorID).Error == nil {
			name = maskName(u.Name) // 公开页脱敏：不对外暴露员工完整姓名
		}
		recent = append(recent, gin.H{
			"checkin_time": r.CheckinTime.Format("2006-01-02 15:04"),
			"result":       r.Result,
			"checkin_type": r.CheckinType,
			"inspector":    name,
		})
	}
	return gin.H{
		"point": gin.H{
			"name": pt.Name, "qrcode_no": pt.QRCodeNo, "type": pt.Type,
			"community_name": s.commName(pt.CommunityID), "building_name": buildingName,
		},
		"stats":  gin.H{"total_30d": total30, "abnormal_30d": abnormal30},
		"recent": recent,
	}, nil
}

// maskName 姓名脱敏：保留首字（公开点位页等免登录场景不暴露员工完整姓名）。
func maskName(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) <= 1 {
		return name
	}
	return string(r[0]) + "*"
}

// TodayTasks 今日任务列表 + 总进度（进行中排最前）。
func (s *MPService) TodayTasks(inspectorID string) (gin.H, *errs.Error) {
	today := time.Now().Format("2006-01-02")
	var tasks []insmodel.InspectionTask
	if err := s.db.Where("inspector_id = ? AND task_date = ?", inspectorID, today).
		Order("CASE WHEN status = 'doing' THEN 0 ELSE 1 END, id ASC").Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(tasks))
	totalPts, donePts := 0, 0
	for i := range tasks {
		t := &tasks[i]
		totalPts += t.TotalPoints
		donePts += t.DonePoints
		var plan insmodel.InspectionPlan
		planName, timeWindow := "", ""
		if s.db.Unscoped().Select("name", "time_window", "deleted_at").First(&plan, "id = ?", t.PlanID).Error == nil {
			planName, timeWindow = plan.Name, plan.TimeWindow
			if plan.DeletedAt.Valid {
				planName += "（已删除）"
			}
		}
		items = append(items, gin.H{
			"id": t.ID, "plan_name": planName, "community_name": s.commName(t.CommunityID),
			"patrol_type": t.PatrolType, // 巡查类型透出，app 端按类型分组展示
			"task_date": today, "time_window": timeWindow, "status": t.Status,
			"total_points": t.TotalPoints, "done_points": t.DonePoints,
			"progress": progressOf(t.DonePoints, t.TotalPoints),
			"started_at": timefmt.TP(t.StartedAt),
		})
	}
	return gin.H{
		"date": today, "total_points": totalPts, "done_points": donePts,
		"progress": progressOf(donePts, totalPts), "tasks": items,
	}, nil
}

// TaskDetail 任务详情（点位路线 + 打卡配置 + 我的打卡状态；仅归属巡检员可见）。
func (s *MPService) TaskDetail(inspectorID, taskID string) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	// Unscoped：计划已删除时执行中的任务仍应可读可打卡（删计划只级联清理未开始任务）
	var plan insmodel.InspectionPlan
	if err := s.db.Unscoped().First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var checkins []insmodel.CheckinRecord
	s.db.Where("task_id = ?", task.ID).Find(&checkins)
	byPoint := map[string]*insmodel.CheckinRecord{}
	for i := range checkins {
		byPoint[checkins[i].PointID] = &checkins[i]
	}
	points := make([]gin.H, 0, len(plan.PointIDs))
	ptTpl := map[int]string{} // points 下标 → 点位绑定的检查项模板 ID（仅非空）
	for i, pid := range plan.PointIDs {
		var pt insmodel.InspectionPoint
		if s.db.First(&pt, "id = ?", pid).Error != nil {
			continue
		}
		buildingName := ""
		if pt.BuildingID != nil {
			var b insmodel.Building
			if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
				buildingName = b.Name
			}
		}
		var myCheckin any
		if ck, ok := byPoint[pid]; ok {
			dist := any(nil)
			if ck.DistanceToPoint != nil {
				dist = int(*ck.DistanceToPoint)
			}
			myCheckin = gin.H{
				"id": ck.ID, "checkin_time": timefmt.T(ck.CheckinTime),
				"checkin_type": ck.CheckinType, "distance_to_point": dist,
				"result": ck.Result, "is_suspect": ck.IsSuspect,
			}
		}
		points = append(points, gin.H{
			"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
			"sort": i + 1, "credential": pt.Credential, "require_fence": pt.RequireFence, "qrcode_no": pt.QRCodeNo,
			"nfc_id": pt.NfcID,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
			"required_photo_items": pt.RequiredPhotoItems, "my_checkin": myCheckin,
		})
		if pt.TemplateID != nil && *pt.TemplateID != "" {
			ptTpl[len(points)-1] = *pt.TemplateID
		}
	}
	// 批量查检查项模板项：收集全部非空 TemplateID 一次 IN 查询（禁止循环单查），按 template_id 分组
	tplItems := map[string][]gin.H{}
	if len(ptTpl) > 0 {
		tplIDs := make([]string, 0, len(ptTpl))
		for _, tid := range ptTpl {
			tplIDs = append(tplIDs, tid)
		}
		var items []insmodel.CheckTemplateItem
		s.db.Where("template_id IN ?", tplIDs).Order("sort ASC").Find(&items)
		for i := range items {
			it := &items[i]
			requirement := ""
			if it.Requirement != nil {
				requirement = *it.Requirement
			}
			tplItems[it.TemplateID] = append(tplItems[it.TemplateID], gin.H{
				"name": it.Name, "requirement": requirement, "photo_required": it.PhotoRequired,
			})
		}
	}
	for idx := range points {
		ci := tplItems[ptTpl[idx]]
		if ci == nil {
			ci = []gin.H{} // 无模板（或模板无项）输出空数组而非 null
		}
		points[idx]["check_items"] = ci
	}
	return gin.H{
		"id": task.ID, "plan_name": plan.Name, "community_name": s.commName(task.CommunityID),
		"patrol_type": task.PatrolType, // 巡查类型透出，app 端按类型分组展示
		"task_date": task.TaskDate.Format("2006-01-02"), "time_window": plan.TimeWindow,
		"status": task.Status, "total_points": task.TotalPoints, "done_points": task.DonePoints,
		"progress": progressOf(task.DonePoints, task.TotalPoints), "points": points,
	}, nil
}

// ========== 我的工单 / 消息 ==========

// MyOrders 我的工单（我上报的 + 指派给我的；type=pool 为可抢工单池）。
func (s *MPService) MyOrders(userID string, q *dto.MyOrdersQuery) (*response.Page, *errs.Error) {
	if q.Type == "pool" {
		// 可抢池：开启抢单项目 + 本人在 order_accept 名单的待派单工单（名单制判定在 OrderService 内）
		return s.orders.PoolOrders(userID, &wodto.OrderListQuery{PageQuery: q.PageQuery, Status: q.Status})
	}
	db := s.db.Model(&womodel.WorkOrder{})
	switch q.Type {
	case "reported":
		db = db.Where("reporter_id = ?", userID)
	case "assigned":
		db = db.Where("assignee_id = ?", userID)
	default:
		db = db.Where("reporter_id = ? OR assignee_id = ?", userID, userID)
	}
	if q.Status != "" {
		// 支持逗号多值（如 pending,assigned 合并为「待处理」筛选）
		db = db.Where("status IN ?", strings.Split(q.Status, ","))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []womodel.WorkOrder
	offset, limit := q.Normalize()
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, o := range rows {
		myRole := "reporter"
		if o.AssigneeID != nil && *o.AssigneeID == userID && o.ReporterID != userID {
			myRole = "assignee"
		}
		pointName := ""
		if o.PointID != nil {
			s.db.Table("inspection_point").Select("name").Where("id = ?", *o.PointID).Scan(&pointName)
		}
		list = append(list, gin.H{
			"id": o.ID, "order_no": o.OrderNo, "title": o.Title,
			"community_name": s.commName(o.CommunityID), "point_name": pointName,
			"priority": o.Priority, "status": o.Status, "my_role": myRole,
			"created_at": timefmt.T(o.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// MyOrderCounts 我的工单按状态计数（type 过滤口径与 MyOrders 一致），
// 供移动端状态筛选 chip 角标与「指派给我」红点使用。返回 {status: count}。
func (s *MPService) MyOrderCounts(userID string, typ string) (gin.H, *errs.Error) {
	db := s.db.Model(&womodel.WorkOrder{})
	switch typ {
	case "reported":
		db = db.Where("reporter_id = ?", userID)
	case "assigned":
		db = db.Where("assignee_id = ?", userID)
	default:
		db = db.Where("reporter_id = ? OR assignee_id = ?", userID, userID)
	}
	type statusCount struct {
		Status string
		Cnt    int64
	}
	var rows []statusCount
	if err := db.Select("status, COUNT(*) AS cnt").Group("status").Scan(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := gin.H{}
	for _, r := range rows {
		out[r.Status] = r.Cnt
	}
	// 可抢池数量（移动端「可抢」入口角标）
	out["pool"] = s.orders.PoolCount(userID)
	return out, nil
}

// Messages 消息列表 + 未读数。
func (s *MPService) Messages(userID string, q *dto.MessageQuery) (gin.H, *errs.Error) {
	db := s.db.Model(&sysmodel.SysMessage{}).Where("user_id = ?", userID)
	if q.Type != "" {
		db = db.Where("type = ?", q.Type)
	}
	if v, ok, _ := bind.BoolFilter(q.IsRead); ok {
		db = db.Where("is_read = ?", v)
	}
	var unread int64
	s.db.Model(&sysmodel.SysMessage{}).Where("user_id = ? AND is_read = false", userID).Count(&unread)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []sysmodel.SysMessage
	offset, limit := q.Normalize()
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, m := range rows {
		list = append(list, gin.H{
			"id": m.ID, "type": m.Type, "title": m.Title, "content": m.Content,
			"biz_id": m.BizID, "is_read": m.IsRead, "created_at": timefmt.T(m.CreatedAt),
		})
	}
	return gin.H{
		"unread_count": unread, "list": list,
		"total": total, "page": q.Page, "page_size": q.PageSize,
	}, nil
}

// MarkRead 标记已读；id=0 全部已读。
func (s *MPService) MarkRead(userID, id string) *errs.Error {
	if id == "0" || id == "" {
		s.db.Model(&sysmodel.SysMessage{}).Where("user_id = ? AND is_read = false", userID).Update("is_read", true)
		return nil
	}
	res := s.db.Model(&sysmodel.SysMessage{}).Where("id = ? AND user_id = ?", id, userID).Update("is_read", true)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

func (s *MPService) commName(id string) string {
	var c sysmodel.Community
	if s.db.Select("name").First(&c, "id = ?", id).Error == nil {
		return c.Name
	}
	return ""
}

func progressOf(done, total int) int {
	if total == 0 {
		return 0
	}
	return done * 100 / total
}
