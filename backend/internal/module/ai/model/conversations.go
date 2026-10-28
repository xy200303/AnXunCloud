package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type Conversation struct {
	types.UUIDModel
	TenantID  string    `gorm:"type:uuid" json:"tenant_id"`
	UserID    string    `gorm:"type:uuid" json:"user_id"`
	Title     string    `gorm:"size:128" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Conversation) TableName() string { return "ai_conversation" }

type Message struct {
	types.UUIDModel
	ConversationID   string        `gorm:"type:uuid" json:"conversation_id"`
	Role             string        `gorm:"size:16" json:"role"`
	Content          string        `gorm:"type:text" json:"content"`
	ProviderCode     string        `gorm:"size:64" json:"provider_code"`
	ModelID          string        `gorm:"size:128" json:"model_id"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	Status           string        `gorm:"size:16" json:"status"`
	ErrorMessage     string        `gorm:"size:512" json:"error_message"`
	ToolCalls        types.JSONMap `gorm:"type:jsonb" json:"tool_calls"`
	ToolCallID       string        `gorm:"size:64" json:"tool_call_id"`
	ToolName         string        `gorm:"size:64" json:"tool_name"`
	CreatedAt        time.Time     `json:"created_at"`
}

func (Message) TableName() string { return "ai_message" }
