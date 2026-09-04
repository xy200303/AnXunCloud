package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	authsvc "anxuncloud/internal/module/auth/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	inssvc "anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// ChannelMP 小程序端标记（仅用于登录日志/审计的客户端类型标识）。
// 会话通道自 v21 起与 App 合并为 ChannelApp：app/mp 共用一套会话体系，token 两端正通用。
const ChannelMP = "mp"

// MPService 小程序端服务（登录/任务/消息/公告）。
type MPService struct {
	db     *gorm.DB
	rdb    *redis.Client
	sess   *session.Store
	jwtm   *jwtutil.Manager
	wechat config.WechatConfig
	httpc  *http.Client
}

func NewMPService(db *gorm.DB, rdb *redis.Client, sess *session.Store, jwtm *jwtutil.Manager, wechat config.WechatConfig) *MPService {
	return &MPService{db: db, rdb: rdb, sess: sess, jwtm: jwtm, wechat: wechat, httpc: &http.Client{Timeout: 8 * time.Second}}
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
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
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

// PointByCode 任务定位：二维码编号与 NFC UID 分别精确匹配，并返回今日任务上下文（非打卡凭证）。
func (s *MPService) PointByCode(inspectorID, code string) (gin.H, *errs.Error) {
	var pt insmodel.InspectionPoint
	code = strings.TrimSpace(code)
	if err := s.db.Where("qrcode_no = ? OR nfc_id = ?", code, code).First(&pt).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	// 数据权限：先卡租户边界（非超管不得跨租户），再按数据范围/编制收窄
	var user sysmodel.SysUser
	if err := s.db.Select("id", "role_ids", "tenant_id").First(&user, "id = ?", inspectorID).Error; err != nil {
		return nil, errs.ErrUnauthorized
	}
	roleIDs, err := middleware.EffectiveRoleIDs(s.db, &user)
	if err != nil {
		return nil, errs.ErrInternal
	}
	var roles []sysmodel.SysRole
	if len(roleIDs) > 0 {
		s.db.Select("id", "data_scope").Where("id IN ?", roleIDs).Find(&roles)
	}
	scopeAll := false
	for _, r := range roles {
		if r.DataScope == sysmodel.ScopeAll {
			scopeAll = true
			break
		}
	}
	// 租户边界：超管（super_admin 角色）不限；其余一律不得跨租户
	isSuper := false
	for _, r := range roles {
		if r.Code == sysmodel.SuperAdminCode {
			isSuper = true
			break
		}
	}
	if !isSuper && pt.TenantID != nil && user.TenantID != "" && *pt.TenantID != user.TenantID {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
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
		if !insmodel.TaskPointIDs(t).Contains(pt.ID) {
			continue
		}
		checked := s.db.Where("task_id = ? AND point_id = ? AND superseded_by IS NULL", t.ID, pt.ID).
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
			"credential":    pt.Credential, "require_fence": pt.RequireFence,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
		},
		"tasks": matched,
	}, nil
}

// PublicPoint 短链接公开点位摘要（免登录扫码打开 H5 信息页的数据源）。
// 脱敏原则：仅点位基础信息 + 巡检结果摘要，不出坐标、照片、凭证配置等敏感项。
func (s *MPService) PublicPoint(code string) (gin.H, *errs.Error) {
	var pt insmodel.InspectionPoint
	if err := s.db.Where("qrcode_no = ?", strings.TrimSpace(code)).First(&pt).Error; err != nil {
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
	s.db.Model(&insmodel.CheckinRecord{}).Where("point_id = ? AND checkin_time >= ? AND superseded_by IS NULL", pt.ID, since).Count(&total30)
	s.db.Model(&insmodel.CheckinRecord{}).
		Where("point_id = ? AND checkin_time >= ? AND result = ? AND superseded_by IS NULL", pt.ID, since, insmodel.ResultAbnormal).Count(&abnormal30)
	// 最近 5 条巡检记录（过滤已被覆盖的旧记录）
	var recs []insmodel.CheckinRecord
	s.db.Where("point_id = ? AND superseded_by IS NULL", pt.ID).Order("checkin_time DESC").Limit(5).Find(&recs)
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

// patrolTypeLabels 巡查类型 value→字典 label（sys_dict_data type_code=patrol_type）；
// 新类型（如 fire 消防设施专项）字典化后自动生效，字典缺值回落空串由前端兜底。
func (s *MPService) patrolTypeLabels(values ...string) map[string]string {
	labels := map[string]string{}
	if len(values) == 0 {
		return labels
	}
	var rows []sysmodel.SysDictData
	s.db.Select("value", "label").Where("type_code = ? AND value IN ?", "patrol_type", values).Find(&rows)
	for i := range rows {
		labels[rows[i].Value] = rows[i].Label
	}
	return labels
}

// TodayTasks 今日任务列表 + 总进度（进行中排最前）。
func (s *MPService) TodayTasks(inspectorID string) (gin.H, *errs.Error) {
	return s.tasksByDate(inspectorID, time.Now().Format("2006-01-02"))
}

// HistoryTasks 历史任务列表（按日期回看；逾期任务可从详情进向导/表单补拍）。
func (s *MPService) HistoryTasks(inspectorID, date string) (gin.H, *errs.Error) {
	if _, err := time.ParseInLocation("2006-01-02", date, time.Local); err != nil {
		return nil, errs.ErrParam.WithMsg("date 格式应为 YYYY-MM-DD")
	}
	return s.tasksByDate(inspectorID, date)
}

// tasksByDate 指定日期任务列表 + 总进度（进行中排最前；今日/历史共用）。
func (s *MPService) tasksByDate(inspectorID, date string) (gin.H, *errs.Error) {
	var tasks []insmodel.InspectionTask
	if err := s.db.Where("inspector_id = ? AND task_date = ?", inspectorID, date).
		Order("CASE WHEN status = 'doing' THEN 0 ELSE 1 END, id ASC").Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 收集本批任务的巡查类型，一次 IN 查询取字典 label（避免循环单查）
	typeSet := map[string]bool{}
	typeValues := make([]string, 0, 4)
	for i := range tasks {
		if v := tasks[i].PatrolType; v != "" && !typeSet[v] {
			typeSet[v] = true
			typeValues = append(typeValues, v)
		}
	}
	typeLabels := s.patrolTypeLabels(typeValues...)
	items := make([]gin.H, 0, len(tasks))
	totalPts, donePts := 0, 0
	for i := range tasks {
		t := &tasks[i]
		totalPts += t.TotalPoints
		donePts += t.DonePoints
		var plan insmodel.InspectionPlan
		planName := ""
		if s.db.Unscoped().Select("name", "time_window", "deleted_at").First(&plan, "id = ?", t.PlanID).Error == nil {
			planName = plan.Name
			if plan.DeletedAt.Valid {
				planName += "（已删除）"
			}
		}
		items = append(items, gin.H{
			"id": t.ID, "plan_name": planName, "community_name": s.commName(t.CommunityID),
			"patrol_type":       t.PatrolType,             // 巡查类型透出，app 端按类型分组展示
			"patrol_type_label": typeLabels[t.PatrolType], // 字典 label（App 直接展示，不再硬编码映射）
			"task_date":         date, "time_window": t.TimeWindow, "round_name": t.RoundName, "status": t.Status,
			"total_points": t.TotalPoints, "done_points": t.DonePoints,
			"progress":   progressOf(t.DonePoints, t.TotalPoints),
			"started_at": timefmt.TP(t.StartedAt),
		})
	}
	return gin.H{
		"date": date, "total_points": totalPts, "done_points": donePts,
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
	s.db.Where("task_id = ? AND superseded_by IS NULL", task.ID).Find(&checkins)
	byPoint := map[string]*insmodel.CheckinRecord{}
	for i := range checkins {
		byPoint[checkins[i].PointID] = &checkins[i]
	}
	// 任务点位名单：以任务快照为准（生成时固化，不随计划后续编辑变化）
	pointIDs := insmodel.TaskPointIDs(&task)
	// 点位/楼栋一次批量预载（3900+ 点位任务逐点 First 会产生数千次 SQL），再按名单顺序组装。
	// 注意 pointIDs 是 types.IDArray（实现了 driver.Valuer 会整体序列化为 JSON 字符串），必须显式转 []string 才能被 IN 展开。
	var ptRows []insmodel.InspectionPoint
	if len(pointIDs) > 0 {
		s.db.Where("id IN ?", []string(pointIDs)).Find(&ptRows)
	}
	ptByID := make(map[string]*insmodel.InspectionPoint, len(ptRows))
	buildingIDSet := map[string]struct{}{}
	for i := range ptRows {
		ptByID[ptRows[i].ID] = &ptRows[i]
		if ptRows[i].BuildingID != nil && *ptRows[i].BuildingID != "" {
			buildingIDSet[*ptRows[i].BuildingID] = struct{}{}
		}
	}
	buildingNames := map[string]string{}
	if len(buildingIDSet) > 0 {
		bIDs := make([]string, 0, len(buildingIDSet))
		for id := range buildingIDSet {
			bIDs = append(bIDs, id)
		}
		var bs []insmodel.Building
		s.db.Select("id", "name").Where("id IN ?", bIDs).Find(&bs)
		for _, b := range bs {
			buildingNames[b.ID] = b.Name
		}
	}
	points := make([]gin.H, 0, len(pointIDs))
	ptTpl := map[int]string{} // points 下标 → 点位绑定的检查项模板 ID（仅非空）
	for i, pid := range pointIDs {
		pt, ok := ptByID[pid]
		if !ok {
			continue
		}
		buildingName := ""
		if pt.BuildingID != nil {
			buildingName = buildingNames[*pt.BuildingID]
		}
		var myCheckin any
		if ck, ok := byPoint[pid]; ok {
			dist := any(nil)
			if ck.DistanceToPoint != nil {
				dist = int(*ck.DistanceToPoint)
			}
			// 海拔/定位精度（可空，仅参考展示，不参与校验）
			alt := any(nil)
			if ck.Altitude != nil {
				alt = *ck.Altitude
			}
			acc := any(nil)
			if ck.Accuracy != nil {
				acc = *ck.Accuracy
			}
			myCheckin = gin.H{
				"id": ck.ID, "checkin_time": timefmt.T(ck.CheckinTime),
				"checkin_type": ck.CheckinType, "distance_to_point": dist,
				"altitude": alt, "accuracy": acc,
				"result": ck.Result, "is_suspect": ck.IsSuspect,
				"locked": ck.LockedAt != nil, // 已随周期报告归档锁定：不可覆盖修改
			}
		}
		points = append(points, gin.H{
			"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
			"sort": i + 1, "credential": pt.Credential, "require_fence": pt.RequireFence, "qrcode_no": pt.QRCodeNo,
			"nfc_id":    pt.NfcID,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
			"my_checkin": myCheckin,
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
				"judge_type": it.JudgeType, // 判定类型透出（向导区分拍照项/感官项 manual）
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
		"patrol_type":       task.PatrolType,                                      // 巡查类型透出，app 端按类型分组展示
		"patrol_type_label": s.patrolTypeLabels(task.PatrolType)[task.PatrolType], // 字典 label（同 TodayTasks 口径）
		"task_date":         task.TaskDate.Format("2006-01-02"), "time_window": insmodel.TaskTimeWindow(&task),
		"round_name": task.RoundName,
		"status":     task.Status, "total_points": task.TotalPoints, "done_points": task.DonePoints,
		"progress": progressOf(task.DonePoints, task.TotalPoints), "points": points,
	}, nil
}

// ========== 打卡记录 ==========

// CheckinItems 本人打卡记录的逐项 AI 结论（GET /checkins/:id/items，供 App 提交后回显 / 记录卡展示）。
// 不透出 ai_hint（内部识别要点，仅供大模型核对）；非本人记录按「不存在」口径返回（防枚举）。
// ai_verdict 为空串 = 模型未返回该项结论（AI 未启用/异步未完成/无逐项结论）。
// photo_urls 为该项照片可访问 URL（优先水印图；记录卡逐项展示用）。
func (s *MPService) CheckinItems(inspectorID, checkinID string) ([]gin.H, *errs.Error) {
	var rec insmodel.CheckinRecord
	if err := s.db.Select("id", "inspector_id").First(&rec, "id = ?", checkinID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if rec.InspectorID != inspectorID {
		return nil, errs.ErrNotFound.WithMsg("打卡记录不存在或不属于当前巡检员")
	}
	var items []insmodel.CheckinRecordItem
	if err := s.db.Where("record_id = ?", rec.ID).Order("sort ASC").Find(&items).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := make([]gin.H, 0, len(items))
	for i := range items {
		it := &items[i]
		out = append(out, gin.H{
			"name": it.Name, "pass": it.Pass,
			"ai_verdict": strVal(it.AIVerdict), "ai_reason": strVal(it.AIReason),
			"ai_reading":     strVal(it.AIReading),
			"note":           it.Note,
			"exception_type": it.ExceptionType,
			"photo_urls":     inssvc.ItemPhotoURLs(s.db, it.Photos),
		})
	}
	return out, nil
}

// CheckinBrief 本人打卡记录摘要（GET /checkins/:id）：消息深链定位记录卡用，
// 只透出 task_id/point_id/审核状态等最小字段；非本人记录按「不存在」口径返回（防枚举）。
func (s *MPService) CheckinBrief(inspectorID, checkinID string) (gin.H, *errs.Error) {
	var rec insmodel.CheckinRecord
	if err := s.db.Select("id", "inspector_id", "task_id", "point_id", "audit_status").First(&rec, "id = ?", checkinID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if rec.InspectorID != inspectorID {
		return nil, errs.ErrNotFound.WithMsg("打卡记录不存在或不属于当前巡检员")
	}
	return gin.H{
		"id": rec.ID, "task_id": rec.TaskID, "point_id": rec.PointID,
		"audit_status": rec.AuditStatus,
	}, nil
}

// ========== 消息 ==========

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

// haversineMeters 两点球面距离（米）。
func haversineMeters(lng1, lat1, lng2, lat2 float64) float64 {
	const earthR = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthR * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// nearbyLimit 附近点位返回条数上限。
const nearbyLimit = 20

// NearbyPoints 附近点位：按当前定位对「我今日未完成任务涉及的点位」按距离升序推荐（找点辅助）。
// 注意：民用 GPS 精度 5~20m，楼内密集点位无法区分，仅供人工点选——打卡凭证校验（扫码/NFC/围栏）照常。
// 返回 {list: [...]}；ai_enabled 由控制器层透出（配置归 checkin 服务管，同 TaskDetail 模式）。
func (s *MPService) NearbyPoints(inspectorID string, lng, lat float64) (gin.H, *errs.Error) {
	if lng < -180 || lng > 180 || lat < -90 || lat > 90 || (lng == 0 && lat == 0) {
		return nil, errs.ErrParam.WithMsg("定位坐标非法")
	}
	today := time.Now().Format("2006-01-02")
	var tasks []insmodel.InspectionTask
	if err := s.db.Where("inspector_id = ? AND task_date = ? AND status IN ?", inspectorID, today,
		[]string{insmodel.TaskPending, insmodel.TaskDoing, insmodel.TaskOverdue}).Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(tasks) == 0 {
		return gin.H{"list": []gin.H{}}, nil
	}
	// 批量预载：计划名（Unscoped，已删除计划的任务仍可读）、任务点位快照、点位、楼栋、我的打卡集合
	planIDs := make([]string, 0, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	pointIDSet := map[string]struct{}{}
	taskPointIDs := make(map[string][]string, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		planIDs = append(planIDs, t.PlanID)
		taskIDs = append(taskIDs, t.ID)
		var plan insmodel.InspectionPlan
		if err := s.db.Unscoped().First(&plan, "id = ?", t.PlanID).Error; err != nil {
			continue
		}
		ids := insmodel.TaskPointIDs(t)
		taskPointIDs[t.ID] = []string(ids)
		for _, pid := range ids {
			pointIDSet[pid] = struct{}{}
		}
	}
	planNames := map[string]string{}
	{
		var plans []insmodel.InspectionPlan
		s.db.Unscoped().Select("id", "name").Where("id IN ?", planIDs).Find(&plans)
		for _, p := range plans {
			planNames[p.ID] = p.Name
		}
	}
	ptIDs := make([]string, 0, len(pointIDSet))
	for pid := range pointIDSet {
		ptIDs = append(ptIDs, pid)
	}
	ptByID := map[string]*insmodel.InspectionPoint{}
	buildingIDSet := map[string]struct{}{}
	if len(ptIDs) > 0 {
		var pts []insmodel.InspectionPoint
		s.db.Where("id IN ?", ptIDs).Find(&pts)
		for i := range pts {
			ptByID[pts[i].ID] = &pts[i]
			if pts[i].BuildingID != nil && *pts[i].BuildingID != "" {
				buildingIDSet[*pts[i].BuildingID] = struct{}{}
			}
		}
	}
	buildingNames := map[string]string{}
	if len(buildingIDSet) > 0 {
		bIDs := make([]string, 0, len(buildingIDSet))
		for id := range buildingIDSet {
			bIDs = append(bIDs, id)
		}
		var bs []insmodel.Building
		s.db.Select("id", "name").Where("id IN ?", bIDs).Find(&bs)
		for _, b := range bs {
			buildingNames[b.ID] = b.Name
		}
	}
	checkedSet := map[string]bool{} // task_id|point_id
	{
		var recs []insmodel.CheckinRecord
		s.db.Select("task_id", "point_id").Where("task_id IN ? AND superseded_by IS NULL", taskIDs).Find(&recs)
		for _, r := range recs {
			checkedSet[r.TaskID+"|"+r.PointID] = true
		}
	}
	type entry struct {
		row  gin.H
		dist float64
	}
	entries := make([]entry, 0, len(pointIDSet))
	for i := range tasks {
		t := &tasks[i]
		for _, pid := range taskPointIDs[t.ID] {
			pt, ok := ptByID[pid]
			if !ok || (pt.Longitude == 0 && pt.Latitude == 0) {
				continue // 点位无坐标（导入留空待现场刷新）无法算距离，不出现在附近列表
			}
			d := haversineMeters(lng, lat, pt.Longitude, pt.Latitude)
			buildingName := ""
			if pt.BuildingID != nil {
				buildingName = buildingNames[*pt.BuildingID]
			}
			entries = append(entries, entry{row: gin.H{
				"task_id": t.ID, "plan_name": planNames[t.PlanID], "patrol_type": t.PatrolType,
				"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
				"distance":   int(d + 0.5),
				"checked":    checkedSet[t.ID+"|"+pid],
				"credential": pt.Credential, "require_fence": pt.RequireFence,
				"task_status": t.Status,
			}, dist: d})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		// 未打卡优先，同状态下按距离升序
		ci, cj := entries[i].row["checked"].(bool), entries[j].row["checked"].(bool)
		if ci != cj {
			return !ci
		}
		return entries[i].dist < entries[j].dist
	})
	out := make([]gin.H, 0, nearbyLimit)
	for i := 0; i < len(entries) && i < nearbyLimit; i++ {
		out = append(out, entries[i].row)
	}
	return gin.H{"list": out}, nil
}
