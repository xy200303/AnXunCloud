package service

import (
	"testing"
	"time"

	"anxuncloud/internal/module/equipment/model"
)

func eq(id, code string, nextDue *time.Time, warnDays *int) model.Equipment {
	e := model.Equipment{
		Type: "extinguisher", Code: code, Name: "设备" + code,
		NextDueDate: nextDue, WarnDays: warnDays, Status: model.StatusInService,
	}
	e.ID = id
	return e
}

func TestJudgeDevice(t *testing.T) {
	now := day("2026-09-08")

	t.Run("无到期日→no_data 且可补录", func(t *testing.T) {
		j := JudgeDevice(eq("e1", "X-001", nil, nil), false, 30, now)
		if j.State != AutoNoData || !j.ShowRegister {
			t.Fatalf("got %+v", j)
		}
	})
	t.Run("正常", func(t *testing.T) {
		j := JudgeDevice(eq("e1", "X-001", dayPtr("2026-12-01"), nil), false, 30, now)
		if j.State != DueNormal || j.ShowRegister || j.OverdueDays != 0 {
			t.Fatalf("got %+v", j)
		}
	})
	t.Run("临期带登记入口", func(t *testing.T) {
		j := JudgeDevice(eq("e1", "X-001", dayPtr("2026-09-20"), nil), false, 30, now)
		if j.State != DueWarning || !j.ShowRegister {
			t.Fatalf("got %+v", j)
		}
	})
	t.Run("逾期天数与登记入口", func(t *testing.T) {
		j := JudgeDevice(eq("e1", "X-001", dayPtr("2026-09-01"), nil), false, 30, now)
		if j.State != DueOverdue || j.OverdueDays != 7 || !j.ShowRegister {
			t.Fatalf("got %+v", j)
		}
	})
	t.Run("设备级 warn_days 覆盖", func(t *testing.T) {
		warn7 := 7
		j := JudgeDevice(eq("e1", "X-001", dayPtr("2026-09-20"), &warn7), false, 30, now)
		if j.State != DueNormal || j.WarnDays != 7 { // 12 天后到期，设备阈值 7 天 → 尚未临期
			t.Fatalf("got %+v", j)
		}
	})
	t.Run("pending 标记透传", func(t *testing.T) {
		j := JudgeDevice(eq("e1", "X-001", dayPtr("2026-09-01"), nil), true, 30, now)
		if !j.HasPending {
			t.Fatalf("got %+v", j)
		}
	})
}

func TestJudgeDevices(t *testing.T) {
	now := day("2026-09-08")
	out := JudgeDevices([]model.Equipment{
		eq("e1", "X-001", dayPtr("2026-09-01"), nil), // 逾期
		eq("e2", "X-002", dayPtr("2027-01-01"), nil), // 正常
	}, map[string]bool{"e1": true}, 30, now)
	if len(out) != 2 || out[0].State != DueOverdue || !out[0].HasPending || out[1].State != DueNormal || out[1].HasPending {
		t.Fatalf("got %+v", out)
	}
}

func TestSyntheticItemName(t *testing.T) {
	j := DeviceJudge{Name: "1栋1楼灭火器", Code: "MFZ-001"}
	if got := SyntheticItemName(j); got != "设备维保·1栋1楼灭火器(MFZ-001)" {
		t.Fatalf("got %q", got)
	}
}

func TestDeviceJudgeSubmit(t *testing.T) {
	now := day("2026-09-08")
	overdue := DeviceJudge{EquipmentID: "e1", Code: "X-001", State: DueOverdue, NextDueDate: dayPtr("2026-09-01"), OverdueDays: 7}

	t.Run("首次逾期→异常", func(t *testing.T) {
		pass, note := DeviceJudgeSubmit(overdue, true, now)
		if pass || note != "设备维保逾期：编号 X-001 到期日 2026-09-01" {
			t.Fatalf("pass=%v note=%q", pass, note)
		}
	})
	t.Run("持续逾期→催办中不产新异常", func(t *testing.T) {
		pass, note := DeviceJudgeSubmit(overdue, false, now)
		if !pass || note != "设备维保逾期：编号 X-001 到期日 2026-09-01（维保逾期·催办中）" {
			t.Fatalf("pass=%v note=%q", pass, note)
		}
	})
	t.Run("逾期但有 pending 登记→待确认不产异常", func(t *testing.T) {
		j := overdue
		j.HasPending = true
		pass, note := DeviceJudgeSubmit(j, true, now)
		if !pass || note != "设备维保逾期：编号 X-001 到期日 2026-09-01（已登记维保待确认）" {
			t.Fatalf("pass=%v note=%q", pass, note)
		}
	})
	t.Run("no_data→合格+补录备注", func(t *testing.T) {
		pass, note := DeviceJudgeSubmit(DeviceJudge{State: AutoNoData}, false, now)
		if !pass || note != "台账数据缺失待补录" {
			t.Fatalf("pass=%v note=%q", pass, note)
		}
	})
	t.Run("临期→合格+天数提示", func(t *testing.T) {
		pass, note := DeviceJudgeSubmit(DeviceJudge{State: DueWarning, NextDueDate: dayPtr("2026-09-20")}, false, now)
		if !pass || note != "将于 12 天内到期（到期日 2026-09-20）" {
			t.Fatalf("pass=%v note=%q", pass, note)
		}
	})
	t.Run("正常→合格无备注", func(t *testing.T) {
		pass, note := DeviceJudgeSubmit(DeviceJudge{State: DueNormal}, false, now)
		if !pass || note != "" {
			t.Fatalf("pass=%v note=%q", pass, note)
		}
	})
}
