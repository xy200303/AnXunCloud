package service

import (
	"testing"
	"time"

	insmodel "anxuncloud/internal/module/inspection/model"
)

// TestExpiredState 逾期口径：已翻转 overdue 原样；未完成且窗口（或回落规则）已过 → expired_doing；
// 窗口未过/已完成/无窗口不误判。判定细节（12 场景）由 insmodel.ShouldOverdue 单测覆盖。
func TestExpiredState(t *testing.T) {
	day := time.Date(2026, 8, 19, 0, 0, 0, 0, time.Local)
	at := func(d, h, m int) time.Time { return time.Date(2026, 8, d, h, m, 0, 0, time.Local) }
	cases := []struct {
		scene  string
		status string
		window string
		now    time.Time
		want   string
	}{
		{"窗口已过 pending 动态逾期", insmodel.TaskPending, "08:00-10:00", at(19, 10, 1), "expired_doing"},
		{"窗口已过 doing 动态逾期", insmodel.TaskDoing, "08:00-10:00", at(19, 10, 1), "expired_doing"},
		{"恰在结束时刻不算逾期", insmodel.TaskPending, "08:00-10:00", at(19, 10, 0), ""},
		{"窗口内不计逾期", insmodel.TaskPending, "08:00-10:00", at(19, 9, 0), ""},
		{"已完成不做动态判定", insmodel.TaskDone, "08:00-10:00", at(19, 11, 0), ""},
		{"已翻转 overdue 原样", insmodel.TaskOverdue, "08:00-10:00", at(19, 11, 0), insmodel.TaskOverdue},
		{"跨零点窗口内不误判（夜班次日 00:10）", insmodel.TaskPending, "19:00-07:00", at(20, 0, 10), ""},
		{"跨零点窗口结束已过", insmodel.TaskPending, "19:00-07:00", at(20, 7, 1), "expired_doing"},
		{"无窗口快照回落日期规则（次日）", insmodel.TaskPending, "", at(20, 0, 10), "expired_doing"},
		{"无窗口快照当天不误判", insmodel.TaskPending, "", at(19, 12, 0), ""},
	}
	for _, c := range cases {
		if got := expiredState(c.status, day, c.window, c.now); got != c.want {
			t.Errorf("%s: expiredState(%q, %q, %s) = %q, 期望 %q",
				c.scene, c.status, c.window, c.now.Format("01-02 15:04"), got, c.want)
		}
	}
}
