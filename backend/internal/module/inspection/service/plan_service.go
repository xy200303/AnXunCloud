package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

var timeWindowRe = regexp.MustCompile(`^\d{2}:\d{2}-\d{2}:\d{2}$`)

// PlanService 巡检计划服务。
type PlanService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewPlanService(db *gorm.DB, rdb *redis.Client) *PlanService {
	return &PlanService{db: db, rdb: rdb}
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
	return gin.H{
		"id": p.ID, "community_id": p.CommunityID, "community_name": commName,
		"name": p.Name, "patrol_type": p.PatrolType, "point_count": len(p.PointIDs),
		"cycle_type": p.CycleType, "cycle_config": p.CycleConfig,
		"inspector_ids": p.InspectorIDs, "inspector_names": inspectorNames,
		"start_date": p.StartDate.Format("2006-01-02"), "end_date": endDate,
		"time_window": p.TimeWindow, "status": sysmodel.StatusInt(p.Status),
		"created_at": timefmt.T(p.CreatedAt),
	}
}

// Detail 计划详情（含路线点位快照）。
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
	points := make([]gin.H, 0, len(p.PointIDs))
	for i, pid := range p.PointIDs {
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
	start, end, be := s.validate(req)
	if be != nil {
		return "", be
	}
	patrolType, be := resolvePatrolType(req.PatrolType)
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
	if err := s.db.Create(&p).Error; err != nil {
		return "", errs.ErrInternal
	}
	return p.ID, nil
}

// Update 修改计划（仅影响之后生成的任务）。
func (s *PlanService) Update(c *gin.Context, id string, req *dto.PlanSaveReq) *errs.Error {
	var p model.InspectionPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	start, end, be := s.validate(req)
	if be != nil {
		return be
	}
	patrolType, be := resolvePatrolType(req.PatrolType)
	if be != nil {
		return be
	}
	cfg := types.JSONMap(req.CycleConfig)
	if cfg == nil {
		cfg = types.JSONMap{}
	}
	updates := map[string]any{
		"community_id": req.CommunityID, "name": req.Name, "patrol_type": patrolType,
		"point_ids": types.IDArray(req.PointIDs), "cycle_type": req.CycleType,
		"cycle_config": cfg, "inspector_ids": types.IDArray(req.InspectorIDs),
		"start_date": start, "end_date": end, "time_window": req.TimeWindow, "remark": req.Remark,
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(&p).Updates(updates).Error; err != nil {
		return errs.ErrInternal
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

// validate 校验计划周期配置、日期范围与时段格式。
func (s *PlanService) validate(req *dto.PlanSaveReq) (time.Time, *time.Time, *errs.Error) {
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
	if !timeWindowRe.MatchString(req.TimeWindow) {
		return start, nil, errs.ErrParam.WithMsg("time_window 格式应为 HH:MM-HH:MM")
	}
	// 周期细则校验（43003）
	cfg := types.JSONMap(req.CycleConfig)
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
	// 点位与巡检员存在性校验
	var count int64
	s.db.Model(&model.InspectionPoint{}).Where("id IN ? AND community_id = ?", req.PointIDs, req.CommunityID).Count(&count)
	if count != int64(len(uniqueIDs(req.PointIDs))) {
		return start, nil, errs.ErrParam.WithMsg("point_ids 中存在无效或不属于该小区的点位")
	}
	s.db.Model(&sysmodel.SysUser{}).Where("id IN ? AND status = ?", req.InspectorIDs, sysmodel.StatusEnabled).Count(&count)
	if count != int64(len(uniqueIDs(req.InspectorIDs))) {
		return start, nil, errs.ErrParam.WithMsg("inspector_ids 中存在无效或已停用的巡检员")
	}
	return start, end, nil
}

// resolvePatrolType 巡查类型解析：缺省 safety（安全巡查），非法取值报参数错误。
// 执行人仍按计划 inspector_ids 手动指定，类型仅作维度标记与筛选（不发明自动派单）。
func resolvePatrolType(t string) (string, *errs.Error) {
	if t == "" {
		return model.PatrolSafety, nil
	}
	if !model.ValidPatrolType(t) {
		return "", errs.ErrParam.WithMsg("patrol_type 取值非法（safety/equipment/environment/building）")
	}
	return t, nil
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
		if !ShouldRunOn(&plans[i], date) || len(plans[i].InspectorIDs) == 0 {
			continue
		}
		eligible++
		for _, inspectorID := range plans[i].InspectorIDs {
			var cnt int64
			s.db.Model(&model.InspectionTask{}).
				Where("plan_id = ? AND task_date = ? AND inspector_id = ?", plans[i].ID, date.Format("2006-01-02"), inspectorID).
				Count(&cnt)
			if cnt > 0 {
				continue
			}
			task := model.InspectionTask{
				TenantID:    plans[i].TenantID, // 冗余列随计划快照（=所属小区租户）
				PlanID:      plans[i].ID,
				CommunityID: plans[i].CommunityID,
				InspectorID: inspectorID,
				PatrolType:  plans[i].PatrolType, // 巡查类型随任务快照，计划后续改类型不影响已生成任务
				TaskDate:    date,
				Status:      model.TaskPending,
				TotalPoints: len(plans[i].PointIDs),
			}
			if err := s.db.Create(&task).Error; err != nil {
				continue // uk_task_plan_date_inspector 兜底，冲突即跳过
			}
			created++
		}
	}
	return created, eligible, nil
}

// FlipOverdue 将昨日及以前仍未完成的任务置为 overdue，并定向通知：
// 巡检员本人（任务已逾期）+ 该巡查业务线的汇报线成员（按「小区 × 巡查类型」汇总分线提醒；名单为空则该环节无提醒）。
func (s *PlanService) FlipOverdue() (int64, error) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	var tasks []model.InspectionTask
	if err := s.db.Select("id", "community_id", "inspector_id", "task_date", "patrol_type").
		Where("task_date <= ? AND status IN ?", yesterday, []string{model.TaskPending, model.TaskDoing}).
		Find(&tasks).Error; err != nil {
		return 0, err
	}
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
		s.db.Create(&sysmodel.SysMessage{
			UserID: t.InspectorID, Type: "task",
			Title:   "巡检任务已逾期",
			Content: "你的巡检任务（" + t.TaskDate.Format("2006-01-02") + "）已逾期未完成，请尽快补巡或向主管说明情况。",
			BizID:   &t.ID,
		})
		lineStats[t.CommunityID+"|"+t.PatrolType]++
	}
	for key, n := range lineStats {
		parts := strings.SplitN(key, "|", 2)
		cid, patrolType := parts[0], parts[1]
		slot := communitysvc.ResolveReportLineSlot(s.db, cid, patrolType)
		for _, uid := range communitysvc.SlotUserIDs(s.db, cid, slot) {
			s.db.Create(&sysmodel.SysMessage{
				UserID: uid, Type: "task",
				Title:   "巡查任务逾期提醒",
				Content: fmt.Sprintf("截至昨日，本项目%s有 %d 个巡查任务逾期未巡，请跟进督办。", patrolLineName(patrolType), n),
			})
		}
	}
	return res.RowsAffected, nil
}

// patrolLineName 巡查类型中文名（逾期提醒文案用；空串按安全巡查）。
func patrolLineName(patrolType string) string {
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
