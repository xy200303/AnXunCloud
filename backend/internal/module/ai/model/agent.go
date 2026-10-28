package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type AgentTool struct {
	types.UUIDModel
	Scope     string    `gorm:"size:16" json:"scope"`
	ScopeID   string    `gorm:"type:uuid" json:"scope_id"`
	Code      string    `gorm:"size:64" json:"code"`
	Status    string    `gorm:"size:16" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AgentTool) TableName() string { return "ai_agent_tool" }

type AgentCall struct {
	types.UUIDModel
	TenantID       string        `gorm:"type:uuid" json:"tenant_id"`
	UserID         string        `gorm:"type:uuid" json:"user_id"`
	ConversationID string        `gorm:"type:uuid" json:"conversation_id"`
	ToolCode       string        `gorm:"size:64" json:"tool_code"`
	Arguments      types.JSONMap `gorm:"type:jsonb" json:"arguments"`
	Result         types.JSONMap `gorm:"type:jsonb" json:"result"`
	Status         string        `gorm:"size:16" json:"status"`
	ErrorMessage   string        `gorm:"size:512" json:"error_message"`
	CreatedAt      time.Time     `json:"created_at"`
}

func (AgentCall) TableName() string { return "ai_agent_call" }
