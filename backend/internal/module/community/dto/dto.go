// Package dto 小区与楼栋模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type CommunityListQuery struct {
	response.PageQuery
	Name   string `form:"name"`
	Status string `form:"status"`
	// TenantID 租户过滤（仅超管生效；未指定时超级管理员查看全部租户，非超管强制按自身租户过滤）
	TenantID string `form:"tenant_id"`
}

type CommunitySaveReq struct {
	Name      string  `json:"name" binding:"required"`
	Address   string  `json:"address"`
	ManagerID *string `json:"manager_id"`
	Status    *int    `json:"status"`
	Remark    string  `json:"remark"`
}

// StaffSaveReq 任职：角色取用户唯一角色，责任楼栋只用于分派筛选。
type StaffSaveReq struct {
	UserID         string   `json:"user_id" binding:"required"`
	BuildingIDs    []string `json:"building_ids"`
	EffectiveFrom  *string  `json:"effective_from"`
	EffectiveTo    *string  `json:"effective_to"`
	DelegateUserID *string  `json:"delegate_user_id"`
	DelegateUntil  *string  `json:"delegate_until"`
	Status         *int     `json:"status"`
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
	ID         string                  `json:"id"`
	Name       string                  `json:"name"`
	TenantID   string                  `json:"tenant_id"`
	TenantName string                  `json:"tenant_name"`
	Buildings  []CommunityTreeBuilding `json:"buildings"`
}
