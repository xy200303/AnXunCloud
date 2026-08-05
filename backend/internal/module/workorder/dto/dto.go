// Package dto 工单模块请求结构。
package dto

import "property-inspection/internal/pkg/response"

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

type OrderCreateReq struct {
	CommunityID string     `json:"community_id" binding:"required"`
	PointID     *string    `json:"point_id"`
	Title       string     `json:"title" binding:"required"`
	Description string     `json:"description" binding:"required"`
	Photos      []PhotoRef `json:"photos"`
	Priority    string     `json:"priority"`
	AssigneeID  *string    `json:"assignee_id"`
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
	FixRemark string     `json:"fix_remark" binding:"required"`
	FixPhotos []PhotoRef `json:"fix_photos"`
}

type ReviewReq struct {
	Result       string `json:"result" binding:"required,oneof=pass reject"`
	ReviewRemark string `json:"review_remark"`
}
