package model

import "testing"

// TestItemResultOf 逐项三态显式赋值口径（迁移 00066 回填同款规则）：
// exception_type 非空=escaped（逃生，没检成）；否则 pass=false=abnormal；其余 normal。
func TestItemResultOf(t *testing.T) {
	cases := []struct {
		name          string
		exceptionType string
		pass          bool
		want          string
	}{
		{"合格正常", "", true, ItemResultNormal},
		{"不合格异常", "", false, ItemResultAbnormal},
		{"设备不存在逃生", "device_missing", false, ItemResultEscaped},
		{"无法拍摄逃生", "unable_to_capture", false, ItemResultEscaped},
		{"标签磨损逃生", "label_missing", false, ItemResultEscaped},
		// 逃生优先于 pass（提交侧已拦 pass=true+exception_type 的组合，这里防御兜底）
		{"逃生优先于合格标记", "device_missing", true, ItemResultEscaped},
	}
	for _, c := range cases {
		if got := ItemResultOf(c.exceptionType, c.pass); got != c.want {
			t.Errorf("%s: ItemResultOf(%q, %v)=%q, want %q", c.name, c.exceptionType, c.pass, got, c.want)
		}
	}
}

// TestCheckinRecordItemEscaped 逃生态判定 helper。
func TestCheckinRecordItemEscaped(t *testing.T) {
	if (CheckinRecordItem{Result: ItemResultNormal}).Escaped() {
		t.Error("normal 不应判 escaped")
	}
	if (CheckinRecordItem{Result: ItemResultAbnormal}).Escaped() {
		t.Error("abnormal 不应判 escaped")
	}
	if !(CheckinRecordItem{Result: ItemResultEscaped, ExceptionType: "device_missing"}).Escaped() {
		t.Error("escaped 应判 escaped")
	}
}

// TestHasAIVerdict 记录级 AI 结论空串语义：空=未经 AI/识别未完成，非空=已有结论（pass/review/abnormal/error）。
func TestHasAIVerdict(t *testing.T) {
	if (&CheckinRecord{}).HasAIVerdict() {
		t.Error("空 ai_verdict 不应判已有结论")
	}
	for _, v := range []string{AIVerdictPass, AIVerdictReview, AIVerdictAbnormal, AIVerdictError} {
		if !(&CheckinRecord{AIVerdict: v}).HasAIVerdict() {
			t.Errorf("ai_verdict=%q 应判已有结论", v)
		}
	}
}
