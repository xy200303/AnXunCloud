package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
	aipkg "anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/strutil"
)

type Actor struct {
	TenantID string
	UserID   string
}

type ChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content" binding:"required"`
}

type AssistantService struct {
	db       *gorm.DB
	registry *RegistryService
	usage    *UsageService
}

func NewAssistantService(db *gorm.DB, registry *RegistryService, usage *UsageService) *AssistantService {
	return &AssistantService{db: db, registry: registry, usage: usage}
}

func (s *AssistantService) Status(actor Actor) (gin.H, *errs.Error) {
	_, be := s.registry.ResolveTask(Scope{Name: model.ScopeTenant, ID: actor.TenantID}, "assistant")
	if be != nil {
		return gin.H{"available": false, "reason": be.Msg}, nil
	}
	return gin.H{"available": true, "reason": ""}, nil
}

func (s *AssistantService) Conversations(actor Actor) ([]model.Conversation, *errs.Error) {
	var conversations []model.Conversation
	if err := s.db.Where("tenant_id = ? AND user_id = ?", actor.TenantID, actor.UserID).Order("updated_at DESC").Limit(50).Find(&conversations).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return conversations, nil
}

func (s *AssistantService) Messages(actor Actor, conversationID string) ([]model.Message, *errs.Error) {
	var conversation model.Conversation
	if err := s.db.First(&conversation, "id = ? AND tenant_id = ? AND user_id = ?", conversationID, actor.TenantID, actor.UserID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var messages []model.Message
	if err := s.db.Where("conversation_id = ?", conversation.ID).Order("created_at, id").Find(&messages).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return messages, nil
}

func (s *AssistantService) Chat(ctx context.Context, actor Actor, req ChatRequest) (gin.H, *errs.Error) {
	content := strings.TrimSpace(req.Content)
	if content == "" || len([]rune(content)) > 4000 {
		return nil, errs.ErrParam.WithMsg("消息须为 1-4000 字")
	}
	resolved, be := s.registry.ResolveTask(Scope{Name: model.ScopeTenant, ID: actor.TenantID}, "assistant")
	if be != nil {
		return nil, be
	}
	if s.usage == nil {
		return nil, errs.ErrInternal.WithMsg("AI 配额服务未配置")
	}
	reservation, be := s.usage.Reserve(actor, "assistant")
	if be != nil {
		return nil, be
	}
	failed := true
	promptTokens, completionTokens := 0, 0
	defer func() { s.usage.Finish(reservation, promptTokens, completionTokens, failed) }()
	var conversation model.Conversation
	if req.ConversationID != "" {
		if err := s.db.First(&conversation, "id = ? AND tenant_id = ? AND user_id = ?", req.ConversationID, actor.TenantID, actor.UserID).Error; err != nil {
			return nil, errs.ErrNotFound
		}
	} else {
		conversation = model.Conversation{TenantID: actor.TenantID, UserID: actor.UserID, Title: strutil.Truncate(content, 40)}
		if err := s.db.Create(&conversation).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	userMessage := model.Message{ConversationID: conversation.ID, Role: "user", Content: content, Status: "ready"}
	if err := s.db.Create(&userMessage).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var history []model.Message
	s.db.Where("conversation_id = ? AND status = ?", conversation.ID, "ready").Order("created_at DESC, id DESC").Limit(30).Find(&history)
	messages := make([]aipkg.TextMessage, 0, len(history)+1)
	messages = append(messages, aipkg.TextMessage{Role: "system", Content: "你是 AI云管家的物业巡检管理助手。只依据用户提供的信息回答，不编造业务数据；需要查询或修改业务数据时必须由有权限的工具执行，当前没有工具时明确说明。"})
	for i := len(history) - 1; i >= 0; i-- {
		messages = append(messages, aipkg.TextMessage{Role: history[i].Role, Content: history[i].Content})
	}
	result, err := aipkg.ChatText(ctx, aipkg.TextChatParams{
		Protocol: resolved.Provider.Protocol, BaseURL: resolved.Provider.BaseURL, APIKey: resolved.APIKey, Model: resolved.Model.ModelID,
		TimeoutSeconds: resolved.Binding.TimeoutSeconds, MaxOutput: resolved.Model.MaxOutput, Messages: messages,
	})
	promptTokens, completionTokens = result.PromptTokens, result.CompletionTokens
	failed = err != nil
	assistant := model.Message{ConversationID: conversation.ID, Role: "assistant", ProviderCode: resolved.Provider.Code, ModelID: resolved.Model.ModelID, Status: "ready"}
	if err != nil {
		assistant.Status = "failed"
		assistant.ErrorMessage = strutil.Truncate(err.Error(), 512)
	} else {
		assistant.Content = result.Text
		assistant.PromptTokens = result.PromptTokens
		assistant.CompletionTokens = result.CompletionTokens
	}
	if err := s.db.Create(&assistant).Error; err != nil {
		return nil, errs.ErrInternal
	}
	s.db.Model(&conversation).Update("updated_at", assistant.CreatedAt)
	return gin.H{"conversation_id": conversation.ID, "message": assistant}, nil
}
