package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type ReportRenderTask struct {
	types.UUIDModel
	TenantID     string    `gorm:"type:uuid" json:"tenant_id"`
	ReportID     string    `gorm:"type:uuid" json:"report_id"`
	Stage        string    `gorm:"size:24" json:"stage"`
	Status       string    `gorm:"size:16" json:"status"`
	ErrorMessage string    `gorm:"size:512" json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (ReportRenderTask) TableName() string { return "report_render_task" }
