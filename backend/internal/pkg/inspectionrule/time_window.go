package inspectionrule

import (
	"fmt"
	"strings"
	"time"
)

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
		end = end.Add(24 * time.Hour)
	}
	return end, true
}

func ShouldOverdue(taskDate time.Time, timeWindow string, now time.Time) bool {
	if end, ok := RoundWindowEnd(taskDate, timeWindow); ok {
		return now.After(end)
	}
	dayStart := time.Date(taskDate.Year(), taskDate.Month(), taskDate.Day(), 0, 0, 0, 0, time.Local)
	return !dayStart.AddDate(0, 0, 1).After(now)
}
