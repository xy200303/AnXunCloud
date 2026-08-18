// Package model 工单域 GORM 模型（主键为应用层 UUIDv7）。
package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// 工单状态（P2 闭环状态机，见《多租户组织架构与角色权限设计方案》§6.2）：
// reported 待分诊（项目关闭分诊时上报直接进 pending_dispatch）
// → pending_dispatch 待派单（分诊通过；分诊驳回 → closed_invalid 已作废）
// → processing 处理中（派单指定维修工或抢单，即视为接单）
// → pending_confirm 待验收（维修工完工提交，附完工照片）
// → closed 已闭环（验收通过；验收不通过退回 processing 并记录退回原因）
const (
	OrderReported        = "reported"         // 待分诊
	OrderPendingDispatch = "pending_dispatch" // 待派单
	OrderProcessing      = "processing"       // 处理中
	OrderPendingConfirm  = "pending_confirm"  // 待验收
	OrderClosed          = "closed"           // 已闭环
	OrderClosedInvalid   = "closed_invalid"   // 已作废（分诊驳回）
)

// 工单来源（字典 order_source）
const (
	SourceInspection = "inspection" // 巡检异常转单（视同已分诊，直接待派单）
	SourceActive     = "active"     // 主动上报（移动端问题上报）
	SourceFrontdesk  = "frontdesk"  // 前台代录（管理端建单）
)

// 工单流转动作（work_order_log.action，CHECK 约束取值）
const (
	ActionCreate        = "create"
	ActionTriagePass    = "triage_pass"
	ActionTriageReject  = "triage_reject"
	ActionDispatch      = "dispatch"
	ActionGrab          = "grab"
	ActionFinish        = "finish"
	ActionConfirmPass   = "confirm_pass"
	ActionConfirmReject = "confirm_reject"
)

// WorkOrder 工单
type WorkOrder struct {
	types.UUIDModel
	TenantID     *string          `gorm:"type:uuid" json:"tenant_id"` // 冗余列（=所属小区租户）
	OrderNo      string           `gorm:"size:32" json:"order_no"`
	CheckinID    *string          `gorm:"type:uuid" json:"checkin_id"`
	CommunityID  string           `gorm:"type:uuid" json:"community_id"`
	PointID      *string          `gorm:"type:uuid" json:"point_id"`
	Title        string           `gorm:"size:128" json:"title"`
	Description  string           `gorm:"type:text" json:"description"`
	Photos       types.PhotoArray `gorm:"type:jsonb" json:"photos"`
	Source       string           `gorm:"size:16" json:"source"`
	Category     string           `gorm:"size:32" json:"category"` // 工单分类（分诊时填写）
	ReporterID   string           `gorm:"type:uuid" json:"reporter_id"`
	AssigneeID   *string          `gorm:"type:uuid" json:"assignee_id"`   // 维修工（order_accept 槽位成员）
	DispatcherID *string          `gorm:"type:uuid" json:"dispatcher_id"` // 派单人（order_dispatch 槽位成员；抢单为空）
	Priority     string           `gorm:"size:16" json:"priority"`
	Status       string           `gorm:"size:16" json:"status"`
	// Items 不合格项快照（v17 起）：整改前后对比，photos 存 file_key
	Items types.OrderItemArray `gorm:"type:jsonb" json:"items"`
	// 分诊
	TriageBy   *string    `gorm:"type:uuid" json:"triage_by"`
	TriageAt   *time.Time `json:"triage_at"`
	TriageNote string     `gorm:"size:512" json:"triage_note"`
	// 派单/接单（派单即视为接单，dispatch_at 与 accept_at 同时写入）
	DispatchAt *time.Time `json:"dispatch_at"`
	AcceptAt   *time.Time `json:"accept_at"`
	// 完工
	FinishPhotos types.PhotoArray `gorm:"type:jsonb" json:"finish_photos"`
	FinishNote   string           `gorm:"size:512" json:"finish_note"`
	FinishAt     *time.Time       `json:"finish_at"`
	// 验收
	ConfirmBy   *string    `gorm:"type:uuid" json:"confirm_by"`
	ConfirmAt   *time.Time `json:"confirm_at"`
	ConfirmNote string     `gorm:"size:512" json:"confirm_note"`
	// RejectReason 最近一次驳回原因（分诊驳回 / 验收退回）
	RejectReason string         `gorm:"size:512" json:"reject_reason"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-"`
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
