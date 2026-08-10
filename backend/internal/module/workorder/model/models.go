// Package model 工单域 GORM 模型（主键为应用层 UUIDv7）。
package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// 工单状态
const (
	OrderPending    = "pending"
	OrderAssigned   = "assigned"
	OrderProcessing = "processing"
	OrderReview     = "review"
	OrderClosed     = "closed"
	OrderRejected   = "rejected"
)

// 工单流转动作（CHECK 约束取值）
const (
	ActionCreate       = "create"
	ActionAssign       = "assign"
	ActionAccept       = "accept"
	ActionFinish       = "finish"
	ActionReviewPass   = "review_pass"
	ActionReviewReject = "review_reject"
	ActionClose        = "close"
)

// WorkOrder 异常工单
type WorkOrder struct {
	types.UUIDModel
	OrderNo      string           `gorm:"size:32" json:"order_no"`
	CheckinID    *string          `gorm:"type:uuid" json:"checkin_id"`
	CommunityID  string           `gorm:"type:uuid" json:"community_id"`
	PointID      *string          `gorm:"type:uuid" json:"point_id"`
	Title        string           `gorm:"size:128" json:"title"`
	Description  string           `gorm:"type:text" json:"description"`
	Photos       types.PhotoArray `gorm:"type:jsonb" json:"photos"`
	ReporterID   string           `gorm:"type:uuid" json:"reporter_id"`
	AssigneeID   *string          `gorm:"type:uuid" json:"assignee_id"`
	Priority     string           `gorm:"size:16" json:"priority"`
	Status       string           `gorm:"size:16" json:"status"`
	// Items 不合格项快照（v17 起）：整改前后对比，photos 存 file_key
	Items        types.OrderItemArray `gorm:"type:jsonb" json:"items"`
	FixPhotos    types.PhotoArray `gorm:"type:jsonb" json:"fix_photos"`
	FixRemark    string           `gorm:"size:512" json:"fix_remark"`
	FinishedAt   *time.Time       `json:"finished_at"`
	ReviewedBy   *string          `gorm:"type:uuid" json:"reviewed_by"`
	ReviewRemark string           `gorm:"size:512" json:"review_remark"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	DeletedAt    gorm.DeletedAt   `json:"-"`
}

func (WorkOrder) TableName() string { return "work_order" }

// WorkOrderLog 工单流转留痕
type WorkOrderLog struct {
	types.UUIDModel
	OrderID    string    `gorm:"type:uuid" json:"order_id"`
	Action     string    `gorm:"size:32" json:"action"`
	OperatorID string    `gorm:"type:uuid" json:"operator_id"`
	Detail     string    `gorm:"size:512" json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

func (WorkOrderLog) TableName() string { return "work_order_log" }
