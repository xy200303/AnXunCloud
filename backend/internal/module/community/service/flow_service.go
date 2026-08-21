// 审批链解析（《汇报线与审批链扩展设计方案》§3，P2）。
// 流程 = 有序环节列表（flow_code 系统定义，steps 可配）；每个环节引用一个职责槽位，
// 名单解析/授权判定完全复用槽位体系（SlotUserIDs / SlotAuthorized），不引入第二套规则。
package service

import (
	"strings"

	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

// DefaultCheckinReviewFlow 内置默认打卡审批流程（无任何配置时兜底：单步主管审核，行为与链化前一致）。
func DefaultCheckinReviewFlow() types.FlowStepArray {
	return types.FlowStepArray{{Slot: sysmodel.SlotPatrolReportLine, Name: "主管审核"}}
}

// ResolveFlow 解析审批链配置：项目级 → 租户级 → 平台默认 → 内置默认（steps 为空视为未配置，继续回落）。
func ResolveFlow(db *gorm.DB, projectID, flowCode string) types.FlowStepArray {
	var f sysmodel.ApprovalFlow
	if err := db.Where("project_id = ? AND flow_code = ?", projectID, flowCode).First(&f).Error; err == nil && len(f.Steps) > 0 {
		return f.Steps
	}
	if tid := middleware.CommunityTenantID(db, projectID); tid != nil {
		if err := db.Where("project_id IS NULL AND tenant_id = ? AND flow_code = ?", *tid, flowCode).First(&f).Error; err == nil && len(f.Steps) > 0 {
			return f.Steps
		}
	}
	if err := db.Where("project_id IS NULL AND tenant_id IS NULL AND flow_code = ?", flowCode).First(&f).Error; err == nil && len(f.Steps) > 0 {
		return f.Steps
	}
	return DefaultCheckinReviewFlow()
}

// ResolveFlowWithSource 解析审批链并给出来源（project/tenant/platform/default），供配置页展示。
func ResolveFlowWithSource(db *gorm.DB, projectID, flowCode string) (types.FlowStepArray, string) {
	var f sysmodel.ApprovalFlow
	if err := db.Where("project_id = ? AND flow_code = ?", projectID, flowCode).First(&f).Error; err == nil && len(f.Steps) > 0 {
		return f.Steps, "project"
	}
	if tid := middleware.CommunityTenantID(db, projectID); tid != nil {
		if err := db.Where("project_id IS NULL AND tenant_id = ? AND flow_code = ?", *tid, flowCode).First(&f).Error; err == nil && len(f.Steps) > 0 {
			return f.Steps, "tenant"
		}
	}
	if err := db.Where("project_id IS NULL AND tenant_id IS NULL AND flow_code = ?", flowCode).First(&f).Error; err == nil && len(f.Steps) > 0 {
		return f.Steps, "platform"
	}
	return DefaultCheckinReviewFlow(), "default"
}

// FlowStepSlot 环节槽位解析：汇报线通用槽位按巡查类型路由到维度槽位（扩展方案 §2），其余槽位原样使用。
func FlowStepSlot(db *gorm.DB, projectID, patrolType, stepSlot string) string {
	if stepSlot == sysmodel.SlotPatrolReportLine {
		return ResolveReportLineSlot(db, projectID, patrolType)
	}
	return stepSlot
}

// ValidateFlowSteps 审批链环节入参校验（1-5 环节；槽位须在槽位目录内（含字典衍生维度槽位）且不重复；名称 1-32 字）。
func ValidateFlowSteps(db *gorm.DB, steps types.FlowStepArray) *errs.Error {
	if len(steps) == 0 || len(steps) > 5 {
		return errs.ErrParam.WithMsg("审批链须为 1-5 个环节")
	}
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
		seen[st.Slot] = true
	}
	return nil
}
