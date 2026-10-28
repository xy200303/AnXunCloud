package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

const (
	ScopePlatform   = "platform"
	ScopeTenant     = "tenant"
	Enabled         = "enabled"
	Disabled        = "disabled"
	PlatformScopeID = "00000000-0000-0000-0000-000000000000"
)

type Provider struct {
	types.UUIDModel
	Scope         string    `gorm:"size:16" json:"scope"`
	ScopeID       string    `gorm:"type:uuid" json:"scope_id"`
	Code          string    `gorm:"size:64" json:"code"`
	Name          string    `gorm:"size:128" json:"name"`
	Protocol      string    `gorm:"size:32" json:"protocol"`
	BaseURL       string    `gorm:"size:512" json:"base_url"`
	CredentialRef string    `gorm:"size:128" json:"credential_ref"`
	Status        string    `gorm:"size:16" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Provider) TableName() string { return "ai_provider" }

type Model struct {
	types.UUIDModel
	Scope         string            `gorm:"size:16" json:"scope"`
	ScopeID       string            `gorm:"type:uuid" json:"scope_id"`
	ProviderCode  string            `gorm:"size:64" json:"provider_code"`
	ModelID       string            `gorm:"size:128" json:"model_id"`
	DisplayName   string            `gorm:"size:128" json:"display_name"`
	ContextWindow int               `json:"context_window"`
	MaxOutput     int               `json:"max_output"`
	Capabilities  types.StringArray `gorm:"type:jsonb" json:"capabilities"`
	Status        string            `gorm:"size:16" json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func (Model) TableName() string { return "ai_model" }

type TaskBinding struct {
	types.UUIDModel
	Scope          string    `gorm:"size:16" json:"scope"`
	ScopeID        string    `gorm:"type:uuid" json:"scope_id"`
	TaskType       string    `gorm:"size:48" json:"task_type"`
	ProviderCode   string    `gorm:"size:64" json:"provider_code"`
	ModelID        string    `gorm:"size:128" json:"model_id"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	MaxRetries     int       `json:"max_retries"`
	CustomPrompt   string    `json:"custom_prompt"`
	Status         string    `gorm:"size:16" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (TaskBinding) TableName() string { return "ai_task_binding" }
