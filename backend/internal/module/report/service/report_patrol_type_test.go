package service

import (
	"testing"

	sysmodel "anxuncloud/internal/module/system/model"
)

// TestReportTitle 综合月报标题保持现状格式（回归）；非法期间走兜底拼接。
func TestReportTitle(t *testing.T) {
	if got := reportTitle("锦绣华庭", "2026-08"); got != "锦绣华庭2026年8月月度巡检工作报告" {
		t.Fatalf("综合月报标题不符预期: %s", got)
	}
	if got := reportTitle("锦绣华庭", "bad"); got != "锦绣华庭bad月度巡检工作报告" {
		t.Fatalf("非法期间兜底标题不符预期: %s", got)
	}
}

// TestSpecialReportTitle 专项检查报告标题：{小区名}{YYYY年M月}{类型名}专项检查报告；
// label 尾部「专项巡查/专项/巡查」裁掉再拼，避免叠字；其他 label 原样拼接。
func TestSpecialReportTitle(t *testing.T) {
	cases := []struct {
		label, want string
	}{
		{"设备设施专项巡查", "锦绣华庭2026年8月设备设施专项检查报告"},
		{"消防设施专项", "锦绣华庭2026年8月消防设施专项检查报告"},
		{"环境巡查", "锦绣华庭2026年8月环境专项检查报告"},
		{"设备设施", "锦绣华庭2026年8月设备设施专项检查报告"}, // 无后缀原样拼接
		{"电梯维保", "锦绣华庭2026年8月电梯维保专项检查报告"}, // 无后缀原样拼接
	}
	for _, c := range cases {
		if got := specialReportTitle("锦绣华庭", "2026-08", c.label); got != c.want {
			t.Errorf("label=%q 标题不符预期: %s，期望 %s", c.label, got, c.want)
		}
	}
	if got := specialReportTitle("锦绣华庭", "bad", "消防设施专项"); got != "锦绣华庭bad消防设施专项检查报告" {
		t.Fatalf("非法期间兜底标题不符预期: %s", got)
	}
}

// TestSupervisorSlot 主管级签字默认槽位：空类型=固定月报主管级槽位（综合月报维持现状）；
// 非空=该类型汇报线槽位解析结果（patrol_report_line.<type> 或回落通用汇报线，由解析器决定）。
func TestSupervisorSlot(t *testing.T) {
	resolveCalled := false
	resolve := func(pt string) string {
		resolveCalled = true
		return "patrol_report_line." + pt
	}
	if got := supervisorSlot("", resolve); got != sysmodel.SlotReportSignSupervisor {
		t.Fatalf("空类型应走固定月报主管级槽位，实际 %s", got)
	}
	if resolveCalled {
		t.Fatal("空类型不应调用汇报线解析器")
	}
	if got := supervisorSlot("fire", resolve); got != "patrol_report_line.fire" {
		t.Fatalf("非空类型应走汇报线槽位，实际 %s", got)
	}
}
