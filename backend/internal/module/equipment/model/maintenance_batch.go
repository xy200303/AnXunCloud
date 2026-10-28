package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

const (
	BatchPending   = "pending"
	BatchRejected  = "rejected"
	BatchConfirmed = "confirmed"
)

type MaintenanceSelection struct {
	Token        string            `gorm:"column:token;type:uuid;primaryKey" json:"token"`
	TenantID     string            `gorm:"type:uuid" json:"tenant_id"`
	ActorID      string            `gorm:"type:uuid" json:"actor_id"`
	Action       string            `gorm:"size:32" json:"action"`
	OperationID  string            `gorm:"size:64" json:"operation_id"`
	PayloadHash  string            `gorm:"size:64" json:"payload_hash"`
	EquipmentIDs types.StringArray `gorm:"type:jsonb" json:"equipment_ids"`
	ExcludedIDs  types.StringArray `gorm:"type:jsonb" json:"excluded_ids"`
	Filter       types.JSONMap     `gorm:"type:jsonb" json:"filter"`
	ExpiresAt    time.Time         `json:"expires_at"`
	CreatedAt    time.Time         `json:"created_at"`
}

func (MaintenanceSelection) TableName() string { return "maintenance_selection" }

type MaintenanceBatch struct {
	types.UUIDModel
	TenantID        string              `gorm:"type:uuid" json:"tenant_id"`
	CommunityID     string              `gorm:"type:uuid" json:"community_id"`
	MaintenanceType string              `gorm:"size:16" json:"maintenance_type"`
	MaintenanceDate time.Time           `gorm:"type:date" json:"maintenance_date"`
	Status          string              `gorm:"size:16" json:"status"`
	Revision        int                 `json:"revision"`
	ReviewFlow      types.FlowStepArray `gorm:"type:jsonb" json:"review_flow"`
	ConfirmStep     int                 `json:"confirm_step"`
	OperationID     string              `gorm:"size:64" json:"operation_id"`
	PayloadHash     string              `gorm:"size:64" json:"payload_hash"`
	OperatorName    string              `gorm:"size:64" json:"operator_name"`
	Vendor          *string             `gorm:"size:128" json:"vendor"`
	Note            string              `gorm:"type:text" json:"note"`
	FileIDs         types.StringArray   `gorm:"type:jsonb" json:"file_ids"`
	CreatedBy       string              `gorm:"type:uuid" json:"created_by"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	ConfirmedAt     *time.Time          `json:"confirmed_at"`
	RejectReason    *string             `gorm:"size:512" json:"reject_reason"`
}

func (MaintenanceBatch) TableName() string { return "maintenance_batch" }

type MaintenanceBatchItem struct {
	types.UUIDModel
	TenantID    string            `gorm:"type:uuid" json:"tenant_id"`
	BatchID     string            `gorm:"type:uuid" json:"batch_id"`
	EquipmentID string            `gorm:"type:uuid" json:"equipment_id"`
	Status      string            `gorm:"size:16" json:"status"`
	Note        string            `gorm:"type:text" json:"note"`
	FileIDs     types.StringArray `gorm:"type:jsonb" json:"file_ids"`
	CreatedAt   time.Time         `json:"created_at"`
}

func (MaintenanceBatchItem) TableName() string { return "maintenance_batch_item" }
