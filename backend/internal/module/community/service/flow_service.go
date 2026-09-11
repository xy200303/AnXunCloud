// 审批链解析（《汇报线与审批链扩展设计方案》§3，P2）。
// 流程 = 有序环节列表（flow_code 系统定义，steps 可配）；每个环节引用一个职责槽位，
// 名单解析/授权判定完全复用槽位体系（SlotUserIDs / SlotAuthorized），不引入第二套规则。
package service

import (
	"strconv"
	"strings"

	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

// DefaultCheckinReviewFlow 内置默认打卡审批流程（无任何配置时兜底）：空流程 = 打卡记录默认通过。
// 要人工/AI 审核就在流程里配环节（AI 环节 = 闸门：无问题直接过，有问题流向下游）。
func DefaultCheckinReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{}
}

// DefaultReportReviewFlow 报告审核默认链。它只是无配置时的默认值，项目/租户可以配置任意顺序和数量，
// 也可以完全移除巡检员节点或保存空流程。
func DefaultReportReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{
		{Slot: sysmodel.SlotReportInspector, Name: "巡检员确认", Mode: "all"},
		{Slot: sysmodel.SlotPatrolReportLine, Name: "主管审核", Mode: "any"},
		{Slot: sysmodel.SlotProjectReview, Name: "项目复核", Mode: "any"},
	}
}

// ResolveFlow 解析审批链配置：项目级 → 租户级 → 平台默认 → 内置默认。
// 记录存在即命中（含显式空 = 默认通过），无记录才继续回落；打卡/维保链空流程语义 = 提交即生效。
func ResolveFlow(db *gorm.DB, projectID, flowCode string) types.FlowStepArray {
	var f sysmodel.ApprovalFlow
	if err := db.Where("project_id = ? AND flow_code = ?", projectID, flowCode).First(&f).Error; err == nil {
		return f.Steps
	}
	if tid := middleware.CommunityTenantID(db, projectID); tid != nil {
		if err := db.Where("project_id IS NULL AND tenant_id = ? AND flow_code = ?", *tid, flowCode).First(&f).Error; err == nil {
			return f.Steps
		}
	}
	if err := db.Where("project_id IS NULL AND tenant_id IS NULL AND flow_code = ?", flowCode).First(&f).Error; err == nil {
		return f.Steps
	}
	if flowCode == sysmodel.FlowReportReview {
		return DefaultReportReviewFlow()
	}
	if flowCode == sysmodel.FlowMaintReview {
		return DefaultMaintReviewFlow(db)
	}
	return DefaultCheckinReviewFlow()
}

// ResolveFlowWithSource 解析审批链并给出来源（project/tenant/platform/default），供配置页展示。
func ResolveFlowWithSource(db *gorm.DB, projectID, flowCode string) (types.FlowStepArray, string) {
	var f sysmodel.ApprovalFlow
	if err := db.Where("project_id = ? AND flow_code = ?", projectID, flowCode).First(&f).Error; err == nil {
		return f.Steps, "project"
	}
	if tid := middleware.CommunityTenantID(db, projectID); tid != nil {
		if err := db.Where("project_id IS NULL AND tenant_id = ? AND flow_code = ?", *tid, flowCode).First(&f).Error; err == nil {
			return f.Steps, "tenant"
		}
	}
	if err := db.Where("project_id IS NULL AND tenant_id IS NULL AND flow_code = ?", flowCode).First(&f).Error; err == nil {
		return f.Steps, "platform"
	}
	if flowCode == sysmodel.FlowReportReview {
		return DefaultReportReviewFlow(), "default"
	}
	if flowCode == sysmodel.FlowMaintReview {
		return DefaultMaintReviewFlow(db), "default"
	}
	return DefaultCheckinReviewFlow(), "default"
}

// DefaultMaintReviewFlow 维保登记审核内置默认（无任何配置记录时兜底）：
// equipment.ai_auto_confirm 开 → [AI 闸门]（可信直接生效；其余按 AI 环节默认 OnReview=next 转人工兜底）；
// 关（甲方口径：一律经理确认）→ [项目经理复核]。
func DefaultMaintReviewFlow(db *gorm.DB) types.FlowStepArray {
	var cfg sysmodel.SysConfig
	autoConfirm := false
	if err := db.Select("value").Where("key = ?", "equipment.ai_auto_confirm").First(&cfg).Error; err == nil {
		autoConfirm = cfg.Value == "true"
	}
	if autoConfirm {
		return types.FlowStepArray{{Kind: sysmodel.FlowStepKindAI, Name: "AI 审核"}}
	}
	return types.FlowStepArray{{Slot: sysmodel.SlotProjectReview, Name: "经理确认"}}
}

// WalkResult 通用走链结果（打卡/维保审核链共用）。
type WalkResult struct {
	Finish    bool // 整单通过，流程结束（Step = len(flow)）
	Reject    bool // 整单打回（Step = 当前 AI 环节下标）
	Step      int  // 落定环节下标
	NeedGate  bool // 停在 AI 环节且尚无结论（调用方异步接力）
	NotifyIdx int  // 需通知的人工环节下标（-1 = 不通知）
	Fallback  bool // 通知走兜底（AI 环节越末位/空流程强制人工）
}

// WalkFlow 通用走链：人工环节 → 待审；AI 环节 → 按 outcome 三分支路由（'' = 无结论原地等闸门）；
// 空流程 → Finish（forcedHuman 时 Fallback 兜底人工）。goto 向前跳，必终止。
func WalkFlow(flow types.FlowStepArray, startIdx int, outcome string, forcedHuman bool) WalkResult {
	if len(flow) == 0 {
		if forcedHuman {
			return WalkResult{Step: 0, NotifyIdx: -1, Fallback: true}
		}
		return WalkResult{Finish: true, Step: 0, NotifyIdx: -1}
	}
	idx := startIdx
	for idx < len(flow) {
		st := flow[idx]
		if st.Kind != sysmodel.FlowStepKindAI {
			return WalkResult{Step: idx, NotifyIdx: idx}
		}
		if outcome == "" {
			return WalkResult{Step: idx, NotifyIdx: -1, NeedGate: true}
		}
		d := RouteAIStep(flow, idx, outcome)
		switch {
		case d.Finish:
			return WalkResult{Finish: true, Step: len(flow), NotifyIdx: -1}
		case d.Reject:
			return WalkResult{Reject: true, Step: idx, NotifyIdx: -1}
		case d.NextIdx >= len(flow):
			// 越过末位：停在本环节，兜底人工
			return WalkResult{Step: idx, NotifyIdx: -1, Fallback: true}
		default:
			idx = d.NextIdx
		}
	}
	return WalkResult{Finish: true, Step: len(flow), NotifyIdx: -1}
}
// ValidateReportFlowSteps 报告审核链校验（0-5 环节；不允许 AI 环节——报告签字是人的动作）。
func ValidateReportFlowSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	if len(steps) > 5 {
		return errs.ErrParam.WithMsg("报告审核链最多 5 个环节")
	}
	if len(steps) == 0 {
		return nil
	}
	for _, st := range steps {
		if st.Kind == sysmodel.FlowStepKindAI {
			return errs.ErrParam.WithMsg("报告审核链不支持 AI 环节")
		}
	}
	return validateHumanSteps(db, steps)
}

// FlowStepSlot 环节槽位解析：汇报线通用槽位按巡查类型路由到维度槽位（扩展方案 §2），其余槽位原样使用。
func FlowStepSlot(db *gorm.DB, projectID, patrolType, stepSlot string) string {
	if stepSlot == sysmodel.SlotPatrolReportLine {
		return ResolveReportLineSlot(db, projectID, patrolType)
	}
	return stepSlot
}

// ValidateFlowSteps 打卡/维保审核链环节入参校验（0-5 环节，空 = 默认通过；
// AI 环节免槽位、不参与查重，三分支路由逐值校验（含 goto 目标合法性）；人工环节照旧）。
func ValidateFlowSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	if len(steps) > 5 {
		return errs.ErrParam.WithMsg("审批链最多 5 个环节")
	}
	human := make(types.FlowStepArray, 0, len(steps))
	for i, st := range steps {
		if st.Kind == sysmodel.FlowStepKindAI {
			if len([]rune(strings.TrimSpace(st.Name))) > 32 {
				return errs.ErrParam.WithMsg("环节名称须为 1-32 字")
			}
			if be := validateAIRoute(steps, i, st.OnPass, "无异常", false); be != nil {
				return be
			}
			if be := validateAIRoute(steps, i, st.OnAbnormal, "有异常", true); be != nil {
				return be
			}
			if be := validateAIRoute(steps, i, st.OnReview, "存疑", true); be != nil {
				return be
			}
			continue
		}
		human = append(human, st)
	}
	return validateHumanSteps(db, human)
}

// validateAIRoute AI 分支路由值校验：''（默认）/finish/next/reject/goto:N（N 须指向后面的环节，防环）。
func validateAIRoute(steps types.FlowStepArray, idx int, route, label string, allowReject bool) *errs.Error {
	if route == "" {
		return nil
	}
	if strings.HasPrefix(route, sysmodel.FlowRouteGotoPrefix) {
		n, err := strconv.Atoi(strings.TrimPrefix(route, sysmodel.FlowRouteGotoPrefix))
		if err != nil || n < 1 || n > len(steps) {
			return errs.ErrParam.WithMsg("AI 环节「" + label + "」跳转目标无效")
		}
		if n <= idx+1 {
			return errs.ErrParam.WithMsg("AI 环节「" + label + "」只能向后跳转")
		}
		if steps[n-1].Kind == sysmodel.FlowStepKindAI {
			return errs.ErrParam.WithMsg("AI 环节「" + label + "」不能跳转到另一个 AI 环节")
		}
		return nil
	}
	if route == sysmodel.FlowRouteFinish || route == sysmodel.FlowRouteNext {
		return nil
	}
	if route == sysmodel.FlowRouteReject && allowReject {
		return nil
	}
	return errs.ErrParam.WithMsg("AI 环节「" + label + "」去向配置无效")
}

// RouteDecision 环节路由结果（纯函数；调用方负责落库/通知）。
type RouteDecision struct {
	Finish  bool // 整单通过，流程结束
	Reject  bool // 整单打回
	NextIdx int  // 推进到该环节下标；>= len(flow) 表示越过末位（无下游：调用方按汇报线兜底人工处理，记录停在本环节）
}

// RouteHumanPass 人工环节通过：推进下一环节；末环节 → Finish。
func RouteHumanPass(flow types.FlowStepArray, idx int) RouteDecision {
	if idx+1 >= len(flow) {
		return RouteDecision{Finish: true, NextIdx: len(flow)}
	}
	return RouteDecision{NextIdx: idx + 1}
}

// RouteAIStep AI 闸门路由：outcome ∈ AIGatePass/AIGateAbnormal/AIGateReview。
// 缺省保持旧行为：无异常/有异常 → finish（直接生效）；存疑 → next（转人工）。
// goto:N 已在校验期保证合法（向后、非 AI 环节），这里直接按目标下标推进。
func RouteAIStep(flow types.FlowStepArray, idx int, outcome string) RouteDecision {
	if idx < 0 || idx >= len(flow) || flow[idx].Kind != sysmodel.FlowStepKindAI {
		// 防御：非 AI 环节按人工通过推进
		return RouteHumanPass(flow, idx)
	}
	step := flow[idx]
	route := ""
	switch outcome {
	case sysmodel.AIGatePass:
		route = step.OnPass
		if route == "" {
			route = sysmodel.FlowRouteFinish
		}
	case sysmodel.AIGateAbnormal:
		route = step.OnAbnormal
		if route == "" {
			route = sysmodel.FlowRouteFinish
		}
	default: // review / error / 强制人工
		route = step.OnReview
		if route == "" {
			route = sysmodel.FlowRouteNext
		}
	}
	if strings.HasPrefix(route, sysmodel.FlowRouteGotoPrefix) {
		if n, err := strconv.Atoi(strings.TrimPrefix(route, sysmodel.FlowRouteGotoPrefix)); err == nil && n >= 1 && n <= len(flow) {
			return RouteDecision{NextIdx: n - 1}
		}
		return RouteDecision{NextIdx: idx + 1} // 配置被改坏时兜底顺序推进
	}
	switch route {
	case sysmodel.FlowRouteFinish:
		return RouteDecision{Finish: true, NextIdx: len(flow)}
	case sysmodel.FlowRouteReject:
		return RouteDecision{Reject: true, NextIdx: idx}
	default:
		return RouteDecision{NextIdx: idx + 1}
	}
}

// validateHumanSteps 人工环节校验（槽位须在槽位目录内（含字典衍生维度槽位）且不重复；名称 1-32 字）。
func validateHumanSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	known := make(map[string]bool, len(sysmodel.DutySlots))
	for _, ds := range AllDutySlots(db) {
		known[ds.Slot] = true
	}
	seen := map[string]bool{}
	for _, st := range steps {
		name := strings.TrimSpace(st.Name)
		if name == "" || len([]rune(name)) > 32 {
			return errs.ErrParam.WithMsg("环节名称须为 1-32 字")
		}
		if !known[st.Slot] {
			return errs.ErrParam.WithMsg("未知职责槽位「" + st.Slot + "」")
		}
		if seen[st.Slot] {
			return errs.ErrParam.WithMsg("槽位「" + st.Slot + "」在链中重复")
		}
		if st.Mode != "" && st.Mode != "any" && st.Mode != "all" {
			return errs.ErrParam.WithMsg("环节 mode 须为 any 或 all")
		}
		seen[st.Slot] = true
	}
	return nil
}

