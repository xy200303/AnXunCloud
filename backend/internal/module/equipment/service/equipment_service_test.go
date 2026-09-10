package service

import (
	"testing"
	"time"

	"anxuncloud/internal/pkg/types"
)

func day(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func dayPtr(s string) *time.Time {
	t := day(s)
	return &t
}

func TestDueStatus(t *testing.T) {
	now := day("2026-09-08")
	cases := []struct {
		name     string
		nextDue  *time.Time
		warnDays int
		want     string
	}{
		{"无到期日", nil, 30, DueNone},
		{"正常（超出临期窗口）", dayPtr("2026-11-01"), 30, DueNormal},
		{"临期上边界（30 天后）", dayPtr("2026-10-08"), 30, DueWarning},
		{"临期（今天到期）", dayPtr("2026-09-08"), 30, DueWarning},
		{"逾期（昨天到期）", dayPtr("2026-09-07"), 30, DueOverdue},
		{"阈值覆盖为 7 天", dayPtr("2026-09-20"), 7, DueNormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DueStatus(tc.nextDue, tc.warnDays, now); got != tc.want {
				t.Fatalf("DueStatus() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCalcNextDueDate(t *testing.T) {
	manufacture := day("2021-05-01")
	lastMaint := day("2025-03-10")

	// 有维保记录：lastMaint + cycle_months
	got := CalcNextDueDate(&manufacture, &lastMaint, 60, 24)
	if got == nil || got.Format("2006-01-02") != "2027-03-10" {
		t.Fatalf("有维保记录：got %v, want 2027-03-10", got)
	}
	// 无维保记录：manufacture + first_months（灭火器首保 5 年）
	got = CalcNextDueDate(&manufacture, nil, 60, 24)
	if got == nil || got.Format("2006-01-02") != "2026-05-01" {
		t.Fatalf("首保规则：got %v, want 2026-05-01", got)
	}
	// first_months=0 用 cycle_months
	got = CalcNextDueDate(&manufacture, nil, 0, 12)
	if got == nil || got.Format("2006-01-02") != "2022-05-01" {
		t.Fatalf("first=0 用周期：got %v, want 2022-05-01", got)
	}
	// 都无 → nil
	if got := CalcNextDueDate(nil, nil, 60, 24); got != nil {
		t.Fatalf("无日期数据应返回 nil, got %v", got)
	}
	// 零值规则（cycle=0）→ nil（不自动判到期）
	if got := CalcNextDueDate(&manufacture, nil, 0, 0); got != nil {
		t.Fatalf("零值规则应返回 nil, got %v", got)
	}
}

func TestCalcScrapDate(t *testing.T) {
	manufacture := day("2020-01-15")
	got := CalcScrapDate(&manufacture, 120) // 灭火器 10 年
	if got == nil || got.Format("2006-01-02") != "2030-01-15" {
		t.Fatalf("got %v, want 2030-01-15", got)
	}
	if got := CalcScrapDate(&manufacture, 0); got != nil {
		t.Fatalf("scrap_months=0 应返回 nil, got %v", got)
	}
	if got := CalcScrapDate(nil, 120); got != nil {
		t.Fatalf("无出厂日期应返回 nil, got %v", got)
	}
}

func TestParseTypeRule(t *testing.T) {
	// 灭火器规则（attrs jsonb 读回为 float64）
	r := ParseTypeRule(types.JSONMap{
		"first_months": float64(60), "cycle_months": float64(24),
		"remind": true, "scrap_months": float64(120),
	})
	if r.FirstMonths != 60 || r.CycleMonths != 24 || !r.Remind || r.ScrapMonths != 120 {
		t.Fatalf("灭火器规则解析错误: %+v", r)
	}
	if !r.HasCycle() {
		t.Fatal("有周期应 HasCycle")
	}
	// attrs 空 → 零值规则（不自动判到期、不催办）
	r = ParseTypeRule(nil)
	if r.FirstMonths != 0 || r.CycleMonths != 0 || r.Remind || r.ScrapMonths != 0 {
		t.Fatalf("空 attrs 应为零值规则: %+v", r)
	}
	if r.HasCycle() {
		t.Fatal("零值规则不应 HasCycle")
	}
}

func TestParseFlexibleDate(t *testing.T) {
	cases := []struct {
		in   string
		want string // 空串表示应解析失败（nil）
	}{
		{"2018-05-01", "2018-05-01"},
		{"2018/5/1", "2018-05-01"},
		{"2018.5.1", "2018-05-01"},
		{"2018年5月", "2018-05-01"},
		{"2018年5月2日", "2018-05-02"},
		{"2018-05", "2018-05-01"},
		{" 2018-05-01 ", "2018-05-01"},
		{"43221", "2018-05-01"},   // Excel 序列日期（1899-12-30 原点）
		{"43221.0", "2018-05-01"}, // 序列日期小数形态
		{"2018-05-01 00:00:00", "2018-05-01"},
		{"", ""},
		{"0", ""}, // 甲方台账占位垃圾值
		{"-", ""},
		{"/", ""},
		{"360", ""}, // 数量误入日期列（不在序列日期合理区间）
		{"乱七八糟", ""},
	}
	for _, tc := range cases {
		got := ParseFlexibleDate(tc.in)
		if tc.want == "" {
			if got != nil {
				t.Errorf("ParseFlexibleDate(%q) = %v, want nil", tc.in, got)
			}
			continue
		}
		if got == nil || got.Format("2006-01-02") != tc.want {
			t.Errorf("ParseFlexibleDate(%q) = %v, want %s", tc.in, got, tc.want)
		}
	}
}

func TestLocateHeader(t *testing.T) {
	// 甲方台账结构：第 1 行大标题、第 2 行列表头
	rows := [][]string{
		{"雄楚春天项目设施设备台帐"},
		{"序号", "机房名称", "设备等级", "设备分类", "设备名称", "设备编号"},
		{"1", "水泵房", "一级", "灭火器设施", "1栋灭火器", "XCCT-0001"},
	}
	idx, colOf := locateHeader(rows)
	if idx != 1 {
		t.Fatalf("header 行索引 = %d, want 1", idx)
	}
	if colOf["设备编号"] != 5 {
		t.Fatalf("设备编号列 = %d, want 5", colOf["设备编号"])
	}
	// 无表头
	if idx, _ := locateHeader([][]string{{"a"}, {"b"}}); idx != -1 {
		t.Fatalf("无表头应返回 -1, got %d", idx)
	}
}

func TestJoinRemark(t *testing.T) {
	if got := joinRemark("1栋3楼通道", "1栋", ""); got != "1栋3楼通道 · 1栋" {
		t.Fatalf("got %q", got)
	}
	if got := joinRemark("", "1栋", "弱电系统"); got != "1栋；弱电系统" {
		t.Fatalf("got %q", got)
	}
	if got := joinRemark("1栋", "1栋", ""); got != "1栋" { // 位置与区域相同去重
		t.Fatalf("got %q", got)
	}
	if got := joinRemark("", "", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}
