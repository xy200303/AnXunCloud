// Package timefmt 统一时间格式化（接口文档 §1.6：YYYY-MM-DD HH:mm:ss）。
package timefmt

import "time"

// Layout 统一时间格式。
const Layout = "2006-01-02 15:04:05"

// T 格式化时间；零值返回空串。
func T(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(Layout)
}

// TP 格式化时间指针；nil 返回空串。
func TP(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(Layout)
}

// Day 按本地时区截断到日（到期判定/日期入库统一日粒度；收编自各 service 包内重复的 truncateDay）。
func Day(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// Parse 解析 YYYY-MM-DD HH:mm:ss。
func Parse(s string) (time.Time, error) {
	return time.ParseInLocation(Layout, s, time.Local)
}
