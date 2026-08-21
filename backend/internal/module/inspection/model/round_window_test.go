package model

import (
	"testing"
	"time"
)

// TestRoundWindowEnd 窗口结束时刻：当天窗口拼任务日期；跨零点（含起止相等）归次日；非法窗口不通过。
func TestRoundWindowEnd(t *testing.T) {
	day := time.Date(2026, 8, 19, 0, 0, 0, 0, time.Local)
	end, ok := RoundWindowEnd(day, "08:00-10:00")
	if !ok || end.Hour() != 10 || end.Day() != 19 {
		t.Fatalf("当天窗口结束时刻不符预期: %v ok=%v", end, ok)
	}
	end, ok = RoundWindowEnd(day, "19:00-07:00")
	if !ok || end.Day() != 20 || end.Hour() != 7 {
		t.Fatalf("跨零点窗口结束应归次日: %v ok=%v", end, ok)
	}
	// 起止相等视为 24 小时窗口（保存校验会拒绝新配置，存量数据仍按此兜底）
	end, ok = RoundWindowEnd(day, "07:00-07:00")
	if !ok || end.Day() != 20 || end.Hour() != 7 {
		t.Fatalf("起止相等应按 24h 窗口归次日: %v ok=%v", end, ok)
	}
	if _, ok = RoundWindowEnd(day, "8点-10点"); ok {
		t.Fatal("非法窗口应返回 ok=false")
	}
	if _, ok = RoundWindowEnd(day, "25:00-26:00"); ok {
		t.Fatal("越界时刻应返回 ok=false")
	}
	if _, ok = RoundWindowEnd(day, ""); ok {
		t.Fatal("空窗口应返回 ok=false")
	}
}

// TestShouldOverdue 逾期判定全覆盖《专项巡检与专项检查报告设计方案》§3.2「时间场景判定规则表」12 种场景。
// 任务日期统一取 D=2026-08-19（本地时区，规则表 #12）。
func TestShouldOverdue(t *testing.T) {
	day := time.Date(2026, 8, 19, 0, 0, 0, 0, time.Local)
	at := func(d, h, m int) time.Time { return time.Date(2026, 8, d, h, m, 0, 0, time.Local) }
	cases := []struct {
		scene      string // 规则表场景编号与名称
		taskDate   time.Time
		window     string
		now        time.Time
		wantOverdue bool
	}{
		// #1 普通轮次，窗口内：正常可执行
		{"#1 普通轮次窗口内", day, "08:00-10:00", at(19, 9, 0), false},
		// #2 普通轮次，窗口已过未完成：实质逾期
		{"#2 普通轮次窗口已过", day, "08:00-10:00", at(19, 10, 1), true},
		{"#2 恰在窗口结束时刻不算逾期", day, "08:00-10:00", at(19, 10, 0), false},
		// #3 跨零点轮次，窗口内（含 00:10 例行翻转时刻不得误伤）
		{"#3 跨零点窗口内（次日凌晨）", day, "19:00-07:00", at(20, 2, 0), false},
		{"#3 跨零点 00:10 例行翻转不误伤", day, "19:00-07:00", at(20, 0, 10), false},
		// #4 跨零点轮次，窗口已过：实质逾期
		{"#4 跨零点窗口已过", day, "19:00-07:00", at(20, 7, 1), true},
		// #5 未来任务（提前 N 天生成）：一律不参与逾期判定
		{"#5 未来轮次任务", day.AddDate(0, 0, 1), "08:00-10:00", at(19, 12, 0), false},
		{"#5 未来无窗口任务", day.AddDate(0, 0, 1), "", at(19, 12, 0), false},
		// #6 非轮次任务（无窗口快照）：回落 task_date+1天 <= now（存量行为）
		{"#6 无窗口昨日任务次日 00:10 逾期", day, "", at(20, 0, 10), true},
		{"#6 无窗口当天任务不逾期", day, "", at(19, 23, 59), false},
		{"#6 非法窗口回落日期规则", day, "8点-10点", at(20, 0, 10), true},
		// #7 计划顶层 time_window 跨零点：与轮次同规则（结束归次日）
		{"#7 顶层跨零点窗口 00:10 不误伤", day, "19:00-07:00", at(20, 0, 10), false},
		{"#7 顶层跨零点窗口结束已过", day, "19:00-07:00", at(20, 7, 1), true},
		// #8 起止相等：保存校验拒绝（见 validateRounds 测试）；存量按 24h 窗口兜底
		{"#8 起止相等按 24h 窗口兜底", day, "07:00-07:00", at(20, 6, 59), false},
		{"#8 起止相等 24h 后逾期", day, "07:00-07:00", at(20, 7, 1), true},
		// #9 统计日期段含今天：今天白班窗口未过不计逾期
		{"#9 今天白班窗口内", day, "08:00-10:00", at(19, 9, 0), false},
		// #10 夜班任务按天归属：任务日期=开始日，执行中（当晚）不逾期
		{"#10 夜班开始日当晚执行中", day, "19:00-07:00", at(19, 20, 0), false},
		// #11 翻转条件统一：窗口结束时刻已过且未完成 → 逾期（白班任务次日 00:10 正常翻转）
		{"#11 白班任务次日 00:10 正常翻转", day, "08:00-10:00", at(20, 0, 10), true},
	}
	for _, c := range cases {
		if got := ShouldOverdue(c.taskDate, c.window, c.now); got != c.wantOverdue {
			t.Errorf("%s: ShouldOverdue(%s, %q, %s) = %v, 期望 %v",
				c.scene, c.taskDate.Format("01-02"), c.window, c.now.Format("01-02 15:04"), got, c.wantOverdue)
		}
	}
}
