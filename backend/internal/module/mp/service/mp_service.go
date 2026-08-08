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
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	womodel "anxuncloud/internal/module/workorder/model"
	wosvc "anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/timefmt"
)

// ChannelMP 小程序会话渠道。
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
// 【mock 模式】wechat.appid/secret 未配置或 mock=true 时，code 传 "mock:<手机号>"：
// 按手机号查找已开通账号，首次自动绑定伪 openid（mock-openid-<手机号>），仅用于开发联调。
func (s *MPService) Login(ctx context.Context, req *dto.MPLoginReq, ip, ua string) (gin.H, *errs.Error) {
	var user *sysmodel.SysUser
	if s.wechat.MockEnabled() {
		u, be := s.mockLogin(req.Code)
		if be != nil {
			s.loginLog(nil, req.Code, ip, ua, "fail", be.Msg)
			return nil, be
		}
		user = u
	} else {
		u, be := s.wxLogin(ctx, req)
		if be != nil {
			s.loginLog(nil, req.Code, ip, ua, "fail", be.Msg)
			return nil, be
		}
		user = u
	}
	if user.Status != sysmodel.StatusEnabled {
		s.loginLog(&user.ID, user.Username, ip, ua, "fail", "账号已停用")
		return nil, errs.ErrAccountDisabled
	}
	resp, be := s.issueTokens(ctx, user)
	if be != nil {
		return nil, be
	}
	now := time.Now()
	s.db.Model(&sysmodel.SysUser{}).Where("id = ?", user.ID).Update("last_login_at", now)
	s.loginLog(&user.ID, user.Username, ip, ua, "success", "小程序登录成功")
	return resp, nil
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
	err = s.sess.Save(ctx, ChannelMP, user.ID, session.Info{
		TokenID: accessJTI, RefreshID: refreshJTI, Name: user.Name,
		Roles: strings.Join(roles, ","), LoginAt: time.Now().Format(timefmt.Layout),
	}, s.jwtm.RefreshTTL())
	if err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{
		"token_type": "Bearer", "access_token": access, "refresh_token": refresh,
		"expires_in": int64(s.jwtm.AccessTTL().Seconds()),
		"user": gin.H{
			"id": user.ID, "name": user.Name, "avatar": user.Avatar,
			"roles": roles, "community_ids": user.CommunityIDs,
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
	sessInfo, err := s.sess.Get(ctx, ChannelMP, claims.UserID)
	if err != nil || sessInfo == nil || sessInfo.RefreshID != claims.ID {
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
func (s *MPService) loginLog(userID *string, username, ip, ua, status, msg string) {
	rec := sysmodel.SysLoginLog{UserID: userID, Username: username, Channel: ChannelMP, IP: ip, Status: status, Msg: msg}
	if len(ua) > 500 {
		ua = ua[:500]
	}
	rec.UA = ua
	s.db.Create(&rec)
}

// ========== 任务 ==========

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
		if s.db.Select("name", "time_window").First(&plan, "id = ?", t.PlanID).Error == nil {
			planName, timeWindow = plan.Name, plan.TimeWindow
		}
		items = append(items, gin.H{
			"id": t.ID, "plan_name": planName, "community_name": s.commName(t.CommunityID),
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
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var checkins []insmodel.CheckinRecord
	s.db.Where("task_id = ?", task.ID).Find(&checkins)
	byPoint := map[string]*insmodel.CheckinRecord{}
	for i := range checkins {
		byPoint[checkins[i].PointID] = &checkins[i]
	}
	points := make([]gin.H, 0, len(plan.PointIDs))
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
			"sort": i + 1, "checkin_mode": pt.CheckinMode, "qrcode_no": pt.QRCodeNo,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
			"required_photo_items": pt.RequiredPhotoItems, "my_checkin": myCheckin,
		})
	}
	return gin.H{
		"id": task.ID, "plan_name": plan.Name, "community_name": s.commName(task.CommunityID),
		"task_date": task.TaskDate.Format("2006-01-02"), "time_window": plan.TimeWindow,
		"status": task.Status, "total_points": task.TotalPoints, "done_points": task.DonePoints,
		"progress": progressOf(task.DonePoints, task.TotalPoints), "points": points,
	}, nil
}

// ========== 我的工单 / 消息 ==========

// MyOrders 我的工单（我上报的 + 指派给我的）。
func (s *MPService) MyOrders(userID string, q *dto.MyOrdersQuery) (*response.Page, *errs.Error) {
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
		db = db.Where("status = ?", q.Status)
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
			"review_remark": o.ReviewRemark, "created_at": timefmt.T(o.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
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
