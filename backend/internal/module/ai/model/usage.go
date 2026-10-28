package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type Quota struct {
	types.UUIDModel
	TenantID         string    `gorm:"type:uuid" json:"tenant_id"`
	TaskType         string    `gorm:"size:48" json:"task_type"`
	MaxRequestsDaily int       `json:"max_requests_daily"`
	MaxTokensDaily   int64     `json:"max_tokens_daily"`
	MaxConcurrent    int       `json:"max_concurrent"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (Quota) TableName() string { return "ai_quota" }

type UsageReservation struct {
	types.UUIDModel
	TenantID         string     `gorm:"type:uuid" json:"tenant_id"`
	TaskType         string     `gorm:"size:48" json:"task_type"`
	Status           string     `gorm:"size:16" json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
}

func (UsageReservation) TableName() string { return "ai_usage_reservation" }
