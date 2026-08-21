package service

import (
	"strings"
	"testing"

	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/types"
)

// TestPlanRounds 轮次配置解析：未配置/结构非法返回 nil，正常配置原样解析。
func TestPlanRounds(t *testing.T) {
	if got := model.PlanRounds(types.JSONMap{}); got != nil {
		t.Fatalf("未配置 rounds 应返回 nil，实际 %v", got)
	}
	if got := model.PlanRounds(types.JSONMap{"rounds": "bad"}); got != nil {
		t.Fatalf("rounds 非数组应返回 nil，实际 %v", got)
	}
	cfg := types.JSONMap{"interval": 1, "rounds": []any{
		map[string]any{"name": "早班-1", "window": "08:00-10:00"},
		map[string]any{"name": "晚班", "window": "22:00-02:00"},
	}}
	got := model.PlanRounds(cfg)
	if len(got) != 2 || got[0].Name != "早班-1" || got[0].Window != "08:00-10:00" || got[1].Window != "22:00-02:00" {
		t.Fatalf("rounds 解析不符预期: %+v", got)
	}
}

// TestValidateRounds 轮次校验：monthly 拒绝、名称长度/重复、window 格式、跨零点放行。
func TestValidateRounds(t *testing.T) {
	ok := []model.Round{{Name: "早班-1", Window: "08:00-10:00"}, {Name: "夜班", Window: "22:00-02:00"}}
	if be := validateRounds("daily", ok); be != nil {
		t.Fatalf("daily+合法 rounds 应通过: %v", be)
	}
	if be := validateRounds("weekly", ok); be != nil {
		t.Fatalf("weekly+合法 rounds 应通过: %v", be)
	}
	if be := validateRounds("monthly", ok); be == nil {
		t.Fatal("monthly+rounds 应报参数错误")
	}
	if be := validateRounds("daily", []model.Round{{Name: strings.Repeat("巡", 33), Window: "08:00-10:00"}}); be == nil {
		t.Fatal("轮次名称超过 32 字应报参数错误")
	}
	if be := validateRounds("daily", []model.Round{{Name: "早班", Window: "8点-10点"}}); be == nil {
		t.Fatal("window 非 HH:MM-HH:MM 应报参数错误")
	}
	if be := validateRounds("daily", []model.Round{{Name: "全天", Window: "07:00-07:00"}}); be == nil {
		t.Fatal("window 起止相等应报参数错误（规则表 #8）")
	}
	dup := []model.Round{{Name: "早班", Window: "08:00-10:00"}, {Name: "早班", Window: "11:00-13:00"}}
	if be := validateRounds("daily", dup); be == nil {
		t.Fatal("轮次重名应报参数错误")
	}
	if be := validateRounds("monthly", nil); be != nil {
		t.Fatalf("未配 rounds 任意周期应通过: %v", be)
	}
}

// TestPlanDailyMinRounds 每日达标轮次线：未配置/负数/非整数返回 nil，非负整数原样解析。
func TestPlanDailyMinRounds(t *testing.T) {
	if got := model.PlanDailyMinRounds(types.JSONMap{}); got != nil {
		t.Fatalf("未配置 daily_min_rounds 应返回 nil，实际 %v", *got)
	}
	if got := model.PlanDailyMinRounds(types.JSONMap{"daily_min_rounds": float64(-1)}); got != nil {
		t.Fatalf("负数应返回 nil，实际 %v", *got)
	}
	if got := model.PlanDailyMinRounds(types.JSONMap{"daily_min_rounds": float64(1.5)}); got != nil {
		t.Fatalf("非整数应返回 nil，实际 %v", *got)
	}
	if got := model.PlanDailyMinRounds(types.JSONMap{"daily_min_rounds": "3"}); got != nil {
		t.Fatalf("非数字类型应返回 nil，实际 %v", *got)
	}
	if got := model.PlanDailyMinRounds(types.JSONMap{"daily_min_rounds": float64(3)}); got == nil || *got != 3 {
		t.Fatalf("合法配置应解析为 3，实际 %v", got)
	}
}

// TestTaskPointIDs 任务点位名单：快照优先，空快照回落计划名单（存量任务兼容）。
func TestTaskPointIDs(t *testing.T) {
	plan := &model.InspectionPlan{PointIDs: types.IDArray{"p1", "p2"}}
	task := &model.InspectionTask{}
	if got := model.TaskPointIDs(task, plan); len(got) != 2 || got[0] != "p1" {
		t.Fatalf("空快照应回落计划名单: %v", got)
	}
	task.PointIDs = types.IDArray{"p3"}
	if got := model.TaskPointIDs(task, plan); len(got) != 1 || got[0] != "p3" {
		t.Fatalf("快照非空应优先读快照: %v", got)
	}
}

// TestTaskTimeWindow 任务时段：快照优先（轮次任务），空回落计划 time_window。
func TestTaskTimeWindow(t *testing.T) {
	plan := &model.InspectionPlan{TimeWindow: "08:00-20:00"}
	task := &model.InspectionTask{}
	if got := model.TaskTimeWindow(task, plan); got != "08:00-20:00" {
		t.Fatalf("空快照应回落计划时段: %q", got)
	}
	task.TimeWindow = "22:00-02:00"
	if got := model.TaskTimeWindow(task, plan); got != "22:00-02:00" {
		t.Fatalf("快照非空应优先读快照: %q", got)
	}
}

// TestRenderBatchPointName 批量建点名称渲染：占位符替换 + 负楼层 B 系命名。
func TestRenderBatchPointName(t *testing.T) {
	cases := []struct {
		pattern, building string
		floor, seq        int
		want              string
	}{
		{"{building}{floor}层消防箱", "1栋", 1, 1, "1栋1层消防箱"},
		{"{building}{floor}层消防箱-{seq}", "2栋", 3, 2, "2栋3层消防箱-2"},
		{"{building}{floor}层灭火器", "1栋", -1, 1, "1栋B1层灭火器"},
		{"{building}{floor}层灭火器", "1栋", -2, 1, "1栋B2层灭火器"},
		{"{floor}层消防栓", "", 5, 1, "5层消防栓"}, // 不挂楼栋时 {building} 渲染为空
	}
	for _, c := range cases {
		if got := renderBatchPointName(c.pattern, c.building, c.floor, c.seq); got != c.want {
			t.Errorf("renderBatchPointName(%q,%q,%d,%d) = %q, 期望 %q", c.pattern, c.building, c.floor, c.seq, got, c.want)
		}
	}
}
