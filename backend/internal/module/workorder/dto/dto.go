// Package dto 工单模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type OrderListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Status      string `form:"status"` // 支持逗号合并，如 reported,pending_dispatch
	Priority    string `form:"priority"`
	Source      string `form:"source"` // inspection/active/frontdesk
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

// OrderCreateReq 管理端建单（前台代录，source 固定 frontdesk）；
// 指定 assignee_id 时视为直接派单（进入处理中，省略受理/派单环节）。
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

// OrderReportReq 移动端主动上报（source 固定 active）。
type OrderReportReq struct {
	CommunityID string     `json:"community_id" binding:"required"`
	PointID     *string    `json:"point_id"` // 选填，须属于该小区
	Title       string     `json:"title" binding:"required"`
	Description string     `json:"description" binding:"required"`
	Photos      []PhotoRef `json:"photos"`
	Priority    string     `json:"priority"`
}

type OrderUpdateReq struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Priority    *string `json:"priority"`
}

// TriageReq 受理：pass 通过（可选定优先级/分类）→ 待派单；reject 驳回（原因必填）→ 已作废。
type TriageReq struct {
	Result   string `json:"result" binding:"required,oneof=pass reject"`
	Priority string `json:"priority"`
	Category string `json:"category"`
	Note     string `json:"note"`
}

// DispatchReq 派单：assignee 须为本项目 order_accept 槽位名单成员。
type DispatchReq struct {
	AssigneeID string `json:"assignee_id" binding:"required"`
	Remark     string `json:"remark"`
}

type FinishReq struct {
	FixRemark string     `json:"fix_remark" binding:"required"`
	FixPhotos []PhotoRef `json:"fix_photos"`
	// AfterPhotos 逐项整改后照片：检查项名 → file_key 数组，按 name 合并进工单 items 快照（可选）
	AfterPhotos map[string][]string `json:"after_photos"`
}

// ConfirmReq 验收：pass → 已闭环；reject → 退回处理中（原因必填）。
type ConfirmReq struct {
	Result      string `json:"result" binding:"required,oneof=pass reject"`
	ConfirmNote string `json:"confirm_note"`
}
