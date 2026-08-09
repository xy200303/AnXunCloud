// Package dto 小区与楼栋模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type CommunityListQuery struct {
	response.PageQuery
	Name   string `form:"name"`
	Status string `form:"status"`
}

type CommunitySaveReq struct {
	Name      string `json:"name" binding:"required"`
	Address   string `json:"address"`
	ManagerID *string `json:"manager_id"`
	Status    *int   `json:"status"`
	Remark    string `json:"remark"`
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
