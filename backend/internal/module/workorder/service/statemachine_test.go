package service

import (
	"testing"
	"time"

	"anxuncloud/internal/module/workorder/model"
)

// TestCanTransit 工单闭环状态机表驱动测试：覆盖正常闭环、分诊驳回、验收退回三条核心路径及非法流转。
func TestCanTransit(t *testing.T) {
	cases := []struct {
		name   string
		action string
		from   string
		wantTo string
		wantOK bool
	}{
		// 正常闭环：上报→分诊通过→派单→完工→验收通过
		{"分诊通过", model.ActionTriagePass, model.OrderReported, model.OrderPendingDispatch, true},
		{"派单", model.ActionDispatch, model.OrderPendingDispatch, model.OrderProcessing, true},
		{"抢单", model.ActionGrab, model.OrderPendingDispatch, model.OrderProcessing, true},
		{"完工提交", model.ActionFinish, model.OrderProcessing, model.OrderPendingConfirm, true},
		{"验收通过闭环", model.ActionConfirmPass, model.OrderPendingConfirm, model.OrderClosed, true},
		// 分诊驳回：待分诊 → 已作废
		{"分诊驳回了作废", model.ActionTriageReject, model.OrderReported, model.OrderClosedInvalid, true},
		// 验收退回：待验收 → 处理中（返工后可再次完工/验收）
		{"验收退回返工", model.ActionConfirmReject, model.OrderPendingConfirm, model.OrderProcessing, true},
		{"退回后再次完工", model.ActionFinish, model.OrderProcessing, model.OrderPendingConfirm, true},
		// 非法流转
		{"待派单不可分诊", model.ActionTriagePass, model.OrderPendingDispatch, "", false},
		{"待分诊不可派单", model.ActionDispatch, model.OrderReported, "", false},
		{"待分诊不可完工", model.ActionFinish, model.OrderReported, "", false},
		{"处理中不可验收", model.ActionConfirmPass, model.OrderProcessing, "", false},
		{"已闭环不可再验收", model.ActionConfirmPass, model.OrderClosed, "", false},
		{"已闭环不可派单", model.ActionDispatch, model.OrderClosed, "", false},
		{"已作废不可派单", model.ActionDispatch, model.OrderClosedInvalid, "", false},
		{"已作废不可分诊", model.ActionTriagePass, model.OrderClosedInvalid, "", false},
		{"待验收不可抢单", model.ActionGrab, model.OrderPendingConfirm, "", false},
		{"未知动作", "noop", model.OrderReported, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTo, gotOK := CanTransit(tc.action, tc.from)
			if gotOK != tc.wantOK {
				t.Fatalf("CanTransit(%s, %s) ok = %v, want %v", tc.action, tc.from, gotOK, tc.wantOK)
			}
			if gotOK && gotTo != tc.wantTo {
				t.Fatalf("CanTransit(%s, %s) to = %s, want %s", tc.action, tc.from, gotTo, tc.wantTo)
			}
		})
	}
}

// TestCanTransitFullLoop 正常闭环全链路顺序推演（每步目标状态作为下一步输入）。
func TestCanTransitFullLoop(t *testing.T) {
	steps := []struct {
		action string
		to     string
	}{
		{model.ActionTriagePass, model.OrderPendingDispatch},
		{model.ActionDispatch, model.OrderProcessing},
		{model.ActionFinish, model.OrderPendingConfirm},
		{model.ActionConfirmPass, model.OrderClosed},
	}
	status := model.OrderReported
	for i, step := range steps {
		to, ok := CanTransit(step.action, status)
		if !ok || to != step.to {
			t.Fatalf("第 %d 步 %s：from %s 期望到 %s，实际 to=%s ok=%v", i+1, step.action, status, step.to, to, ok)
		}
		status = to
	}
	if status != model.OrderClosed {
		t.Fatalf("闭环失败，最终状态 %s", status)
	}
}

// TestSLA SLA 简化口径：urgent 4h / high 24h / normal 72h / low 不限；已闭环/已作废不计超时。
func TestSLA(t *testing.T) {
	base := time.Date(2026, 8, 1, 8, 0, 0, 0, time.Local)
	cases := []struct {
		name     string
		priority string
		status   string
		now      time.Time
		wantOver bool
	}{
		{"urgent 4小时内未超时", "urgent", model.OrderProcessing, base.Add(3 * time.Hour), false},
		{"urgent 超4小时超时", "urgent", model.OrderProcessing, base.Add(5*time.Hour), true},
		{"high 24小时内未超时", "high", model.OrderPendingDispatch, base.Add(20 * time.Hour), false},
		{"high 超24小时超时", "high", model.OrderPendingDispatch, base.Add(25*time.Hour), true},
		{"normal 72小时内未超时", "normal", model.OrderReported, base.Add(70 * time.Hour), false},
		{"normal 超72小时超时", "normal", model.OrderReported, base.Add(73*time.Hour), true},
		{"low 不限期", "low", model.OrderProcessing, base.Add(720*time.Hour), false},
		{"已闭环不计超时", "urgent", model.OrderClosed, base.Add(100*time.Hour), false},
		{"已作废不计超时", "urgent", model.OrderClosedInvalid, base.Add(100*time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SLAOverdue(tc.status, tc.priority, base, tc.now); got != tc.wantOver {
				t.Fatalf("SLAOverdue(%s, %s) = %v, want %v", tc.status, tc.priority, got, tc.wantOver)
			}
		})
	}
	if d := SLADeadline("low", base); d != nil {
		t.Fatalf("low 优先级不应有期望完成时间，得到 %v", d)
	}
	if d := SLADeadline("urgent", base); d == nil || !d.Equal(base.Add(4*time.Hour)) {
		t.Fatalf("urgent 期望完成时间应为上报后 4 小时，得到 %v", d)
	}
}
