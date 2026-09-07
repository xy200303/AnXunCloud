package service

import (
	"testing"

	"anxuncloud/internal/pkg/types"
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

func TestReviewStepHelpers(t *testing.T) {
	steps := types.ReportReviewStepArray{{CandidateIDs: types.IDArray{}}, {CandidateIDs: types.IDArray{"u1"}}}
	if firstReviewStep(steps) != 1 || reviewStatus(steps, 1) != "pending_review" || len(reviewCurrentIDs(steps, 1)) != 1 {
		t.Fatal("动态审核步骤辅助函数结果不符合预期")
	}
}
