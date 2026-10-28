package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type MCPToken struct {
	types.UUIDModel
	TenantID   string            `gorm:"type:uuid" json:"tenant_id"`
	UserID     string            `gorm:"type:uuid" json:"user_id"`
	Name       string            `gorm:"size:128" json:"name"`
	Prefix     string            `gorm:"size:24" json:"prefix"`
	SecretHash string            `gorm:"size:64;uniqueIndex" json:"-"`
	Scopes     types.StringArray `gorm:"type:jsonb" json:"scopes"`
	ExpiresAt  *time.Time        `json:"expires_at"`
	RevokedAt  *time.Time        `json:"revoked_at"`
	LastUsedAt *time.Time        `json:"last_used_at"`
	CreatedAt  time.Time         `json:"created_at"`
}

func (MCPToken) TableName() string { return "ai_mcp_token" }
