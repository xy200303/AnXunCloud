// 巡更达成率统计：以轮次任务（round_name 非空）为最小单元，输出应巡/实巡/逾期/达标判定。
// 口径见《专项巡检与专项检查报告设计方案》§3.2。
package service

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/stats/dto"
	"anxuncloud/internal/pkg/errs"
)

// expiredState 逾期口径：已翻转 overdue → overdue；未完成（pending/doing）且 ShouldOverdue
// 判定窗口（或回落日期规则）已过 → expired_doing（动态判定，区分已翻转）；其余不计逾期。
// 判定逻辑与 scheduler 翻转共用 insmodel.ShouldOverdue（§3.2 规则表），杜绝两套口径。
func expiredState(status string, taskDate time.Time, window string, now time.Time) string {
	if status == insmodel.TaskOverdue {
		return insmodel.TaskOverdue
	}
	if (status == insmodel.TaskPending || status == insmodel.TaskDoing) &&
		insmodel.ShouldOverdue(taskDate, window, now) {
		return "expired_doing"
	}
	return ""
}

// PatrolRounds 巡更达成率统计：汇总 + 按天明细 + 逾期轮次清单 + 达标线判定。
func (s *StatsService) PatrolRounds(c *gin.Context, q *dto.PatrolRoundsQuery) (gin.H, *errs.Error) {
	from, err := time.ParseInLocation("2006-01-02", q.From, time.Local)
	if err != nil {
		return nil, errs.ErrParam.WithMsg("from 格式应为 YYYY-MM-DD")
	}
	to, err := time.ParseInLocation("2006-01-02", q.To, time.Local)
	if err != nil {
		return nil, errs.ErrParam.WithMsg("to 格式应为 YYYY-MM-DD")
	}
	if to.Before(from) {
		return nil, errs.ErrParam.WithMsg("to 不能早于 from")
	}
	if to.Sub(from) > 366*24*time.Hour {
		return nil, errs.ErrParam.WithMsg("日期跨度不能超过 366 天")
	}

	scope := s.db.Model(&insmodel.InspectionTask{}).
		Where("round_name <> '' AND community_id = ? AND task_date >= ? AND task_date <= ?", q.CommunityID, q.From, q.To)
	if q.PlanID != "" {
		scope = scope.Where("plan_id = ?", q.PlanID)
	}
	scope = middleware.ApplyCommunityFilter(scope, c, "inspection_task.community_id")
	var tasks []insmodel.InspectionTask
	if err := scope.Session(&gorm.Session{}).
		Select("id", "plan_id", "task_date", "round_name", "time_window", "inspector_id", "status", "done_points", "total_points").
		Order("task_date, id").Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}

	now := time.Now()
	var should, done, open, overdue int64
	var pointRateSum float64
	var pointRateCnt int64
	type dayAgg struct{ should, done, overdue int64 }
	days := map[string]*dayAgg{}
	dayOrder := make([]string, 0)
	overdueList := make([]gin.H, 0)
	planIDs := map[string]bool{}
	for _, t := range tasks {
		planIDs[t.PlanID] = true
		should++
		day := t.TaskDate.Format("2006-01-02")
		da := days[day]
		if da == nil {
			da = &dayAgg{}
			days[day] = da
			dayOrder = append(dayOrder, day)
		}
		da.should++
		if t.TotalPoints > 0 {
			pointRateSum += float64(t.DonePoints) / float64(t.TotalPoints)
			pointRateCnt++
		}
		switch t.Status {
		case insmodel.TaskDone:
			done++
			da.done++
		case insmodel.TaskPending, insmodel.TaskDoing:
			open++
		}
		// 逾期：状态已翻转 overdue，或动态判定窗口已过仍 pending/doing（标 expired_doing 区分）
		state := expiredState(t.Status, t.TaskDate, t.TimeWindow, now)
		if state != "" {
			overdue++
			da.overdue++
			overdueList = append(overdueList, gin.H{
				"task_id": t.ID, "task_date": day, "round_name": t.RoundName,
				"time_window": t.TimeWindow, "inspector_name": s.userName(t.InspectorID),
				"done_points": t.DonePoints, "total_points": t.TotalPoints, "state": state,
			})
		}
	}

	// 平均单轮点位完成率（仅统计 total_points>0 的轮次，保留 1 位小数）
	avgPointCompletion := float64(0)
	if pointRateCnt > 0 {
		avgPointCompletion = float64(int(pointRateSum/float64(pointRateCnt)*1000+0.5)) / 10
	}

	minRounds := s.resolveDailyMinRounds(q.PlanID, planIDs)
	daily := make([]gin.H, 0, len(dayOrder))
	for _, day := range dayOrder {
		da := days[day]
		row := gin.H{
			"date": day, "should_rounds": da.should, "done_rounds": da.done,
			"overdue_rounds": da.overdue, "achievement_rate": pct(da.done, da.should),
		}
		// 达标线：配置了才给判定（实巡 >= daily_min_rounds），否则为 null
		if minRounds != nil {
			row["met"] = da.done >= int64(*minRounds)
		} else {
			row["met"] = nil
		}
		daily = append(daily, row)
	}

	return gin.H{
		"summary": gin.H{
			"should_rounds": should, "done_rounds": done, "open_rounds": open,
			"overdue_rounds": overdue, "achievement_rate": pct(done, should),
			"avg_point_completion": avgPointCompletion, "daily_min_rounds": minRounds,
		},
		"daily":        daily,
		"overdue_list": overdueList,
	}, nil
}

// resolveDailyMinRounds 解析每日达标轮次线：指定 plan_id 时取该计划 cycle_config.daily_min_rounds；
// 汇总口径（plan_id 空）下涉及计划的配置值唯一时采用，多值歧义或未配置则不设线（返回 nil）。
func (s *StatsService) resolveDailyMinRounds(planID string, planIDs map[string]bool) *int {
	if planID != "" {
		var p insmodel.InspectionPlan
		if s.db.Select("cycle_config").First(&p, "id = ?", planID).Error == nil {
			return insmodel.PlanDailyMinRounds(p.CycleConfig)
		}
		return nil
	}
	if len(planIDs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(planIDs))
	for id := range planIDs {
		ids = append(ids, id)
	}
	var plans []insmodel.InspectionPlan
	s.db.Select("id", "cycle_config").Where("id IN ?", ids).Find(&plans)
	vals := map[int]bool{}
	for _, p := range plans {
		if v := insmodel.PlanDailyMinRounds(p.CycleConfig); v != nil {
			vals[*v] = true
		}
	}
	if len(vals) != 1 {
		return nil
	}
	for v := range vals {
		return &v
	}
	return nil
}
