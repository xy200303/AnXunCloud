package service

import (
	"testing"

	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/types"
)

func aiStep(onPass, onAbnormal, onReview string) types.FlowStep {
	return types.FlowStep{Kind: sysmodel.FlowStepKindAI, Name: "AI 审核", OnPass: onPass, OnAbnormal: onAbnormal, OnReview: onReview}
}

func humanStep(slot string) types.FlowStep {
	return types.FlowStep{Slot: slot, Name: slot}
}

// RouteAIStep：三分支缺省值（无异常/有异常 → finish；存疑 → next）保持旧行为。
func TestRouteAIStepDefaults(t *testing.T) {
	flow := types.FlowStepArray{aiStep("", "", ""), humanStep("a")}
	d := RouteAIStep(flow, 0, sysmodel.AIGatePass)
	if !d.Finish {
		t.Fatalf("pass 缺省应直接生效，got %+v", d)
	}
	d = RouteAIStep(flow, 0, sysmodel.AIGateAbnormal)
	if !d.Finish {
		t.Fatalf("abnormal 缺省应直接生效（异常是巡检成果），got %+v", d)
	}
	d = RouteAIStep(flow, 0, sysmodel.AIGateReview)
	if d.Finish || d.Reject || d.NextIdx != 1 {
		t.Fatalf("review 缺省应进下一环节，got %+v", d)
	}
}

// RouteAIStep：自定义路由 finish/next/reject/goto:N。
func TestRouteAIStepCustom(t *testing.T) {
	flow := types.FlowStepArray{aiStep("next", "reject", "goto:3"), humanStep("a"), humanStep("b")}
	if d := RouteAIStep(flow, 0, sysmodel.AIGatePass); d.Finish || d.NextIdx != 1 {
		t.Fatalf("on_pass=next 应推进到环节 2，got %+v", d)
	}
	if d := RouteAIStep(flow, 0, sysmodel.AIGateAbnormal); !d.Reject {
		t.Fatalf("on_abnormal=reject 应直接打回，got %+v", d)
	}
	if d := RouteAIStep(flow, 0, sysmodel.AIGateReview); d.NextIdx != 2 {
		t.Fatalf("on_review=goto:3 应推进到环节 3（下标 2），got %+v", d)
	}
}

// RouteAIStep：末位 AI 环节 next 越过末位（调用方兜底人工）。
func TestRouteAIStepOverEnd(t *testing.T) {
	flow := types.FlowStepArray{aiStep("", "", "next")}
	d := RouteAIStep(flow, 0, sysmodel.AIGateReview)
	if d.NextIdx != 1 || d.Finish {
		t.Fatalf("末位 next 应越过末位（NextIdx=1=len），got %+v", d)
	}
}

// RouteAIStep：非 AI 环节防御（按人工通过推进）。
func TestRouteAIStepNonAI(t *testing.T) {
	flow := types.FlowStepArray{humanStep("a"), humanStep("b")}
	if d := RouteAIStep(flow, 0, sysmodel.AIGatePass); d.NextIdx != 1 || d.Finish {
		t.Fatalf("非 AI 环节应按人工通过推进，got %+v", d)
	}
}

// WalkFlow：空流程默认通过；强制人工例外走兜底。
func TestWalkFlowEmpty(t *testing.T) {
	w := WalkFlow(types.FlowStepArray{}, 0, sysmodel.AIGatePass, false)
	if !w.Finish {
		t.Fatalf("空流程应默认通过，got %+v", w)
	}
	w = WalkFlow(types.FlowStepArray{}, 0, sysmodel.AIGateReview, true)
	if w.Finish || !w.Fallback {
		t.Fatalf("空流程+强制人工应兜底，got %+v", w)
	}
}

// WalkFlow：人工首环节 pending；AI 闸门各分支；连续 AI 穿越；无结论停等闸门。
func TestWalkFlowChains(t *testing.T) {
	// 纯人工链
	w := WalkFlow(types.FlowStepArray{humanStep("a"), humanStep("b")}, 0, "", false)
	if w.Finish || w.Step != 0 || w.NotifyIdx != 0 {
		t.Fatalf("人工首环节应 pending 在环节 1 并通知，got %+v", w)
	}
	// [AI] 无异常 → 直接生效
	w = WalkFlow(types.FlowStepArray{aiStep("", "", "")}, 0, sysmodel.AIGatePass, false)
	if !w.Finish || w.Step != 1 {
		t.Fatalf("[AI] 无异常应直接生效，got %+v", w)
	}
	// [AI] 存疑 → 越末位兜底
	w = WalkFlow(types.FlowStepArray{aiStep("", "", "")}, 0, sysmodel.AIGateReview, false)
	if w.Finish || !w.Fallback || w.Step != 0 {
		t.Fatalf("[AI] 存疑应停在原地兜底，got %+v", w)
	}
	// [AI, 主管] 存疑 → 落主管环节并通知
	w = WalkFlow(types.FlowStepArray{aiStep("", "", ""), humanStep("a")}, 0, sysmodel.AIGateReview, false)
	if w.Finish || w.Step != 1 || w.NotifyIdx != 1 {
		t.Fatalf("[AI,主管] 存疑应落环节 2 并通知，got %+v", w)
	}
	// [AI(on_review=reject)] 存疑 → 打回
	w = WalkFlow(types.FlowStepArray{aiStep("", "", "reject")}, 0, sysmodel.AIGateReview, false)
	if !w.Reject {
		t.Fatalf("on_review=reject 应打回，got %+v", w)
	}
	// [AI, AI] 连续闸门同一结论穿越
	w = WalkFlow(types.FlowStepArray{aiStep("", "", ""), aiStep("", "", "")}, 0, sysmodel.AIGatePass, false)
	if !w.Finish {
		t.Fatalf("连续 AI 环节应按同一结论穿越直至生效，got %+v", w)
	}
	// 无结论 → 停 AI 环节等异步闸门
	w = WalkFlow(types.FlowStepArray{aiStep("", "", "")}, 0, "", false)
	if !w.NeedGate || w.Finish {
		t.Fatalf("无结论应停在 AI 环节等闸门，got %+v", w)
	}
	// goto 跳入人工环节
	w = WalkFlow(types.FlowStepArray{aiStep("goto:3", "", ""), humanStep("a"), humanStep("b")}, 0, sysmodel.AIGatePass, false)
	if w.Finish || w.Step != 2 || w.NotifyIdx != 2 {
		t.Fatalf("goto:3 应落环节 3，got %+v", w)
	}
}

// validateAIRoute：goto 目标合法性（向后、存在、非 AI）。
func TestValidateAIRoute(t *testing.T) {
	steps := types.FlowStepArray{aiStep("", "", ""), humanStep("a"), aiStep("", "", ""), humanStep("b")}
	if be := validateAIRoute(steps, 0, "goto:2", "无异常", false); be != nil {
		t.Fatalf("合法向后跳转应通过，got %v", be)
	}
	if be := validateAIRoute(steps, 0, "goto:1", "无异常", false); be == nil {
		t.Fatal("原地/向前跳转应拒绝")
	}
	if be := validateAIRoute(steps, 0, "goto:9", "无异常", false); be == nil {
		t.Fatal("越界跳转应拒绝")
	}
	if be := validateAIRoute(steps, 0, "goto:3", "无异常", false); be == nil {
		t.Fatal("跳转到 AI 环节应拒绝")
	}
	if be := validateAIRoute(steps, 0, "reject", "无异常", false); be == nil {
		t.Fatal("无异常分支不允许打回")
	}
	if be := validateAIRoute(steps, 0, "reject", "有异常", true); be != nil {
		t.Fatalf("有异常分支允许打回，got %v", be)
	}
	if be := validateAIRoute(steps, 0, "bogus", "存疑", true); be == nil {
		t.Fatal("非法路由值应拒绝")
	}
}
