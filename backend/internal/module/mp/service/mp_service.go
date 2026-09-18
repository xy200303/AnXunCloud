package service

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	eqsvc "anxuncloud/internal/module/equipment/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	inssvc "anxuncloud/internal/module/inspection/service"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/session"
	"anxuncloud/internal/pkg/strutil"
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
	msg    *systemsvc.MessageService
}

func NewMPService(db *gorm.DB, rdb *redis.Client, sess *session.Store, jwtm *jwtutil.Manager, wechat config.WechatConfig, msg *systemsvc.MessageService) *MPService {
	return &MPService{db: db, rdb: rdb, sess: sess, jwtm: jwtm, wechat: wechat, httpc: &http.Client{Timeout: 8 * time.Second}, msg: msg}
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
	// 今日任务上下文：我今日包含该点位的任务及打卡状态（多个任务取列表，客户端选最近未完成）；
	// 抽查任务（due_date 机制）在期限内持续命中——扫码打卡不受生成日限制
	today := time.Now().Format("2006-01-02")
	var tasks []insmodel.InspectionTask
	s.db.Where("inspector_id = ? AND (task_date = ? OR (task_date < ? AND due_date >= ?))", inspectorID, today, today, today).Find(&tasks)
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

// dateOrEmpty 可空日期转 YYYY-MM-DD（nil 返回空串，任务列表 due_date 透出用）。
func dateOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
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
// 今日列表额外带上「生成日已过、完成期限未到、未完成」的抽查任务（due_date 机制）：
// 抽查任务 task_date=每月生成日，期限内对员工持续可见可打卡；历史回看仍按 task_date 精确匹配，日常任务口径不变。
func (s *MPService) tasksByDate(inspectorID, date string) (gin.H, *errs.Error) {
	var tasks []insmodel.InspectionTask
	if err := s.db.Where("inspector_id = ? AND task_date = ?", inspectorID, date).
		Order("CASE WHEN status = 'doing' THEN 0 ELSE 1 END, id ASC").Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if date == time.Now().Format("2006-01-02") {
		var carry []insmodel.InspectionTask
		if err := s.db.Where("inspector_id = ? AND task_date < ? AND due_date >= ? AND status IN ?",
			inspectorID, date, date, []string{insmodel.TaskPending, insmodel.TaskDoing}).
			Order("id ASC").Find(&carry).Error; err != nil {
			return nil, errs.ErrInternal
		}
		tasks = append(tasks, carry...)
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
	// 批量预载：计划名（Unscoped，已删除计划标注「已删除」）与小区名各一次 IN 查询（消除逐任务 2 查）
	planIDs, commIDs := []string{}, []string{}
	seenP, seenC := map[string]bool{}, map[string]bool{}
	for i := range tasks {
		t := &tasks[i]
		if !seenP[t.PlanID] {
			seenP[t.PlanID] = true
			planIDs = append(planIDs, t.PlanID)
		}
		if !seenC[t.CommunityID] {
			seenC[t.CommunityID] = true
			commIDs = append(commIDs, t.CommunityID)
		}
	}
	planNames := map[string]string{}
	if len(planIDs) > 0 {
		var plans []insmodel.InspectionPlan
		s.db.Unscoped().Select("id", "name", "deleted_at").Where("id IN ?", planIDs).Find(&plans)
		for i := range plans {
			name := plans[i].Name
			if plans[i].DeletedAt.Valid {
				name += "（已删除）"
			}
			planNames[plans[i].ID] = name
		}
	}
	commNames := map[string]string{}
	if len(commIDs) > 0 {
		var comms []sysmodel.Community
		s.db.Select("id", "name").Where("id IN ?", commIDs).Find(&comms)
		for i := range comms {
			commNames[comms[i].ID] = comms[i].Name
		}
	}
	items := make([]gin.H, 0, len(tasks))
	totalPts, donePts := 0, 0
	for i := range tasks {
		t := &tasks[i]
		totalPts += t.TotalPoints
		donePts += t.DonePoints
		items = append(items, gin.H{
			"id": t.ID, "plan_name": planNames[t.PlanID], "community_name": commNames[t.CommunityID],
			"patrol_type":       t.PatrolType,             // 巡查类型透出，app 端按类型分组展示
			"patrol_type_label": typeLabels[t.PatrolType], // 字典 label（App 直接展示，不再硬编码映射）
			"task_date":         t.TaskDate.Format("2006-01-02"), "time_window": t.TimeWindow, "round_name": t.RoundName, "status": t.Status,
			"due_date":     dateOrEmpty(t.DueDate), // 抽查任务完成期限（日常任务为空串）
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
				"audit_status": ck.AuditStatus, "audit_remark": ck.AuditRemark, // 审核状态/打回原因透出（巡检员记录页可见）
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
	}
	// 点位检查项 = 该点位全部模板（point_template）检查项并集：一次批量加载（禁循环单查），
	// 按模板组合顺序 + 项 sort 展开；每项携 template_id/template_name 供前端分组显示。
	tplSets := insmodel.LoadPointTemplateSets(s.db, []string(pointIDs))
	for idx := range points {
		pid, _ := points[idx]["point_id"].(string)
		set := tplSets[pid]
		photoMode := insmodel.PhotoModePerItem // 无模板点位为 per_item
		ci := []gin.H{}                        // 无模板（或模板无项）输出空数组而非 null
		if set != nil {
			photoMode = set.PhotoMode
			for _, it := range set.Items {
				requirement := ""
				if it.Requirement != nil {
					requirement = *it.Requirement
				}
				ci = append(ci, gin.H{
					"name": it.Name, "requirement": requirement, "photo_required": it.PhotoRequired,
					"judge_type":  it.JudgeType, // 判定类型透出（向导区分拍照项/感官项 manual）
					"tags":        it.Tags,      // 观察点 tag 数组（向导反向勾选：默认正常，点选异常）
					"template_id": it.TemplateID, "template_name": it.TemplateName,
				})
			}
		}
		points[idx]["photo_mode"] = photoMode
		points[idx]["check_items"] = ci
	}
	// 台账有效期（equipment_validity）合成检查项：绑定即启用——按点位查在用设备，
	// 有设备的点位在 check_items 末尾逐台注入合成项（与模板是否配置无关）
	s.injectEquipmentItems(task.ID, task.TaskDate.Format("2006-01-02"), points)
	return gin.H{
		"id": task.ID, "plan_name": plan.Name, "community_name": s.commName(task.CommunityID),
		"patrol_type":       task.PatrolType,                                      // 巡查类型透出，app 端按类型分组展示
		"patrol_type_label": s.patrolTypeLabels(task.PatrolType)[task.PatrolType], // 字典 label（同 TodayTasks 口径）
		"task_date":         task.TaskDate.Format("2006-01-02"), "time_window": insmodel.TaskTimeWindow(&task),
		"due_date":   dateOrEmpty(task.DueDate), // 抽查任务完成期限（日常任务为空串）
		"round_name": task.RoundName,
		"status":     task.Status, "total_points": task.TotalPoints, "done_points": task.DonePoints,
		"progress": progressOf(task.DonePoints, task.TotalPoints), "points": points,
	}, nil
}

// injectEquipmentItems 台账有效期合成检查项注入（§3.5 v1.6：绑定即启用 + 逐台独立）：
// 任务全部点位一次 IN 批量查关联在用设备（消除 N+1），有待确认登记批量预取；
// 有设备的点位在 check_items 末尾逐台追加合成项（每台设备一条），无设备点位不出现该项——与模板是否配置无关。
// 查询失败静默降级（不注入，打卡不受影响），不阻断任务详情。
func (s *MPService) injectEquipmentItems(taskID, taskDate string, points []gin.H) {
	pointIDs := make([]string, 0, len(points))
	for idx := range points {
		if pid, _ := points[idx]["point_id"].(string); pid != "" {
			pointIDs = append(pointIDs, pid)
		}
	}
	if len(pointIDs) == 0 {
		return
	}
	byPoint, err := eqsvc.LoadPointEquipment(s.db, pointIDs)
	if err != nil {
		logger.L.Warn("台账合成检查项注入：查询关联设备失败，跳过注入", zap.Error(err))
		return
	}
	if len(byPoint) == 0 {
		return
	}
	// 批量预取待确认登记（逐台「待确认」展示态）
	eqIDs := make([]string, 0, 8)
	for _, eqs := range byPoint {
		for i := range eqs {
			eqIDs = append(eqIDs, eqs[i].ID)
		}
	}
	pendingSet := eqsvc.PendingMaintenanceSet(s.db, eqIDs)
	globalWarn := eqsvc.GlobalWarnDays(s.db)
	now := time.Now()
	// 抽查触发准备（v1.7 二期）：总开关 + 全局比例 + 盐值 + 长期未验证翻倍
	spotEnabled := eqsvc.CfgBool(s.db, "equipment.spotcheck_enabled", true)
	globalRatio := eqsvc.CfgInt(s.db, "equipment.spotcheck_ratio", 2)
	spotSalt := eqsvc.CfgString(s.db, "equipment.spotcheck_salt", "")
	var rules map[string]eqsvc.TypeRule
	var verified map[string]time.Time
	if spotEnabled {
		rules = eqsvc.NewEquipmentService(s.db).TypeRules()
		verified = eqsvc.LastVerifiedMap(s.db, eqIDs)
	}
	for idx := range points {
		pid, _ := points[idx]["point_id"].(string)
		eqs := byPoint[pid]
		if len(eqs) == 0 {
			continue
		}
		judges := eqsvc.JudgeDevices(eqs, pendingSet, globalWarn, now)
		// 同模板点位共享 check_items 切片：追加合成项前逐点拷贝，避免跨点位串数据
		items, _ := points[idx]["check_items"].([]gin.H)
		copied := make([]gin.H, len(items), len(items)+len(judges)*2)
		for i, it := range items {
			m := gin.H{}
			for k, v := range it {
				m[k] = v
			}
			copied[i] = m
		}
		for _, j := range judges {
			copied = append(copied, gin.H{
				"name":           eqsvc.SyntheticItemName(j),
				"requirement":    "",
				"photo_required": types.PhotoReqNone,
				"judge_type":     ai.JudgeEquipmentValidity,
				// 设备快照：提交落库与逐台去重键控用
				"judge_config": gin.H{"equipment_id": j.EquipmentID, "equipment_no": j.Code},
				"auto_judge":   j.View(),
			})
			// 日期标签抽查（紧跟该设备的有效期项之后）：临期/逾期必触发（到期核验），否则确定性哈希随机；
			// 台账长期未验证（超 6 个月）概率翻倍；标签缺失设备有效期项已判异常，不再叠加抽查
			if spotEnabled && j.State != eqsvc.AutoLabelMissing {
				triggered := j.State == eqsvc.DueWarning || j.State == eqsvc.DueOverdue
				if !triggered {
					lv, ok := verified[j.EquipmentID]
					var lvPtr *time.Time
					if ok {
						lvPtr = &lv
					}
					ratio := eqsvc.SpotRatioFor(rules[j.Type], globalRatio, lvPtr, now)
					triggered = eqsvc.SpotTriggered(taskID, pid, j.EquipmentID, taskDate, spotSalt, ratio)
				}
				if triggered {
					copied = append(copied, gin.H{
						"name":           eqsvc.SpotItemPrefix + j.Name + "(" + j.Code + ")",
						"requirement":    "拍 1 张瓶体标签/钢印照片，填写生产日期与维修日期（无贴纸选「无」）；标签磨损选「标签缺失」",
						"photo_required": types.PhotoReqRequired,
						"judge_type":     ai.JudgeEquipmentDateSpot,
						"judge_config":   gin.H{"equipment_id": j.EquipmentID, "equipment_no": j.Code},
						"auto_judge":     j.View(),
					})
				}
			}
		}
		points[idx]["check_items"] = copied
	}
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
			"ai_verdict": strutil.StrVal(it.AIVerdict), "ai_reason": strutil.StrVal(it.AIReason),
			"ai_reading":     strutil.StrVal(it.AIReading),
			"note":           it.Note,
			"exception_type": it.ExceptionType,
			"tags":           it.Tags, "abnormal_tags": it.AbnormalTags,
			"disposition": it.Disposition, "resolution_note": it.ResolutionNote,
			"photo_urls":            inssvc.ItemPhotoURLs(s.db, it.Photos),
			"resolution_photo_urls": inssvc.ItemPhotoURLs(s.db, it.ResolutionFileIDs),
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

// Messages 消息列表 + 未读数。查询/未读口径收敛至 system 侧 MessageService.List，
// 此处仅保留 App 端响应差异（多回传 page/page_size）。
// 行为对齐说明：is_read 非法值统一返回 400（以 system 版为准）——旧 mp 版静默忽略
// 并非有意容错（无注释/契约说明，且 type 过滤之外两版逐行一致属漂移）；非法值仅可能
// 来自异常客户端，正常 App 调用不受影响。
func (s *MPService) Messages(userID string, q *systemsvc.MessageListQuery) (gin.H, *errs.Error) {
	res, be := s.msg.List(userID, q)
	if be != nil {
		return nil, be
	}
	res["page"] = q.Page
	res["page_size"] = q.PageSize
	return res, nil
}

// MarkRead 标记已读；id=0 全部已读。实现收敛至 system 侧 MessageService.MarkRead。
// （旧 mp 版 id="" 也按全读处理，但控制器 pathID 已拒绝空 id，该分支为死代码，随收敛移除。）
func (s *MPService) MarkRead(userID, id string) *errs.Error {
	return s.msg.MarkRead(userID, id)
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
	// 抽查任务（due_date 机制）在期限内视同「今日未完成」参与附近点位推荐；日常任务（due_date 空）口径不变
	if err := s.db.Where("inspector_id = ? AND status IN ? AND (task_date = ? OR (task_date < ? AND due_date >= ?))",
		inspectorID, []string{insmodel.TaskPending, insmodel.TaskDoing, insmodel.TaskOverdue}, today, today, today).Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(tasks) == 0 {
		return gin.H{"list": []gin.H{}}, nil
	}
	// 批量预载：计划名（Unscoped，已删除计划的任务仍可读）、任务点位快照、点位、楼栋、我的打卡集合
	planIDs := make([]string, 0, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		planIDs = append(planIDs, t.PlanID)
		taskIDs = append(taskIDs, t.ID)
	}
	planNames := map[string]string{}
	{
		var plans []insmodel.InspectionPlan
		s.db.Unscoped().Select("id", "name").Where("id IN ?", planIDs).Find(&plans)
		for _, p := range plans {
			planNames[p.ID] = p.Name
		}
	}
	pointIDSet := map[string]struct{}{}
	taskPointIDs := make(map[string][]string, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		// 计划查不到（含物理删除）的任务跳过：存在性判断并入上方 planNames 批量结果，不再逐任务 First
		if _, ok := planNames[t.PlanID]; !ok {
			continue
		}
		ids := insmodel.TaskPointIDs(t)
		taskPointIDs[t.ID] = []string(ids)
		for _, pid := range ids {
			pointIDSet[pid] = struct{}{}
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
