package model

import (
	"fmt"
	"strings"
	"time"
)

// RoundWindowEnd 轮次/计划窗口结束时刻（《专项巡检与专项检查报告设计方案》§3.2 规则表）：
// 解析 window（HH:MM-HH:MM）结束时刻拼上任务日期；结束 ≤ 开始（跨零点窗口，如 19:00-07:00）
// 结束时刻归次日（起止相等按 24h 窗口处理）。window 非法返回 ok=false。一律服务器本地时区。
func RoundWindowEnd(taskDate time.Time, window string) (time.Time, bool) {
	parts := strings.Split(window, "-")
	if len(parts) != 2 {
		return time.Time{}, false
	}
	parseHM := func(s string) (int, bool) {
		var h, m int
		if n, err := fmt.Sscanf(s, "%d:%d", &h, &m); n != 2 || err != nil {
			return 0, false
		}
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, false
		}
		return h*60 + m, true
	}
	start, ok := parseHM(parts[0])
	if !ok {
		return time.Time{}, false
	}
	endMin, ok := parseHM(parts[1])
	if !ok {
		return time.Time{}, false
	}
	day := time.Date(taskDate.Year(), taskDate.Month(), taskDate.Day(), 0, 0, 0, 0, time.Local)
	end := day.Add(time.Duration(endMin) * time.Minute)
	if endMin <= start {
		end = end.Add(24 * time.Hour) // 跨零点窗口（含起止相等的 24h 窗口）结束时刻归次日
	}
	return end, true
}

// ShouldOverdue 逾期翻转统一判定（scheduler 每日翻转与统计动态判定共用，杜绝两套口径）：
// timeWindow 非空且可解析 → now 已过窗口结束时刻即逾期（夜班 19:00-07:00 任务次日 00:10 不误翻）；
// 否则回落存量行为 task_date+1天 <= now（次日零点起算逾期）。
func ShouldOverdue(taskDate time.Time, timeWindow string, now time.Time) bool {
	if end, ok := RoundWindowEnd(taskDate, timeWindow); ok {
		return now.After(end)
	}
	dayStart := time.Date(taskDate.Year(), taskDate.Month(), taskDate.Day(), 0, 0, 0, 0, time.Local)
	return !dayStart.AddDate(0, 0, 1).After(now)
}
