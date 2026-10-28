package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

const (
	StatusPending       = "pending"
	StatusInProgress    = "in_progress"
	StatusPendingReview = "pending_review"
	StatusDone          = "done"
	StatusClosed        = "closed"

	WorkTypeInspectionFix = "inspection_fix"
	WorkTypeMaintenance   = "maintenance"
)

type WorkOrder struct {
	types.UUIDModel
	TenantID    string              `gorm:"type:uuid" json:"tenant_id"`
	CommunityID string              `gorm:"type:uuid" json:"community_id"`
	OrderNo     string              `gorm:"size:32;uniqueIndex" json:"order_no"`
	WorkType    string              `gorm:"size:32" json:"work_type"`
	Title       string              `gorm:"size:255" json:"title"`
	Status      string              `gorm:"size:24" json:"status"`
	Priority    string              `gorm:"size:16" json:"priority"`
	AssigneeID  *string             `gorm:"type:uuid" json:"assignee_id"`
	Deadline    *time.Time          `json:"deadline"`
	ReviewFlow  types.FlowStepArray `gorm:"type:jsonb" json:"review_flow"`
	ReviewStep  int                 `json:"review_step"`
	Revision    int                 `gorm:"default:1" json:"revision"`
	CloseReason string              `gorm:"size:512" json:"close_reason"`
	ClosedAt    *time.Time          `json:"closed_at"`
	CompletedAt *time.Time          `json:"completed_at"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	DeletedAt   gorm.DeletedAt      `json:"-"`
}

func (WorkOrder) TableName() string { return "work_order" }

type WorkOrderIssue struct {
	types.UUIDModel
	TenantID       string            `gorm:"type:uuid;uniqueIndex:uk_work_order_issue_source,priority:1" json:"tenant_id"`
	CommunityID    string            `gorm:"type:uuid" json:"community_id"`
	OrderID        string            `gorm:"type:uuid" json:"order_id"`
	SourceType     string            `gorm:"size:48;uniqueIndex:uk_work_order_issue_source,priority:2" json:"source_type"`
	SourceRecordID string            `gorm:"type:uuid" json:"source_record_id"`
	SourceItemID   string            `gorm:"type:uuid;uniqueIndex:uk_work_order_issue_source,priority:3" json:"source_item_id"`
	SourceRevision int               `json:"source_revision"`
	Title          string            `gorm:"size:255" json:"title"`
	Problem        string            `gorm:"type:text" json:"problem"`
	Location       string            `gorm:"size:255" json:"location"`
	EquipmentID    *string           `gorm:"type:uuid" json:"equipment_id"`
	Photos         types.StringArray `gorm:"type:jsonb" json:"photos"`
	Status         string            `gorm:"size:24" json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func (WorkOrderIssue) TableName() string { return "work_order_issue" }

type WorkOrderFixAttempt struct {
	types.UUIDModel
	OrderID     string            `gorm:"type:uuid" json:"order_id"`
	IssueID     string            `gorm:"type:uuid" json:"issue_id"`
	Round       int               `json:"round"`
	OperatorID  string            `gorm:"type:uuid" json:"operator_id"`
	Note        string            `gorm:"type:text" json:"note"`
	Photos      types.StringArray `gorm:"type:jsonb" json:"photos"`
	SubmittedAt time.Time         `json:"submitted_at"`
}

func (WorkOrderFixAttempt) TableName() string { return "work_order_fix_attempt" }

type WorkOrderEvent struct {
	types.UUIDModel
	TenantID    string    `gorm:"type:uuid" json:"tenant_id"`
	OrderID     string    `gorm:"type:uuid" json:"order_id"`
	IssueID     *string   `gorm:"type:uuid" json:"issue_id"`
	EventType   string    `gorm:"size:32" json:"event_type"`
	OperationID string    `gorm:"size:64" json:"operation_id"`
	ActorID     string    `gorm:"type:uuid" json:"actor_id"`
	FromStatus  string    `gorm:"size:24" json:"from_status"`
	ToStatus    string    `gorm:"size:24" json:"to_status"`
	Revision    int       `json:"revision"`
	Reason      string    `gorm:"size:512" json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

func (WorkOrderEvent) TableName() string { return "work_order_event" }
