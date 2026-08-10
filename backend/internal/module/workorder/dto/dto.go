// Package dto 工单模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type OrderListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Status      string `form:"status"` // 支持逗号合并，如 assigned,processing
	Priority    string `form:"priority"`
	AssigneeID  string `form:"assignee_id"`
	ReporterID  string `form:"reporter_id"`
	OrderNo     string `form:"order_no"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}

type PhotoRef struct {
	FileKey string `json:"file_key" binding:"required"`
}

// OrderItemReq 手工建单不合格项入参（before_photos 为 file_key 数组）。
type OrderItemReq struct {
	Name         string   `json:"name" binding:"required"`
	Remark       string   `json:"remark"`
	BeforePhotos []string `json:"before_photos"`
}

type OrderCreateReq struct {
	CommunityID string         `json:"community_id" binding:"required"`
	PointID     *string        `json:"point_id"`
	Title       string         `json:"title" binding:"required"`
	Description string         `json:"description" binding:"required"`
	Photos      []PhotoRef     `json:"photos"`
	Items       []OrderItemReq `json:"items"` // 不合格项快照（可选）
	Priority    string         `json:"priority"`
	AssigneeID  *string        `json:"assignee_id"`
}

type OrderUpdateReq struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
}

type AssignReq struct {
	AssigneeID string `json:"assignee_id" binding:"required"`
	Remark     string `json:"remark"`
}

type FinishReq struct {
	FixRemark string `json:"fix_remark" binding:"required"`
	FixPhotos []PhotoRef `json:"fix_photos"`
	// AfterPhotos 逐项整改后照片：检查项名 → file_key 数组，按 name 合并进工单 items 快照（可选）
	AfterPhotos map[string][]string `json:"after_photos"`
}

type ReviewReq struct {
	Result       string `json:"result" binding:"required,oneof=pass reject"`
	ReviewRemark string `json:"review_remark"`
}
