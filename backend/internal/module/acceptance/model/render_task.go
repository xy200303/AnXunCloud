package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type RenderTask struct {
	types.UUIDModel
	TenantID     string    `gorm:"type:uuid" json:"tenant_id"`
	BatchID      string    `gorm:"type:uuid" json:"batch_id"`
	Stage        string    `gorm:"size:24" json:"stage"`
	Status       string    `gorm:"size:16" json:"status"`
	ErrorMessage string    `gorm:"size:512" json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (RenderTask) TableName() string { return "acceptance_render_task" }
