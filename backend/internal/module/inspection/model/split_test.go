package model

import (
	"testing"
	"time"

	"anxuncloud/internal/pkg/types"
)

// TestSplitPointsForDate 均分切块：执行日 × 巡检员二维连续切块，全量覆盖不重不漏。
func TestSplitPointsForDate(t *testing.T) {
	local := time.FixedZone("CST", 8*3600)
	p := &InspectionPlan{
		CycleType:    "monthly",
		CycleConfig:  types.JSONMap{"days": []any{float64(1), float64(2), float64(3)}},
		InspectorIDs: types.IDArray{"u1", "u2"},
		AssignMode:   AssignSplit,
	}
	points := make([]string, 120)
	for i := range points {
		points[i] = string(rune('a'+i%26)) + string(rune('A'+i/26))
	}
	seen := map[string]int{}
	total := 0
	for day := 1; day <= 3; day++ {
		date := time.Date(2026, 8, day, 0, 0, 0, 0, local)
		for _, uid := range []string{"u1", "u2"} {
			chunk := SplitPointsForDate(p, points, date, uid)
			if len(chunk) != 20 {
				t.Errorf("day=%d uid=%s 块大小=%d，期望 20", day, uid, len(chunk))
			}
			total += len(chunk)
			for _, id := range chunk {
				seen[id]++
			}
		}
	}
	if total != 120 {
		t.Fatalf("总切块数=%d，期望 120", total)
	}
	for _, id := range points {
		if seen[id] != 1 {
			t.Fatalf("点位 %s 被分配 %d 次，期望恰好 1 次", id, seen[id])
		}
	}
	// 连续性：day1/u1 应取前 20 个
	chunk := SplitPointsForDate(p, points, time.Date(2026, 8, 1, 0, 0, 0, 0, local), "u1")
	if chunk[0] != points[0] || chunk[19] != points[19] {
		t.Errorf("day1/u1 块应为 points[0:20]，实际 %v..%v", chunk[0], chunk[19])
	}
	// all 模式原样返回；非执行日原样返回
	p.AssignMode = AssignAll
	if got := SplitPointsForDate(p, points, time.Date(2026, 8, 1, 0, 0, 0, 0, local), "u1"); len(got) != 120 {
		t.Errorf("all 模式应返回全量，实际 %d", len(got))
	}
	p.AssignMode = AssignSplit
	if got := SplitPointsForDate(p, points, time.Date(2026, 8, 9, 0, 0, 0, 0, local), "u1"); len(got) != 120 {
		t.Errorf("非执行日应返回全量，实际 %d", len(got))
	}
}

// TestPlanExecDaysMonthly 月末 -1 解析与超出当月天数剔除（2 月）。
func TestPlanExecDaysMonthly(t *testing.T) {
	local := time.FixedZone("CST", 8*3600)
	p := &InspectionPlan{
		CycleType:   "monthly",
		CycleConfig: types.JSONMap{"days": []int{1, -1, 31}}, // 进程内构造的 []int 也要能解析（演示种子场景）
	}
	// 2026-02 仅 28 天：31 剔除，-1→28
	days, idx := PlanExecDays(p, time.Date(2026, 2, 28, 0, 0, 0, 0, local))
	if len(days) != 2 || days[0] != 1 || days[1] != 28 || idx != 1 {
		t.Errorf("2 月末解析异常：days=%v idx=%d", days, idx)
	}
	// 非执行日 idx=-1
	if _, idx := PlanExecDays(p, time.Date(2026, 2, 15, 0, 0, 0, 0, local)); idx != -1 {
		t.Errorf("非执行日 idx=%d，期望 -1", idx)
	}
}
