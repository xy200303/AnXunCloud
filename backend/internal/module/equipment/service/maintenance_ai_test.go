package service

import (
	"strings"
	"testing"
	"time"
)

// TestJudgeLabelTrust AI 标签核验可信判定（纯函数，方案决策 4）：
// pass 且维修年月（W）与维保日期年月差 ≤1 个月 → 可信自动生效；
// 生产年月（M）与台账出厂年月不符 → 疑似设备更换转人工；review/读不出/解析失败 → 兜底转人工。
func TestJudgeLabelTrust(t *testing.T) {
	maint := time.Date(2026, 3, 15, 0, 0, 0, 0, time.Local)
	manufacture := time.Date(2020, 5, 1, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name        string
		verdict     string
		reading     string
		manufacture *time.Time
		wantTrusted bool
		wantSuspect bool
	}{
		{"同月可信", "pass", "M2020-05|W2026-03", &manufacture, true, false},
		{"差一月可信（容差内）", "pass", "M2020-05|W2026-02", &manufacture, true, false},
		{"晚一月可信（容差内）", "pass", "M2020-05|W2026-04", &manufacture, true, false},
		{"差两月不可信", "pass", "M2020-05|W2026-01", &manufacture, false, false},
		{"生产年月不符疑似更换", "pass", "M2021-08|W2026-03", &manufacture, false, true},
		{"台账无出厂日期不比对", "pass", "M2021-08|W2026-03", nil, true, false},
		{"生产年月读不出不比对", "pass", "M无|W2026-03", &manufacture, true, false},
		{"维修年月读不出兜底", "pass", "M2020-05|W无", &manufacture, false, false},
		{"全部读不出兜底", "pass", "M无|W无", &manufacture, false, false},
		{"reading 解析失败兜底", "pass", "标签模糊无法识别", &manufacture, false, false},
		{"reading 为空兜底", "pass", "", &manufacture, false, false},
		{"review 不可信", "review", "M2020-05|W2026-03", &manufacture, false, false},
		{"空结论不可信", "", "M2020-05|W2026-03", &manufacture, false, false},
	}
	for _, c := range cases {
		trusted, suspect, note := JudgeLabelTrust(c.verdict, c.reading, maint, c.manufacture)
		if trusted != c.wantTrusted || suspect != c.wantSuspect {
			t.Errorf("%s: got trusted=%v suspect=%v, want trusted=%v suspect=%v",
				c.name, trusted, suspect, c.wantTrusted, c.wantSuspect)
		}
		if suspect && !strings.Contains(note, "疑似设备更换") {
			t.Errorf("%s: 疑似更换须标注，note=%q", c.name, note)
		}
		if !suspect && note != "" {
			t.Errorf("%s: 非疑似更换不应带标注，note=%q", c.name, note)
		}
	}
}

// TestJudgeLabelTrustCrossYear 跨年年月差按月计（2025-12 与 2026-01 差 1 个月）。
func TestJudgeLabelTrustCrossYear(t *testing.T) {
	maint := time.Date(2026, 1, 10, 0, 0, 0, 0, time.Local)
	if trusted, _, _ := JudgeLabelTrust("pass", "M2020-05|W2025-12", maint, nil); !trusted {
		t.Fatal("跨年差一月应在容差内")
	}
	if trusted, _, _ := JudgeLabelTrust("pass", "M2020-05|W2025-11", maint, nil); trusted {
		t.Fatal("跨年差两月应不可信")
	}
}
