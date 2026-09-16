package service

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
)

// SampleSpotPoints 抽查点位抽样（纯函数）：
//   - 抽样数 N：fixed_count>0 用固定数，否则 ceil(池大小 × ratio_percent/100)（至少 1）；
//     比例与数量均为 0 = 抽查关闭，返回空；N 超过池大小取全池。
//   - 连中保护（no_repeat）：先剔除上月命中点位，剔除后不足 N 再从被剔除者按同策略补足。
//   - longest_unseen：按 lastSeen 升序（从未查过=零值排最前，并列按 ID 稳定序）；
//     random：以 now 纳秒为种子洗牌（同一 now 结果可复现）。
func SampleSpotPoints(pointIDs []string, lastSeen map[string]time.Time, lastMonthHit map[string]bool, cfg model.SpotConfig, now time.Time) []string {
	ids := uniqueIDs(pointIDs)
	if len(ids) == 0 {
		return nil
	}
	n := cfg.FixedCount
	if n <= 0 {
		if cfg.RatioPercent <= 0 {
			return nil
		}
		n = int(math.Ceil(float64(len(ids)) * float64(cfg.RatioPercent) / 100))
		if n < 1 {
			n = 1
		}
	}
	if n >= len(ids) {
		n = len(ids)
	}
	pool, held := ids, []string(nil)
	if cfg.NoRepeat && len(lastMonthHit) > 0 {
		pool = make([]string, 0, len(ids))
		for _, id := range ids {
			if lastMonthHit[id] {
				held = append(held, id)
			} else {
				pool = append(pool, id)
			}
		}
		if len(pool) >= n {
			held = nil // 剔除后已够数，补位名单不再需要
		}
	}
	// 池与补位名单按同一策略排序/洗牌后顺序截取，保证补位者也是该策略下最优先的
	if cfg.Strategy == model.SpotStrategyRandom {
		r := rand.New(rand.NewSource(now.UnixNano()))
		r.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		r.Shuffle(len(held), func(i, j int) { held[i], held[j] = held[j], held[i] })
	} else {
		byUnseen := func(list []string) {
			sort.SliceStable(list, func(i, j int) bool {
				ti, tj := lastSeen[list[i]], lastSeen[list[j]]
				if !ti.Equal(tj) {
					return ti.Before(tj)
				}
				return list[i] < list[j]
			})
		}
		byUnseen(pool)
		byUnseen(held)
	}
	return append(pool, held...)[:n]
}

// generateSpotTasks 抽查计划任务生成（GenerateForDate 的 plan_kind=spotcheck 分支）：
// 点位池按圈选模式实时展开 → 抽样（连中保护取上月本计划任务快照并集）→
// 按执行人展开任务（无轮次概念，round_name 固定「抽查」参与查重；due_date=当月期限日）。
// 抽样为空（池空/抽查关闭）不生成，返回新建任务数。
func (s *PlanService) generateSpotTasks(p *model.InspectionPlan, date time.Time) int {
	if p.CycleType != "monthly" { // 抽查计划只认月度生成日（保存时校验，此处兜底）
		return 0
	}
	pointIDs := s.expandPlanPointIDs(p)
	if len(pointIDs) == 0 {
		return 0
	}
	cfg := model.SpotConfigOf(p.SpotcheckConfig)
	sampled := SampleSpotPoints(pointIDs, spotLastSeen(s.db, pointIDs), s.spotLastMonthHits(p, date), cfg, date)
	if len(sampled) == 0 {
		return 0
	}
	due := cfg.DueDate(date)
	created := 0
	for _, inspectorID := range p.InspectorIDs {
		var cnt int64
		s.db.Model(&model.InspectionTask{}).
			Where("plan_id = ? AND task_date = ? AND inspector_id = ? AND round_name = ?",
				p.ID, date.Format("2006-01-02"), inspectorID, model.SpotRoundName).
			Count(&cnt)
		if cnt > 0 {
			continue
		}
		task := model.InspectionTask{
			TenantID:    p.TenantID, // 冗余列随计划快照（=所属小区租户）
			PlanID:      p.ID,
			CommunityID: p.CommunityID,
			InspectorID: inspectorID,
			PatrolType:  p.PatrolType, // 巡查类型随任务快照
			TaskDate:    date,
			RoundName:   model.SpotRoundName,
			TimeWindow:  p.TimeWindow,
			PointIDs:    sampled,
			DueDate:     &due,
			Status:      model.TaskPending,
			TotalPoints: len(sampled),
		}
		if err := s.db.Create(&task).Error; err != nil {
			continue // uk_task_plan_date_inspector（含轮次维度）兜底，冲突即跳过
		}
		created++
	}
	return created
}

// spotLastSeen 点位最近一次有效打卡时间（从未查过的点位不进 map，抽样侧按零值排最前）。
func spotLastSeen(db *gorm.DB, pointIDs []string) map[string]time.Time {
	type row struct {
		PointID string
		LastAt  time.Time
	}
	var rows []row
	db.Model(&model.CheckinRecord{}).
		Select("point_id, MAX(checkin_time) AS last_at").
		Where("point_id IN ? AND superseded_by IS NULL", pointIDs).
		Group("point_id").Scan(&rows)
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		out[r.PointID] = r.LastAt
	}
	return out
}

// spotLastMonthHits 本计划上月抽查命中集：上月生成的本计划任务点位快照并集（连中保护输入）。
func (s *PlanService) spotLastMonthHits(p *model.InspectionPlan, date time.Time) map[string]bool {
	prevFirst := time.Date(date.Year(), date.Month()-1, 1, 0, 0, 0, 0, time.Local)
	prevLast := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	var tasks []model.InspectionTask
	s.db.Select("point_ids").
		Where("plan_id = ? AND task_date >= ? AND task_date <= ?",
			p.ID, prevFirst.Format("2006-01-02"), prevLast.Format("2006-01-02")).
		Find(&tasks)
	out := map[string]bool{}
	for i := range tasks {
		for _, id := range tasks[i].PointIDs {
			out[id] = true
		}
	}
	return out
}
