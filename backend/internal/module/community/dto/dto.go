// Package dto 小区与楼栋模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type CommunityListQuery struct {
	response.PageQuery
	Name   string `form:"name"`
	Status string `form:"status"`
	// TenantID 租户过滤（仅超管生效，由 EffectiveTenantID 解析为租户上下文；非超管强制按自身租户过滤）
	TenantID string `form:"tenant_id"`
}

type CommunitySaveReq struct {
	Name      string  `json:"name" binding:"required"`
	Address   string  `json:"address"`
	ManagerID *string `json:"manager_id"`
	// WoTriageEnabled 工单受理开关（nil 保持不变；新增缺省开）
	WoTriageEnabled *bool `json:"wo_triage_enabled"`
	// WoGrabEnabled 抢单模式开关（nil 保持不变；新增缺省关）
	WoGrabEnabled *bool `json:"wo_grab_enabled"`
	Status    *int    `json:"status"`
	Remark    string  `json:"remark"`
}

// StaffSaveReq 项目编制成员新增/修改（posts 引用 post_dict.code；building_ids 仅楼管员用）。
type StaffSaveReq struct {
	UserID      string   `json:"user_id" binding:"required"`
	Posts       []string `json:"posts" binding:"required,min=1"`
	BuildingIDs []string `json:"building_ids"`
	Status      *int     `json:"status"`
}

// DutyBindingItem 单条槽位绑定（post_codes 空数组 = 该环节跳过/降级）。
type DutyBindingItem struct {
	Slot      string   `json:"slot" binding:"required"`
	PostCodes []string `json:"post_codes"`
}

// DutyBindingsSaveReq 项目级职责槽位绑定整体保存（覆盖项目级配置）。
type DutyBindingsSaveReq struct {
	Bindings []DutyBindingItem `json:"bindings" binding:"required"`
}

type BuildingListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id" binding:"required"`
	Name        string `form:"name"`
	Type        string `form:"type"`
}

type BuildingSaveReq struct {
	CommunityID string `json:"community_id"`
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type" binding:"required,oneof=building area"`
	Sort        int    `json:"sort"`
}

// CommunityTreeBuilding 树节点中的楼栋项。
type CommunityTreeBuilding struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// CommunityTreeNode 小区/楼栋树（点位管理等左树一次加载）。
type CommunityTreeNode struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Buildings []CommunityTreeBuilding `json:"buildings"`
}
