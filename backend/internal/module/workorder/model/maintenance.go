package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type WorkOrderMaintenanceItem struct {
	types.UUIDModel
	TenantID          string    `gorm:"type:uuid;uniqueIndex:uk_workorder_maintenance_item,priority:1" json:"tenant_id"`
	OrderID           string    `gorm:"type:uuid;uniqueIndex:uk_workorder_maintenance_item,priority:2" json:"order_id"`
	IssueID           string    `gorm:"type:uuid;unique" json:"issue_id"`
	EquipmentID       string    `gorm:"type:uuid;uniqueIndex:uk_workorder_maintenance_item,priority:3" json:"equipment_id"`
	MaintenanceAction string    `gorm:"size:16;uniqueIndex:uk_workorder_maintenance_item,priority:4" json:"maintenance_action"`
	DuePeriod         string    `gorm:"size:7" json:"due_period"`
	MaintenanceID     *string   `gorm:"type:uuid" json:"maintenance_id"`
	CreatedAt         time.Time `json:"created_at"`
}

func (WorkOrderMaintenanceItem) TableName() string { return "work_order_maintenance_item" }
