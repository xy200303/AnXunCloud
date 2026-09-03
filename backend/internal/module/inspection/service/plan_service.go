package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

var timeWindowRe = regexp.MustCompile(`^\d{2}:\d{2}-\d{2}:\d{2}$`)

// PlanService 巡检计划服务。
type PlanService struct {
	db       *gorm.DB
	rdb      *redis.Client
	notifier *notify.Notifier
	getCfg   func(string) (string, bool) // 可空；路线优化开关读取用
}

func NewPlanService(db *gorm.DB, rdb *redis.Client, notifier *notify.Notifier, getCfg func(string) (string, bool)) *PlanService {
	return &PlanService{db: db, rdb: rdb, notifier: notifier, getCfg: getCfg}
}

// routeOptimize 路线优化开关（inspection.route_optimize，缺省 true）
func (s *PlanService) routeOptimize() bool {
	if s.getCfg == nil {
		return true
	}
	v, ok := s.getCfg("inspection.route_optimize")
	if !ok {
		return true
	}
	return v == "true" || v == "1" || v == "on" || v == "yes"
}

// List 计划分页列表。
func (s *PlanService) List(c *gin.Context, q *dto.PlanListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.InspectionPlan{})
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.CycleType != "" {
		db = db.Where("cycle_type = ?", q.CycleType)
	}
	if q.PatrolType != "" {
		db = db.Where("patrol_type = ?", q.PatrolType)
	}
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", status)
	}
	db = middleware.ApplyCommunityFilter(db, c, "community_id")
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.InspectionPlan
	offset, limit := q.Normalize()
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, s.toItem(&rows[i]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *PlanService) toItem(p *model.InspectionPlan) gin.H {
	commName := ""
	var comm sysmodel.Community
	if s.db.Select("name").First(&comm, "id = ?", p.CommunityID).Error == nil {
		commName = comm.Name
	}
	inspectorNames := []string{}
	if len(p.InspectorIDs) > 0 {
		s.db.Model(&sysmodel.SysUser{}).Where("id IN ?", []string(p.InspectorIDs)).Order("id ASC").Pluck("name", &inspectorNames)
	}
	endDate := ""
	if p.EndDate != nil {
		endDate = p.EndDate.Format("2006-01-02")
	}
	// 圈选模式点位数为实时命中数（显式名单直接取数组长度）
	pointCount := len(p.PointIDs)
	if p.SelectionMode == model.SelectionByPointTypes {
		pointCount = len(s.expandPlanPointIDs(p))
	}
	return gin.H{
		"id": p.ID, "community_id": p.CommunityID, "community_name": commName,
		"name": p.Name, "patrol_type": p.PatrolType, "point_count": pointCount,
		"cycle_type": p.CycleType, "cycle_config": p.CycleConfig,
		"inspector_ids": p.InspectorIDs, "inspector_names": inspectorNames,
		"start_date": p.StartDate.Format("2006-01-02"), "end_date": endDate,
		"time_window": p.TimeWindow, "status": sysmodel.StatusInt(p.Status),
		"selection_mode": p.SelectionMode, "point_types": p.PointTypes,
		"assign_mode":   p.AssignMode,
		"created_at":    timefmt.T(p.CreatedAt),
	}
}

// Detail 计划详情（含路线点位快照；by_point_types 圈选模式展示当前实时命中点位）。
func (s *PlanService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var p model.InspectionPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return nil, be
	}
	item := s.toItem(&p)
	item["updated_at"] = timefmt.T(p.UpdatedAt)
	var pointIDs types.IDArray
	if p.SelectionMode == model.SelectionByPointTypes {
		pointIDs = s.expandPlanPointIDs(&p)
	} else {
		pointIDs = p.PointIDs
	}
	points := make([]gin.H, 0, len(pointIDs))
	for i, pid := range pointIDs {
		var pt model.InspectionPoint
		if s.db.First(&pt, "id = ?", pid).Error != nil {
			continue
		}
		buildingName := ""
		if pt.BuildingID != nil {
			var b model.Building
			if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
				buildingName = b.Name
			}
		}
		points = append(points, gin.H{"id": pt.ID, "name": pt.Name, "building_name": buildingName, "sort": i + 1})
	}
	item["points"] = points
	return item, nil
}

// Create 新增计划。
func (s *PlanService) Create(c *gin.Context, req *dto.PlanSaveReq) (string, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return "", be
	}
	if middleware.CommunityTenantID(s.db, req.CommunityID) == nil {
		return "", errs.ErrCommunityNotExist
	}
	start, end, be := s.validate(req)
	if be != nil {
		return "", be
	}
	patrolType, be := s.resolvePatrolType(req.PatrolType)
	if be != nil {
		return "", be
	}
	status := sysmodel.StatusEnabled
	if req.Status != nil {
		status = sysmodel.StatusStr(*req.Status)
	}
	cfg := types.JSONMap(req.CycleConfig)
	if cfg == nil {
		cfg = types.JSONMap{}
	}
	p := model.InspectionPlan{
		TenantID:     middleware.CommunityTenantID(s.db, req.CommunityID), // 冗余列（=所属小区租户）
		CommunityID:  req.CommunityID,
		Name:         req.Name,
		PatrolType:   patrolType,
		PointIDs:     req.PointIDs,
		CycleType:    req.CycleType,
		CycleConfig:  cfg,
		InspectorIDs: req.InspectorIDs,
		StartDate:    start,
		EndDate:      end,
		TimeWindow:   req.TimeWindow,
		Status:       status,
		Remark:       req.Remark,
	}
	p.SelectionMode, p.PointTypes = selectionOf(req)
	p.AssignMode = req.AssignMode
	if p.AssignMode == "" {
		p.AssignMode = model.AssignAll
	}
	if err := s.db.Create(&p).Error; err != nil {
		return "", errs.ErrInternal
	}
	return p.ID, nil
}

// Update 修改计划，并同步所有未完成任务的计划快照。
func (s *PlanService) Update(c *gin.Context, id string, req *dto.PlanSaveReq) *errs.Error {
	var p model.InspectionPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return be
	}
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return errs.ErrCommunityNotExist
	}
	start, end, be := s.validate(req)
	if be != nil {
		return be
	}
	patrolType, be := s.resolvePatrolType(req.PatrolType)
	if be != nil {
		return be
	}
	cfg := types.JSONMap(req.CycleConfig)
	if cfg == nil {
		cfg = types.JSONMap{}
	}
	selectionMode, pointTypes := selectionOf(req)
	assignMode := req.AssignMode
	if assignMode == "" {
		assignMode = model.AssignAll
	}
	updates := map[string]any{
		"tenant_id": tenantID, "community_id": req.CommunityID, "name": req.Name, "patrol_type": patrolType,
		"point_ids": types.IDArray(req.PointIDs), "cycle_type": req.CycleType,
		"cycle_config": cfg, "inspector_ids": types.IDArray(req.InspectorIDs),
		"start_date": start, "end_date": end, "time_window": req.TimeWindow, "remark": req.Remark,
		"selection_mode": selectionMode, "point_types": types.StringArray(pointTypes),
		"assign_mode": assignMode,
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if p.CommunityID != req.CommunityID {
		var activeCount int64
		if err := s.db.Model(&model.InspectionTask{}).
			Where("plan_id = ? AND status IN ?", p.ID, activeTaskStatuses).
			Count(&activeCount).Error; err != nil {
			return errs.ErrInternal
		}
		if activeCount > 0 {
			return errs.ErrConflict.WithMsg("计划存在未完成任务，不能变更所属小区")
		}
	}
	p.TenantID, p.CommunityID, p.Name, p.PatrolType = tenantID, req.CommunityID, req.Name, patrolType
	p.PointIDs, p.CycleType, p.CycleConfig, p.InspectorIDs = types.IDArray(req.PointIDs), req.CycleType, cfg, types.IDArray(req.InspectorIDs)
	p.StartDate, p.EndDate, p.TimeWindow, p.Remark = start, end, req.TimeWindow, req.Remark
	p.SelectionMode, p.PointTypes = selectionMode, types.StringArray(pointTypes)
	p.AssignMode = assignMode
	if req.Status != nil {
		p.Status = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&p).Updates(updates).Error; err != nil {
			return err
		}
		return s.syncActiveTasks(tx, &p)
	}); err != nil {
		return errs.ErrInternal
	}
	return nil
}

var activeTaskStatuses = []string{model.TaskPending, model.TaskDoing, model.TaskOverdue}

// syncActiveTasks 将计划最新的路线、巡查类型和时段同步到未完成任务；已完成任务保留生成时快照。
func (s *PlanService) syncActiveTasks(tx *gorm.DB, p *model.InspectionPlan) error {
	var tasks []model.InspectionTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("plan_id = ? AND status IN ?", p.ID, activeTaskStatuses).
		Find(&tasks).Error; err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	pointIDs := s.expandPlanPointIDsWithDB(tx, p)
	// 与任务生成口径一致：路线优化后同步未完成任务
	if s.routeOptimize() {
		pointIDs = OrderPointsByRoute(tx, pointIDs)
	}
	roundWindows := make(map[string]string)
	for _, round := range model.PlanRounds(p.CycleConfig) {
		roundWindows[round.Name] = round.Window
	}
	for i := range tasks {
		task := &tasks[i]
		updates := map[string]any{
			"tenant_id":    p.TenantID,
			"community_id": p.CommunityID,
			"patrol_type":  p.PatrolType,
		}
		if len(pointIDs) > 0 {
			// 与任务生成口径一致：均分模式按任务日期+巡检员切块后同步
			// IDArray 包装：裸 []string 进 Updates 会被 pgx 编码成 record 而非 jsonb（42804）
			taskPoints := model.SplitPointsForDate(p, pointIDs, task.TaskDate, task.InspectorID)
			updates["point_ids"] = types.IDArray(taskPoints)
			updates["total_points"] = len(taskPoints)
			var donePoints int64
			if err := tx.Model(&model.CheckinRecord{}).
				Where("task_id = ? AND point_id IN ?", task.ID, taskPoints).
				Select("COUNT(DISTINCT point_id)").Scan(&donePoints).Error; err != nil {
				return err
			}
			updates["done_points"] = donePoints
		}
		if task.RoundName == "" {
			updates["time_window"] = p.TimeWindow
		} else if window, ok := roundWindows[task.RoundName]; ok {
			updates["time_window"] = window
		}
		if err := tx.Model(&model.InspectionTask{}).
			Where("id = ? AND status IN ?", task.ID, activeTaskStatuses).
			Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

// Delete 软删除计划，并取消未开始任务。
func (s *PlanService) Delete(c *gin.Context, id string) *errs.Error {
	var p model.InspectionPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&p).Error; err != nil {
			return err
		}
		// 未开始任务一并取消（软删）
		return tx.Where("plan_id = ? AND status = ?", id, model.TaskPending).Delete(&model.InspectionTask{}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// SetStatus 启停用计划。
func (s *PlanService) SetStatus(c *gin.Context, id string, status int) *errs.Error {
	var p model.InspectionPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	if err := s.db.Model(&p).Update("status", sysmodel.StatusStr(status)).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// selectionOf 归一圈选模式入参（缺省 explicit）。
func selectionOf(req *dto.PlanSaveReq) (string, []string) {
	mode := req.SelectionMode
	if mode == "" {
		mode = model.SelectionExplicit
	}
	return mode, req.PointTypes
}

// validate 校验计划周期配置、轮次配置、日期范围、时段格式与圈选模式。
func (s *PlanService) validate(req *dto.PlanSaveReq) (time.Time, *time.Time, *errs.Error) {
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return time.Time{}, nil, errs.ErrCommunityNotExist
	}
	start, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
	if err != nil {
		return start, nil, errs.ErrPlanDateInvalid.WithMsg("start_date 格式应为 YYYY-MM-DD")
	}
	var end *time.Time
	if req.EndDate != "" {
		t, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
		if err != nil {
			return start, nil, errs.ErrPlanDateInvalid.WithMsg("end_date 格式应为 YYYY-MM-DD")
		}
		if t.Before(start) {
			return start, nil, errs.ErrPlanDateInvalid.WithMsg("end_date 不能早于 start_date")
		}
		end = &t
	}
	cfg := types.JSONMap(req.CycleConfig)
	rounds := model.PlanRounds(cfg)
	if be := validateRounds(req.CycleType, rounds); be != nil {
		return start, nil, be
	}
	// 每日达标轮次线（可选）：配置了须为非负整数，不配=不设线
	if _, ok := cfg["daily_min_rounds"]; ok && model.PlanDailyMinRounds(cfg) == nil {
		return start, nil, errs.ErrPlanCycleInvalid.WithMsg("daily_min_rounds 须为非负整数")
	}
	// 时段：配了轮次以各轮次 window 为准，顶层 time_window 可留空；未配轮次维持必填
	if req.TimeWindow == "" && len(rounds) == 0 {
		return start, nil, errs.ErrParam.WithMsg("time_window 格式应为 HH:MM-HH:MM")
	}
	// 点位均分（split）仅 weekly/monthly 合法（daily 无执行日集合可切分）；
	// 与轮次互斥：轮次按天全量巡更，均分按周期切块，语义冲突
	if req.AssignMode == model.AssignSplit {
		if req.CycleType == "daily" {
			return start, nil, errs.ErrPlanCycleInvalid.WithMsg("每天周期不支持按日均分（每天本身即全量重复）")
		}
		if len(rounds) > 0 {
			return start, nil, errs.ErrPlanCycleInvalid.WithMsg("按日均分与轮次设置不能同时使用")
		}
	}
	if req.TimeWindow != "" {
		if !timeWindowRe.MatchString(req.TimeWindow) {
			return start, nil, errs.ErrParam.WithMsg("time_window 格式应为 HH:MM-HH:MM")
		}
		// 起止相等非法（规则表 #8；全天请配 00:00-23:59），跨零点（如 19:00-07:00）保持合法
		if req.TimeWindow[:5] == req.TimeWindow[6:] {
			return start, nil, errs.ErrParam.WithMsg("time_window 起止时刻不能相等（全天请配 00:00-23:59）")
		}
	}
	// 周期细则校验（43003）
	switch req.CycleType {
	case "daily":
		if cfg != nil {
			if iv, ok := cfg["interval"]; ok {
				if f, ok := iv.(float64); !ok || f < 1 {
					return start, nil, errs.ErrPlanCycleInvalid.WithMsg("daily 周期 interval 须为 ≥1 的整数")
				}
			}
		}
	case "weekly":
		days := cfg.Ints("weekdays")
		if len(days) == 0 {
			return start, nil, errs.ErrPlanCycleInvalid.WithMsg("weekly 周期须配置 weekdays（1-7）")
		}
		for _, d := range days {
			if d < 1 || d > 7 {
				return start, nil, errs.ErrPlanCycleInvalid.WithMsg("weekdays 取值须在 1-7 之间")
			}
		}
	case "monthly":
		days := cfg.Ints("days")
		if len(days) == 0 {
			return start, nil, errs.ErrPlanCycleInvalid.WithMsg("monthly 周期须配置 days（1-31，-1 为月末）")
		}
		for _, d := range days {
			if d != -1 && (d < 1 || d > 31) {
				return start, nil, errs.ErrPlanCycleInvalid.WithMsg("days 取值须在 1-31 之间或 -1")
			}
		}
	}
	// 点位圈选模式与点位/巡检员存在性校验
	switch selectionMode, _ := selectionOf(req); selectionMode {
	case model.SelectionExplicit:
		if len(req.PointIDs) == 0 {
			return start, nil, errs.ErrParam.WithMsg("explicit 模式 point_ids 不能为空")
		}
		var count int64
		s.db.Model(&model.InspectionPoint{}).Where("id IN ? AND community_id = ?", req.PointIDs, req.CommunityID).Count(&count)
		if count != int64(len(uniqueIDs(req.PointIDs))) {
			return start, nil, errs.ErrParam.WithMsg("point_ids 中存在无效或不属于该小区的点位")
		}
	case model.SelectionByPointTypes:
		if len(req.PointTypes) == 0 {
			return start, nil, errs.ErrParam.WithMsg("by_point_types 模式 point_types 不能为空")
		}
		for _, pt := range uniqueIDs(req.PointTypes) {
			if !validPointType(s.db, pt) {
				return start, nil, errs.ErrParam.WithMsg("point_types 中存在非法或已停用的点位类型：" + pt)
			}
		}
	default:
		return start, nil, errs.ErrParam.WithMsg("selection_mode 取值非法（explicit/by_point_types）")
	}
	var count int64
	userQuery := s.db.Model(&sysmodel.SysUser{}).Where("id IN ? AND status = ?", req.InspectorIDs, sysmodel.StatusEnabled)
	userQuery = userQuery.Where("tenant_id = ?", *tenantID)
	userQuery.Count(&count)
	if count != int64(len(uniqueIDs(req.InspectorIDs))) {
		return start, nil, errs.ErrParam.WithMsg("inspector_ids 中存在无效或已停用的巡检员")
	}
	return start, end, nil
}

// validateRounds 轮次配置校验：仅 daily/weekly 允许配轮次；每轮 name 1-32 字、
// window 须为 HH:MM-HH:MM（允许跨零点，任务日期归属开始时刻所在日；起止相等拒绝）、当天内不重名。
func validateRounds(cycleType string, rounds []model.Round) *errs.Error {
	if len(rounds) == 0 {
		return nil
	}
	if cycleType != "daily" && cycleType != "weekly" {
		return errs.ErrPlanCycleInvalid.WithMsg("仅 daily/weekly 周期支持轮次（rounds）")
	}
	seen := map[string]bool{}
	for _, r := range rounds {
		if n := utf8.RuneCountInString(r.Name); n < 1 || n > 32 {
			return errs.ErrPlanCycleInvalid.WithMsg("轮次名称须为 1-32 字")
		}
		// window 可空 = 自由时段（只考核轮次次数，不限定执行时段；逾期判定回落任务日终）
		if r.Window != "" {
			if !timeWindowRe.MatchString(r.Window) {
				return errs.ErrPlanCycleInvalid.WithMsg("轮次「" + r.Name + "」的 window 格式应为 HH:MM-HH:MM")
			}
			// 起止相等非法（规则表 #8；全天请配 00:00-23:59），跨零点（如 19:00-07:00）保持合法
			if r.Window[:5] == r.Window[6:] {
				return errs.ErrPlanCycleInvalid.WithMsg("轮次「" + r.Name + "」的 window 起止时刻不能相等（全天请配 00:00-23:59）")
			}
		}
		if seen[r.Name] {
			return errs.ErrPlanCycleInvalid.WithMsg("轮次名称重复：" + r.Name)
		}
		seen[r.Name] = true
	}
	return nil
}

// resolvePatrolType 巡查类型解析：缺省 safety（安全巡查），非法取值报参数错误。
// 执行人仍按计划 inspector_ids 手动指定，类型仅作维度标记与筛选（不发明自动派单）。
func (s *PlanService) resolvePatrolType(t string) (string, *errs.Error) {
	if t == "" {
		return model.PatrolSafety, nil
	}
	if !s.validPatrolType(t) {
		return "", errs.ErrParam.WithMsg("patrol_type 取值非法（须为字典 patrol_type 的启用项）")
	}
	return t, nil
}

// validPatrolType 巡查类型字典驱动校验（《专项巡检与专项检查报告设计方案》§3.1）：
// 字典 patrol_type 存在该值时以启用状态为准；字典无此值（seed 未跑/新库初始化顺序）回落内置常量校验。
func (s *PlanService) validPatrolType(t string) bool {
	var status string
	err := s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = ? AND value = ?", "patrol_type", t).
		Limit(1).Pluck("status", &status).Error
	if err != nil || status == "" {
		return model.ValidPatrolType(t)
	}
	return status == sysmodel.StatusEnabled
}

// ShouldRunOn 判断计划在某日期是否应生成任务。
func ShouldRunOn(p *model.InspectionPlan, date time.Time) bool {
	if date.Before(dateOnly(p.StartDate)) {
		return false
	}
	if p.EndDate != nil && date.After(dateOnly(*p.EndDate)) {
		return false
	}
	switch p.CycleType {
	case "daily":
		interval := p.CycleConfig.Int("interval")
		if interval < 1 {
			interval = 1
		}
		return int(date.Sub(dateOnly(p.StartDate)).Hours()/24)%interval == 0
	case "weekly":
		wd := int(date.Weekday())
		if wd == 0 {
			wd = 7 // 周日按 7
		}
		for _, d := range p.CycleConfig.Ints("weekdays") {
			if d == wd {
				return true
			}
		}
		return false
	case "monthly":
		days := p.CycleConfig.Ints("days")
		last := time.Date(date.Year(), date.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
		for _, d := range days {
			if d == date.Day() || (d == -1 && date.Day() == last) {
				return true
			}
		}
		return false
	}
	return false
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// GenerateForDate 为指定日期生成任务（Redis 锁 + 查重，幂等），返回新建任务数与当日应执行的启用计划数（前端据此区分「无计划」与「已生成过」）。
func (s *PlanService) GenerateForDate(ctx context.Context, date time.Time) (int, int, *errs.Error) {
	dateStr := date.Format("20060102")
	lockKey := "lock:plan:gen:" + dateStr
	ok, err := s.rdb.SetNX(ctx, lockKey, "1", 10*time.Minute).Result()
	if err != nil {
		return 0, 0, errs.ErrInternal
	}
	if !ok {
		return 0, 0, nil // 其他实例/请求正在生成，直接返回（幂等）
	}
	// 生成是短临界区（查重 + 唯一索引兜底），完成后立即释放，避免阻塞后续补充生成
	defer s.rdb.Del(ctx, lockKey)
	var plans []model.InspectionPlan
	if err := s.db.Where("status = ?", sysmodel.StatusEnabled).Find(&plans).Error; err != nil {
		return 0, 0, errs.ErrInternal
	}
	created, eligible := 0, 0
	for i := range plans {
		p := &plans[i]
		if !ShouldRunOn(p, date) || len(p.InspectorIDs) == 0 {
			continue
		}
		eligible++
		// 任务点位名单快照：explicit 照抄计划名单；by_point_types 生成时实时展开。
		// 计划更新时会同步未完成任务，新装点位仍从下一次生成任务开始生效。
		pointIDs := s.expandPlanPointIDs(p)
		if len(pointIDs) == 0 {
			continue
		}
		// 路线优化：楼栋聚类 + 最近邻重排，快照进任务（巡检员按最短路线顺序走）。
		// 均分模式在路线排序后按「执行日 × 巡检员」连续切块，每块地理聚集、每人每日距离最小
		if s.routeOptimize() {
			pointIDs = OrderPointsByRoute(s.db, pointIDs)
		}
		// 轮次展开：配了 rounds 每轮 × 每人一个任务（round_name/time_window 快照进任务列）；未配维持单任务现状
		rounds := model.PlanRounds(p.CycleConfig)
		if len(rounds) == 0 {
			rounds = []model.Round{{}}
		}
		for _, inspectorID := range p.InspectorIDs {
			for _, rd := range rounds {
				var cnt int64
				q := s.db.Model(&model.InspectionTask{}).
					Where("plan_id = ? AND task_date = ? AND inspector_id = ?", p.ID, date.Format("2006-01-02"), inspectorID)
				if rd.Name == "" {
					q = q.Where("COALESCE(round_name, '') = ''")
				} else {
					q = q.Where("round_name = ?", rd.Name)
				}
				q.Count(&cnt)
				if cnt > 0 {
					continue
				}
				// 均分模式：取本执行日 × 本巡检员的连续点位块
				taskPoints := model.SplitPointsForDate(p, pointIDs, date, inspectorID)
				if len(taskPoints) == 0 {
					continue
				}
				task := model.InspectionTask{
					TenantID:    p.TenantID, // 冗余列随计划快照（=所属小区租户）
					PlanID:      p.ID,
					CommunityID: p.CommunityID,
					InspectorID: inspectorID,
					PatrolType:  p.PatrolType, // 巡查类型随任务快照，计划更新会同步未完成任务
					TaskDate:    date,
					RoundName:   rd.Name,
					TimeWindow:  rd.Window,
					PointIDs:    taskPoints,
					Status:      model.TaskPending,
					TotalPoints: len(taskPoints),
				}
				if err := s.db.Create(&task).Error; err != nil {
					continue // uk_task_plan_date_inspector（含轮次维度）兜底，冲突即跳过
				}
				created++
			}
		}
	}
	return created, eligible, nil
}

// expandPlanPointIDs 计划点位展开：by_point_types 实时圈选启用点位（sort/创建时间序），explicit 照抄名单。
func (s *PlanService) expandPlanPointIDs(p *model.InspectionPlan) types.IDArray {
	return s.expandPlanPointIDsWithDB(s.db, p)
}

func (s *PlanService) expandPlanPointIDsWithDB(db *gorm.DB, p *model.InspectionPlan) types.IDArray {
	if p.SelectionMode != model.SelectionByPointTypes || len(p.PointTypes) == 0 {
		return p.PointIDs
	}
	var ids types.IDArray
	db.Model(&model.InspectionPoint{}).
		Where("community_id = ? AND type IN ? AND status = ?", p.CommunityID, []string(p.PointTypes), sysmodel.StatusEnabled).
		Order("sort ASC, created_at ASC").Pluck("id", &ids)
	return ids
}

// PreviewPoints 圈选命中预览：按小区 + 点位类型实时列出命中点位（计划表单预览用）。
func (s *PlanService) PreviewPoints(c *gin.Context, communityID, pointTypes string) (gin.H, *errs.Error) {
	if communityID == "" || strings.TrimSpace(pointTypes) == "" {
		return nil, errs.ErrParam.WithMsg("community_id 与 point_types 为必填项")
	}
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	typeList := make([]string, 0, 4)
	for _, pt := range strings.Split(pointTypes, ",") {
		if pt = strings.TrimSpace(pt); pt != "" {
			typeList = append(typeList, pt)
		}
	}
	if len(typeList) == 0 {
		return nil, errs.ErrParam.WithMsg("point_types 为必填项")
	}
	for _, pt := range typeList {
		if !validPointType(s.db, pt) {
			return nil, errs.ErrParam.WithMsg("point_types 中存在非法或已停用的点位类型：" + pt)
		}
	}
	// 命中总数走 COUNT；明细只返回前 previewPointsLimit 条示例（3900+ 点位小区全量返回会拖垮计划表单），
	// 楼栋名一次批量解析，避免逐点查询的 N+1。
	base := s.db.Model(&model.InspectionPoint{}).
		Where("community_id = ? AND type IN ? AND status = ?", communityID, typeList, sysmodel.StatusEnabled)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var pts []model.InspectionPoint
	if err := base.Order("sort ASC, created_at ASC").Limit(previewPointsLimit).Find(&pts).Error; err != nil {
		return nil, errs.ErrInternal
	}
	buildingNames := pointBuildingNames(s.db, pts)
	list := make([]gin.H, 0, len(pts))
	for i := range pts {
		buildingName := ""
		if pts[i].BuildingID != nil {
			buildingName = buildingNames[*pts[i].BuildingID]
		}
		list = append(list, gin.H{"id": pts[i].ID, "name": pts[i].Name, "type": pts[i].Type, "building_name": buildingName})
	}
	return gin.H{"count": total, "points": list, "preview_limit": previewPointsLimit}, nil
}

// FlipOverdue 将窗口结束时刻已过仍未完成的任务置为 overdue，并定向通知：
// 巡检员本人（任务已逾期）+ 该巡查业务线的汇报线成员（按「小区 × 巡查类型」汇总分线提醒；名单为空则该环节无提醒）。
// 翻转条件统一走 model.ShouldOverdue（§3.2 规则表 #11）：有快照 time_window 按窗口结束时刻判定
// （夜班 19:00-07:00 任务不会在次日 00:10 例行翻转中被误判），无窗口快照回落 task_date+1天 <= now。
func (s *PlanService) FlipOverdue() (int64, error) {
	now := time.Now()
	var tasks []model.InspectionTask
	if err := s.db.Select("id", "community_id", "inspector_id", "task_date", "time_window", "patrol_type", "round_name").
		Where("task_date <= ? AND status IN ?", now.Format("2006-01-02"), []string{model.TaskPending, model.TaskDoing}).
		Find(&tasks).Error; err != nil {
		return 0, err
	}
	// 内存过滤到期任务（含当天窗口已过的任务；未来任务天然不命中）
	due := make([]model.InspectionTask, 0, len(tasks))
	for _, t := range tasks {
		if model.ShouldOverdue(t.TaskDate, t.TimeWindow, now) {
			due = append(due, t)
		}
	}
	tasks = due
	if len(tasks) == 0 {
		return 0, nil
	}
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	res := s.db.Model(&model.InspectionTask{}).Where("id IN ?", ids).Update("status", model.TaskOverdue)
	if res.Error != nil {
		return 0, res.Error
	}
	lineStats := map[string]int{} // 小区|巡查类型 → 逾期数
	for _, t := range tasks {
		round := ""
		if t.RoundName != "" {
			round = "「" + t.RoundName + "」" // 轮次任务带上轮次名，按轮次追责
		}
		_ = s.notifier.Send(t.InspectorID, "task",
			"巡检任务已逾期",
			"你的巡检任务"+round+"（"+t.TaskDate.Format("2006-01-02")+"）已逾期未完成，请尽快补巡或向主管说明情况。",
			&t.ID)
		lineStats[t.CommunityID+"|"+t.PatrolType]++
	}
	for key, n := range lineStats {
		parts := strings.SplitN(key, "|", 2)
		cid, patrolType := parts[0], parts[1]
		slot := communitysvc.ResolveReportLineSlot(s.db, cid, patrolType)
		for _, uid := range communitysvc.SlotUserIDs(s.db, cid, slot) {
			_ = s.notifier.Send(uid, "task",
				"巡查任务逾期提醒",
				fmt.Sprintf("截至昨日，本项目%s有 %d 个巡查任务逾期未巡，请跟进督办。", s.patrolLineName(patrolType), n),
				nil)
		}
	}
	return res.RowsAffected, nil
}

// patrolLineName 巡查类型中文名（逾期提醒文案用）：优先取字典 patrol_type 的 label，查不到回落内置命名（空串按安全巡查）。
func (s *PlanService) patrolLineName(patrolType string) string {
	var label string
	if err := s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = ? AND value = ?", "patrol_type", patrolType).
		Limit(1).Pluck("label", &label).Error; err == nil && label != "" {
		return label
	}
	switch patrolType {
	case model.PatrolEquipment:
		return "设备专项巡查"
	case model.PatrolEnvironment:
		return "环境巡查"
	case model.PatrolBuilding:
		return "楼栋巡查"
	default:
		return "安全巡查"
	}
}

func uniqueIDs(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
