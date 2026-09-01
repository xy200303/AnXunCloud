package service

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/report/dto"
	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/types"

	"go.uber.org/zap"
)

// ========== 报告生成计划（统一计划引擎的注册任务之一） ==========

// planCfgInt 读 cycle_config 整数字段（缺省 def）。
func planCfgInt(cfg types.JSONMap, key string, def int) int {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

// duePeriod 计算计划当前应生成的期间（一律为「上一个完整周期」[start, end)，label 为展示串）。
// 未到期（生成时点未到/周期日不匹配）返回 ok=false；monthly 在生成日及之后均视为到期（停机补跑，last_period 判重）。
func duePeriod(p *model.ReportPlan, now time.Time) (start, end time.Time, label string, ok bool) {
	genTime := p.GenTime
	if genTime == "" {
		genTime = "06:00"
	}
	if now.Format("15:04") < genTime {
		return start, end, "", false
	}
	dayStart := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}
	switch p.CycleType {
	case "daily":
		y := now.AddDate(0, 0, -1)
		return dayStart(y), dayStart(now), y.Format("2006-01-02"), true
	case "weekly":
		// weekday：1=周一 … 7=周日（Go Sunday=0）
		wd := planCfgInt(p.CycleConfig, "weekday", 1) % 7
		if int(now.Weekday()) != wd {
			return start, end, "", false
		}
		offset := (int(now.Weekday()) + 6) % 7 // 距本周一的天数
		thisMon := dayStart(now.AddDate(0, 0, -offset))
		lastMon := thisMon.AddDate(0, 0, -7)
		return lastMon, thisMon, lastMon.Format("2006-01-02") + "~" + thisMon.AddDate(0, 0, -1).Format("2006-01-02"), true
	case "monthly":
		day := planCfgInt(p.CycleConfig, "day", 1)
		if day < 1 || day > 28 {
			day = 1
		}
		if now.Day() < day {
			return start, end, "", false
		}
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		prevFirst := first.AddDate(0, -1, 0)
		return prevFirst, first, prevFirst.Format("2006-01"), true
	}
	return start, end, "", false
}

// RunDueReportPlans 计划引擎任务：扫描启用的报告计划，到期且本期未生成的生成上一份完整周期报告。
// 幂等：last_period 判重 + createReport 按 (community, period, patrol_type) 判重 + uk_report_plan_period 唯一索引兜底。
func (s *ReportService) RunDueReportPlans(now time.Time) (int, error) {
	var plans []model.ReportPlan
	if err := s.db.Where("status = ?", sysmodel.StatusEnabled).Find(&plans).Error; err != nil {
		return 0, err
	}
	ran := 0
	for i := range plans {
		p := &plans[i]
		start, end, label, ok := duePeriod(p, now)
		if !ok || label == p.LastPeriod {
			continue
		}
		_, be := s.createReport(p.CommunityID, p.PatrolType, start, end, label, nil, nil, nil, &p.ID)
		upd := map[string]any{"last_period": label, "last_error": ""}
		if be != nil {
			// 已有同口径归档报告：视为本期已完成（记期间跳过）；其他错误记录待下轮重试
			if be == errs.ErrReportApproved {
				s.db.Model(p).Updates(upd)
				continue
			}
			upd["last_error"] = be.Msg
			s.db.Model(p).Updates(upd)
			logger.L.Warn("报告计划生成失败", zap.String("plan", p.Name), zap.String("period", label), zap.String("err", be.Msg))
			continue
		}
		s.db.Model(p).Updates(upd)
		ran++
		logger.L.Info("报告计划生成完成", zap.String("plan", p.Name), zap.String("period", label))
	}
	return ran, nil
}

// ---------- CRUD ----------

// ListReportPlans 报告计划列表（按数据范围过滤小区）。
func (s *ReportService) ListReportPlans(c *gin.Context, communityID string) ([]gin.H, *errs.Error) {
	db := middleware.ApplyCommunityFilter(s.db.Model(&model.ReportPlan{}), c, "community_id")
	if communityID != "" {
		db = db.Where("community_id = ?", communityID)
	}
	var plans []model.ReportPlan
	if err := db.Order("created_at ASC").Find(&plans).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := make([]gin.H, 0, len(plans))
	for i := range plans {
		out = append(out, s.reportPlanView(&plans[i]))
	}
	return out, nil
}

func (s *ReportService) reportPlanView(p *model.ReportPlan) gin.H {
	return gin.H{
		"id": p.ID, "community_id": p.CommunityID, "community_name": s.commName(p.CommunityID),
		"name": p.Name, "patrol_type": p.PatrolType, "patrol_type_label": s.patrolTypeLabel(p.PatrolType),
		"cycle_type": p.CycleType, "cycle_config": p.CycleConfig, "cycle_text": cycleTextOf(p),
		"gen_time": p.GenTime, "status": p.Status,
		"last_period": p.LastPeriod, "last_error": p.LastError, "remark": p.Remark,
		"created_at": p.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// cycleTextOf 周期中文描述（列表展示用）。
func cycleTextOf(p *model.ReportPlan) string {
	switch p.CycleType {
	case "daily":
		return "每天（生成昨日报告）"
	case "weekly":
		names := []string{"", "一", "二", "三", "四", "五", "六", "日"}
		wd := planCfgInt(p.CycleConfig, "weekday", 1)
		if wd < 1 || wd > 7 {
			wd = 1
		}
		return "每周" + names[wd] + "（生成上周报告）"
	default:
		return fmt.Sprintf("每月 %d 日（生成上月报告）", planCfgInt(p.CycleConfig, "day", 1))
	}
}

// validateReportPlanReq 计划参数校验；返回补全默认值后的周期配置。
func (s *ReportService) validateReportPlanReq(c *gin.Context, req *dto.ReportPlanReq) (types.JSONMap, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return nil, be
	}
	if s.commName(req.CommunityID) == "" {
		return nil, errs.ErrCommunityNotExist
	}
	if req.PatrolType != "" && !s.validPatrolType(req.PatrolType) {
		return nil, errs.ErrParam.WithMsg("patrol_type 取值非法（须为字典 patrol_type 的启用项）")
	}
	cfg := types.JSONMap{}
	switch req.CycleType {
	case "daily":
	case "weekly":
		wd := planCfgInt(req.CycleConfig, "weekday", 0)
		if wd < 1 || wd > 7 {
			return nil, errs.ErrParam.WithMsg("weekly 周期须配置 weekday（1=周一 … 7=周日）")
		}
		cfg["weekday"] = wd
	case "monthly":
		day := planCfgInt(req.CycleConfig, "day", 1)
		if day < 1 || day > 28 {
			return nil, errs.ErrParam.WithMsg("monthly 周期 day 须为 1~28")
		}
		cfg["day"] = day
	default:
		return nil, errs.ErrParam.WithMsg("cycle_type 取值非法（daily/weekly/monthly）")
	}
	if len(req.GenTime) != 5 || req.GenTime[2] != ':' {
		return nil, errs.ErrParam.WithMsg("gen_time 格式应为 HH:MM")
	}
	return cfg, nil
}

// CreateReportPlan 新建报告计划。
func (s *ReportService) CreateReportPlan(c *gin.Context, req *dto.ReportPlanReq) (gin.H, *errs.Error) {
	cfg, be := s.validateReportPlanReq(c, req)
	if be != nil {
		return nil, be
	}
	p := model.ReportPlan{
		TenantID: middleware.CommunityTenantID(s.db, req.CommunityID),
		CommunityID: req.CommunityID, Name: req.Name, PatrolType: req.PatrolType,
		CycleType: req.CycleType, CycleConfig: cfg, GenTime: req.GenTime,
		Status: sysmodel.StatusEnabled, Remark: req.Remark,
	}
	if err := s.db.Create(&p).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return s.reportPlanView(&p), nil
}

// UpdateReportPlan 更新报告计划（字段全量覆盖；不含 last_period/last_error）。
func (s *ReportService) UpdateReportPlan(c *gin.Context, id string, req *dto.ReportPlanReq) (gin.H, *errs.Error) {
	var p model.ReportPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	cfg, be := s.validateReportPlanReq(c, req)
	if be != nil {
		return nil, be
	}
	updates := map[string]any{
		"community_id": req.CommunityID, "name": req.Name, "patrol_type": req.PatrolType,
		"cycle_type": req.CycleType, "cycle_config": cfg, "gen_time": req.GenTime, "remark": req.Remark,
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if err := s.db.Model(&p).Updates(updates).Error; err != nil {
		return nil, errs.ErrInternal
	}
	s.db.First(&p, "id = ?", id)
	return s.reportPlanView(&p), nil
}

// DeleteReportPlan 删除报告计划（软删；已生成报告不受影响）。
func (s *ReportService) DeleteReportPlan(c *gin.Context, id string) *errs.Error {
	var p model.ReportPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	if err := s.db.Delete(&p).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// RunReportPlanNow 手动触发一次：无视生成时点，立即按周期规则生成「上一份完整周期」报告。
func (s *ReportService) RunReportPlanNow(c *gin.Context, id string) (gin.H, *errs.Error) {
	var p model.ReportPlan
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return nil, be
	}
	// 手动触发放宽时点：仅按周期日匹配，忽略 gen_time
	cp := p
	cp.GenTime = "00:00"
	start, end, label, ok := duePeriod(&cp, time.Now())
	if !ok {
		return nil, errs.ErrParam.WithMsg("今天不是该计划的生成日（" + cycleTextOf(&p) + "）")
	}
	out, be := s.createReport(p.CommunityID, p.PatrolType, start, end, label, nil, nil, nil, &p.ID)
	if be != nil {
		return nil, be
	}
	s.db.Model(&p).Updates(map[string]any{"last_period": label, "last_error": ""})
	return out, nil
}
