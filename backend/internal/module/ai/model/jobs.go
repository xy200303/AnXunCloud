package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type AIItemJob struct {
	JobID        string            `gorm:"column:id;type:uuid;primaryKey" json:"job_id"`
	TenantID     string            `gorm:"type:uuid" json:"tenant_id"`
	UserID       string            `gorm:"type:uuid" json:"user_id"`
	Status       string            `gorm:"size:16" json:"status"`
	Verdict      string            `gorm:"size:16" json:"verdict"`
	Reason       string            `gorm:"type:text" json:"reason"`
	Reading      string            `gorm:"size:64" json:"reading"`
	AbnormalTags types.StringArray `gorm:"type:jsonb" json:"abnormal_tags"`
	QualityPass  *bool             `json:"quality_pass"`
	QualityIssue string            `gorm:"size:255" json:"quality_issue"`
	Payload      types.JSONMap     `gorm:"type:jsonb" json:"payload"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func (AIItemJob) TableName() string { return "ai_item_job" }
