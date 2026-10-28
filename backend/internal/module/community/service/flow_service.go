// 审批链按职责命名并直接绑定角色；候选人为角色在对象范围内的有效任职人员。
package service

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/configread"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/types"
)

func DefaultCheckinReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{{RoleCode: sysmodel.QualitySupervisorCode, Name: "打卡复核", Mode: "any"}}
}

func DefaultWorkorderReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{{RoleCode: sysmodel.ProjectManagerCode, Name: "工单审核", Mode: "any"}}
}

func DefaultAcceptanceReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{{RoleCode: sysmodel.ProjectManagerCode, Name: "查验报告签署", Mode: "any"}}
}

func DefaultFlow(db *gorm.DB, code string) types.FlowStepArray {
	switch code {
	case sysmodel.FlowReportReview:
		return DefaultReportReviewFlow()
	case sysmodel.FlowWorkorderReview:
		return DefaultWorkorderReviewFlow()
	case sysmodel.FlowAcceptanceReview:
		return DefaultAcceptanceReviewFlow()
	case sysmodel.FlowMaintReview:
		return DefaultMaintReviewFlow(db)
	default:
		return DefaultCheckinReviewFlow()
	}
}

func DefaultReportReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{
		{RoleCode: sysmodel.InspectorCode, Name: "报告现场确认", Mode: "all"},
		{RoleCode: sysmodel.QualitySupervisorCode, Name: "月报主管审核", Mode: "any"},
		{RoleCode: sysmodel.ProjectManagerCode, Name: "月报审批", Mode: "any"},
	}
}

// ResolveFlow 按统一作用域解析审批链：项目级 → 租户级 → 平台默认 → 内置默认。
func ResolveFlow(db *gorm.DB, communityID, flowCode string) types.FlowStepArray {
	var f sysmodel.ApprovalFlow
	if err := db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopeProject, communityID, flowCode).First(&f).Error; err == nil {
		return f.Steps
	}
	if tid := middleware.CommunityTenantID(db, communityID); tid != nil {
		if err := db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopeTenant, *tid, flowCode).First(&f).Error; err == nil {
			return f.Steps
		}
	}
	if err := db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopePlatform, sysmodel.PlatformScopeID, flowCode).First(&f).Error; err == nil {
		return f.Steps
	}
	return DefaultFlow(db, flowCode)
}

// ResolveFlowWithSource 解析审批链并给出来源（project/tenant/platform/default），供配置页展示。
func ResolveFlowWithSource(db *gorm.DB, communityID, flowCode string) (types.FlowStepArray, string) {
	var f sysmodel.ApprovalFlow
	if err := db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopeProject, communityID, flowCode).First(&f).Error; err == nil {
		return f.Steps, "project"
	}
	if tid := middleware.CommunityTenantID(db, communityID); tid != nil {
		if err := db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopeTenant, *tid, flowCode).First(&f).Error; err == nil {
			return f.Steps, "tenant"
		}
	}
	if err := db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopePlatform, sysmodel.PlatformScopeID, flowCode).First(&f).Error; err == nil {
		return f.Steps, "platform"
	}
	return DefaultFlow(db, flowCode), "default"
}

func DefaultMaintReviewFlow(db *gorm.DB) types.FlowStepArray {
	if configread.GetBool(db, "equipment.ai_auto_confirm", false) {
		return types.FlowStepArray{
			{Kind: sysmodel.FlowStepKindAI, Name: "AI 辅助识别", OnReview: sysmodel.FlowRouteNext},
			{RoleCode: sysmodel.EngineerSupervisorCode, Name: "维保审核", Mode: "any"},
		}
	}
	return types.FlowStepArray{{RoleCode: sysmodel.EngineerSupervisorCode, Name: "维保审核", Mode: "any"}}
}

type WalkResult struct {
	Finish       bool
	Reject       bool
	Blocked      bool
	Step         int
	NeedGate     bool
	NotifyIdx    int
	Fallback     bool
	BlockedSteps []string
}

func WalkFlow(flow types.FlowStepArray, startIdx int, outcome string, forcedHuman bool) WalkResult {
	if len(flow) == 0 {
		return WalkResult{Blocked: true, Step: 0, NotifyIdx: -1, BlockedSteps: []string{"未配置审批流程"}}
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
			return WalkResult{Blocked: true, Step: idx, NotifyIdx: -1, BlockedSteps: []string{"AI 环节后缺少人工审核"}}
		default:
			idx = d.NextIdx
		}
	}
	return WalkResult{Finish: true, Step: len(flow), NotifyIdx: -1}
}

func WalkFlowWithVoters(db *gorm.DB, communityID string, flow types.FlowStepArray, startIdx int, outcome string, forcedHuman bool) WalkResult {
	w := WalkFlow(flow, startIdx, outcome, forcedHuman)
	if w.Blocked || w.Finish || w.Reject || w.NeedGate || w.Step >= len(flow) {
		return w
	}
	step := flow[w.Step]
	if step.Kind == sysmodel.FlowStepKindAI {
		return w
	}
	if len(RoleCandidateUserIDsForCommunity(db, communityID, step.RoleCode)) > 0 {
		return w
	}
	w.Blocked = true
	w.BlockedSteps = []string{step.Name}
	return w
}

func NotifyBlockedFlowSteps(db *gorm.DB, n *notify.Notifier, tenantID *string, flowLabel string, blocked []string, bizID *string) {
	if len(blocked) == 0 || n == nil || tenantID == nil || *tenantID == "" {
		return
	}
	ids := adminUserIDs(db, *tenantID)
	if len(ids) == 0 {
		return
	}
	_ = n.SendBatch(ids, tenantID, "flow_blocked", "审批流程已挂起",
		fmt.Sprintf("「%s」在环节【%s】缺少候选人，记录保持待处理，未自动通过。请补充任职或调整审批节点。",
			flowLabel, strings.Join(blocked, "、")), bizID)
}

func adminUserIDs(db *gorm.DB, tenantID string) []string {
	var ids []string
	db.Model(&sysmodel.SysUser{}).Distinct("sys_user.id").
		Joins("JOIN sys_role ON sys_role.id = sys_user.role_id").
		Where("sys_user.tenant_id = ? AND sys_user.status = ? AND sys_role.status = ?", tenantID, sysmodel.StatusEnabled, sysmodel.StatusEnabled).
		Where("sys_role.code IN ?", []string{sysmodel.ProjectManagerCode, sysmodel.TenantAdminCode}).
		Pluck("sys_user.id", &ids)
	return ids
}

// FlowOrResolve 记录审核时取链：快照优先（在途记录按提交时的规则审完，改流程只影响新单）。
// 快照为空（存量 NULL/空流程——FlowStepArray.Scan 把 NULL 读成空数组）才回落现配：
// 只有带环节的链才有"停在第 N 环节"的冻结语义，空流程（默认通过+兜底）跟随最新配置即可。
func FlowOrResolve(db *gorm.DB, snapshot types.FlowStepArray, communityID, flowCode string) types.FlowStepArray {
	if len(snapshot) > 0 {
		return snapshot
	}
	return ResolveFlow(db, communityID, flowCode)
}

func ValidateHumanFlowSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	if len(steps) == 0 || len(steps) > 5 {
		return errs.ErrParam.WithMsg("审批链须包含 1-5 个人工环节")
	}
	for _, st := range steps {
		if st.Kind == sysmodel.FlowStepKindAI {
			return errs.ErrParam.WithMsg("此审核链只支持人工环节")
		}
	}
	return validateHumanSteps(db, steps)
}

func ValidateReportFlowSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	return ValidateHumanFlowSteps(db, steps)
}

// ValidateFlowSteps 打卡/维保审核链入参校验（1-5 个环节，不允许空链直通；
// AI 环节只生成预审建议，人工节点候选人决定能否审核）。
func ValidateFlowSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	if len(steps) == 0 || len(steps) > 5 {
		return errs.ErrParam.WithMsg("审批链须包含 1-5 个环节")
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

// validateAIRoute AI 分支路由值校验：”（默认）/finish/next/reject/goto:N（N 须指向后面的环节，防环）。
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

func validateHumanSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	for _, st := range steps {
		name := strings.TrimSpace(st.Name)
		if name == "" || len([]rune(name)) > 32 {
			return errs.ErrParam.WithMsg("环节名称须为 1-32 字")
		}
		if strings.TrimSpace(st.RoleCode) == "" {
			return errs.ErrParam.WithMsg("人工环节必须绑定角色")
		}
		var count int64
		if err := db.Model(&sysmodel.SysRole{}).Where("code = ? AND status = ?", st.RoleCode, sysmodel.StatusEnabled).Count(&count).Error; err != nil {
			return errs.ErrInternal
		}
		if count == 0 {
			return errs.ErrParam.WithMsg("角色「" + st.RoleCode + "」不存在或已停用")
		}
		if st.Mode != "" && st.Mode != "any" && st.Mode != "all" {
			return errs.ErrParam.WithMsg("环节 mode 须为 any 或 all")
		}
	}
	return nil
}
