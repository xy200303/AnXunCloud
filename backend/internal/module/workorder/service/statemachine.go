// Package service 工单状态机（纯函数，表驱动，供 service 流转与单元测试共用）。
package service

import (
	"time"

	"anxuncloud/internal/module/workorder/model"
)

// orderTransition 单条流转规则：允许的前置状态集合与目标状态。
type orderTransition struct {
	from []string
	to   string
}

// orderTransitions 工单闭环状态机（设计方案 §6.2）：
//
//	reported --triage_pass--> pending_dispatch --dispatch/grab--> processing
//	reported --triage_reject--> closed_invalid（已作废）
//	processing --finish--> pending_confirm --confirm_pass--> closed
//	pending_confirm --confirm_reject--> processing（退回返工，记录退回原因）
var orderTransitions = map[string]orderTransition{
	model.ActionTriagePass:    {[]string{model.OrderReported}, model.OrderPendingDispatch},
	model.ActionTriageReject:  {[]string{model.OrderReported}, model.OrderClosedInvalid},
	model.ActionDispatch:      {[]string{model.OrderPendingDispatch}, model.OrderProcessing},
	model.ActionGrab:          {[]string{model.OrderPendingDispatch}, model.OrderProcessing},
	model.ActionFinish:        {[]string{model.OrderProcessing}, model.OrderPendingConfirm},
	model.ActionConfirmPass:   {[]string{model.OrderPendingConfirm}, model.OrderClosed},
	model.ActionConfirmReject: {[]string{model.OrderPendingConfirm}, model.OrderProcessing},
}

// CanTransit 校验动作在当前状态下是否允许，允许时返回目标状态。
func CanTransit(action, status string) (string, bool) {
	t, ok := orderTransitions[action]
	if !ok {
		return "", false
	}
	for _, from := range t.from {
		if from == status {
			return t.to, true
		}
	}
	return "", false
}

// slaHours 各优先级期望完成时长（自上报起算）。
// 简化实现：SLA 硬编码常量，项目级 SLA 配置与主动超时推送后续再议（此处仅列表/详情展示是否超时）。
var slaHours = map[string]time.Duration{
	"urgent": 4 * time.Hour,
	"high":   24 * time.Hour,
	"normal": 72 * time.Hour,
	// low 不限期
}

// SLADeadline 期望完成时间；low 或未知优先级返回 nil（不限期）。
func SLADeadline(priority string, createdAt time.Time) *time.Time {
	d, ok := slaHours[priority]
	if !ok {
		return nil
	}
	deadline := createdAt.Add(d)
	return &deadline
}

// SLAOverdue 是否超时：有期望完成时间、未闭环（closed/closed_invalid 不再计超时）且当前已超过。
func SLAOverdue(status, priority string, createdAt, now time.Time) bool {
	if status == model.OrderClosed || status == model.OrderClosedInvalid {
		return false
	}
	deadline := SLADeadline(priority, createdAt)
	return deadline != nil && now.After(*deadline)
}
