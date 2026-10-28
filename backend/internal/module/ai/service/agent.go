package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
	aipkg "anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/types"
)

type AgentService struct {
	db       *gorm.DB
	registry *RegistryService
	usage    *UsageService
	mcp      *MCPService
}

func NewAgentService(db *gorm.DB, registry *RegistryService, usage *UsageService, mcp *MCPService) *AgentService {
	return &AgentService{db: db, registry: registry, usage: usage, mcp: mcp}
}

func (s *AgentService) Status(actor Actor) (gin.H, *errs.Error) {
	_, be := s.registry.ResolveTask(Scope{Name: model.ScopeTenant, ID: actor.TenantID}, "agent_review")
	if be != nil {
		return gin.H{"available": false, "reason": be.Msg}, nil
	}
	return gin.H{"available": true, "reason": ""}, nil
}

func (s *AgentService) Tools(actor Actor) ([]gin.H, *errs.Error) {
	var rows []model.AgentTool
	if err := s.db.Where("(scope = ? OR (scope = ? AND scope_id = ?)) AND code = ?", model.ScopePlatform, model.ScopeTenant, actor.TenantID, "inspection.summary").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	enabled := true
	for _, row := range rows {
		if row.Scope == model.ScopeTenant {
			enabled = row.Status == model.Enabled
		}
	}
	return []gin.H{{
		"name": "inspection.summary", "title": "巡检任务汇总", "read_only": true, "enabled": enabled,
		"description":  "按日期范围统计任务、完成、逾期和异常数，继承当前用户数据权限",
		"input_schema": gin.H{"type": "object", "properties": gin.H{"start_date": gin.H{"type": "string", "format": "date"}, "end_date": gin.H{"type": "string", "format": "date"}, "community_id": gin.H{"type": "string", "format": "uuid"}}},
	}}, nil
}

func (s *AgentService) SetTool(actor Actor, code string, enabled bool) *errs.Error {
	if code != "inspection.summary" {
		return errs.ErrParam.WithMsg("只能配置已注册的只读工具")
	}
	row := model.AgentTool{Scope: model.ScopeTenant, ScopeID: actor.TenantID, Code: code}
	if enabled {
		row.Status = model.Enabled
	} else {
		row.Status = model.Disabled
	}
	var count int64
	if err := s.db.Model(&model.AgentTool{}).Where("scope = ? AND scope_id = ? AND code = ?", row.Scope, row.ScopeID, row.Code).Count(&count).Error; err != nil {
		return errs.ErrInternal
	}
	if count > 0 {
		if err := s.db.Model(&model.AgentTool{}).Where("scope = ? AND scope_id = ? AND code = ?", row.Scope, row.ScopeID, row.Code).Update("status", row.Status).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := s.db.Create(&row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *AgentService) toolsEnabled(actor Actor) bool {
	tools, be := s.Tools(actor)
	return be == nil && len(tools) == 1 && tools[0]["enabled"] == true
}

func (s *AgentService) Chat(ctx context.Context, actor Actor, req ChatRequest) (gin.H, *errs.Error) {
	content := strings.TrimSpace(req.Content)
	if content == "" || len([]rune(content)) > 4000 {
		return nil, errs.ErrParam.WithMsg("消息须为 1-4000 字")
	}
	resolved, be := s.registry.ResolveTask(Scope{Name: model.ScopeTenant, ID: actor.TenantID}, "agent_review")
	if be != nil {
		return nil, be
	}
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
	if err := s.db.Create(&model.Message{ConversationID: conversation.ID, Role: "user", Content: content, Status: "ready"}).Error; err != nil {
		return nil, errs.ErrInternal
	}
	messages := s.agentHistory(conversation.ID)
	toolsEnabled := s.toolsEnabled(actor)
	var tools []aipkg.ToolDefinition
	if toolsEnabled {
		tools = []aipkg.ToolDefinition{{Name: "inspection.summary", Description: "按日期范围查询巡检任务、完成、逾期和异常汇总", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"start_date": map[string]any{"type": "string", "format": "date"}, "end_date": map[string]any{"type": "string", "format": "date"}, "community_id": map[string]any{"type": "string", "format": "uuid"}}}}}
	}
	promptTokens, completionTokens := 0, 0
	for step := 0; step < 3; step++ {
		result, err := s.callProvider(ctx, actor, resolved, messages, tools, &promptTokens, &completionTokens)
		if err != nil {
			return nil, err
		}
		if len(result.Calls) == 0 {
			if strings.TrimSpace(result.Text) == "" {
				return nil, errs.ErrInternal.WithMsg("模型未返回有效回答")
			}
			assistant := model.Message{ConversationID: conversation.ID, Role: "assistant", Content: result.Text, ProviderCode: resolved.Provider.Code, ModelID: resolved.Model.ModelID, Status: "ready", PromptTokens: promptTokens, CompletionTokens: completionTokens}
			if err := s.db.Create(&assistant).Error; err != nil {
				return nil, errs.ErrInternal
			}
			s.db.Model(&conversation).Update("updated_at", assistant.CreatedAt)
			return gin.H{"conversation_id": conversation.ID, "message": assistant}, nil
		}
		messages = append(messages, result.Assistant)
		if err := s.db.Create(&model.Message{ConversationID: conversation.ID, Role: "assistant", Content: result.Text, ProviderCode: resolved.Provider.Code, ModelID: resolved.Model.ModelID, Status: "ready", ToolCalls: types.JSONMap{"items": result.Calls}}).Error; err != nil {
			return nil, errs.ErrInternal
		}
		for _, call := range result.Calls {
			toolResult, be := s.executeAgentTool(actor, conversation.ID, call)
			if be != nil {
				return nil, be
			}
			if err := s.db.Create(&model.Message{ConversationID: conversation.ID, Role: "tool", Content: toolResult, ToolCallID: call.ID, ToolName: call.Name, Status: "ready"}).Error; err != nil {
				return nil, errs.ErrInternal
			}
			messages = append(messages, aipkg.TextMessage{Role: "tool", Name: call.Name, ToolCallID: call.ID, Content: toolResult})
		}
	}
	return nil, errs.ErrConflict.WithMsg("模型连续调用工具超过上限，请缩小查询范围后重试")
}

func (s *AgentService) agentHistory(conversationID string) []aipkg.TextMessage {
	messages := []aipkg.TextMessage{{Role: "system", Content: "你是 AI云管家的物业巡检 Agent。只读工具只能在确有数据查询需要时调用，不得编造查询结果，不得执行写操作。没有工具结果时不得声称已经查询。"}}
	var rows []model.Message
	s.db.Where("conversation_id = ? AND status = ?", conversationID, "ready").Order("created_at, id").Find(&rows)
	for _, row := range rows {
		message := aipkg.TextMessage{Role: row.Role, Content: row.Content, Name: row.ToolName, ToolCallID: row.ToolCallID}
		if raw, ok := row.ToolCalls["items"]; ok {
			data, _ := json.Marshal(raw)
			_ = json.Unmarshal(data, &message.ToolCalls)
		}
		messages = append(messages, message)
	}
	return messages
}

func (s *AgentService) callProvider(ctx context.Context, actor Actor, resolved *ResolvedTask, messages []aipkg.TextMessage, tools []aipkg.ToolDefinition, promptTokens, completionTokens *int) (aipkg.TextChatResult, *errs.Error) {
	if s.usage == nil {
		return aipkg.TextChatResult{}, errs.ErrInternal.WithMsg("AI 配额服务未配置")
	}
	reservation, be := s.usage.Reserve(actor, "agent_review")
	if be != nil {
		return aipkg.TextChatResult{}, be
	}
	failed := true
	prompt, completion := 0, 0
	defer func() {
		s.usage.Finish(reservation, prompt, completion, failed)
		*promptTokens += prompt
		*completionTokens += completion
	}()
	result, err := aipkg.ChatText(ctx, aipkg.TextChatParams{
		Protocol: resolved.Provider.Protocol, BaseURL: resolved.Provider.BaseURL, APIKey: resolved.APIKey, Model: resolved.Model.ModelID,
		TimeoutSeconds: resolved.Binding.TimeoutSeconds, MaxOutput: resolved.Model.MaxOutput, Messages: messages, Tools: tools,
	})
	prompt, completion = result.PromptTokens, result.CompletionTokens
	failed = err != nil
	if err != nil {
		return aipkg.TextChatResult{}, errs.ErrInternal.WithMsg(err.Error())
	}
	return result, nil
}

func (s *AgentService) executeAgentTool(actor Actor, conversationID string, call aipkg.ToolCall) (string, *errs.Error) {
	if !s.toolsEnabled(actor) {
		return "", errs.ErrNoPerm.WithMsg("该工具已被租户停用")
	}
	var arguments map[string]any
	if err := json.Unmarshal([]byte(call.Arguments), &arguments); err != nil {
		return "", errs.ErrParam.WithMsg("工具参数不是有效 JSON 对象")
	}
	result, be := s.mcp.Execute(actor, call.Name, arguments)
	audit := model.AgentCall{TenantID: actor.TenantID, UserID: actor.UserID, ConversationID: conversationID, ToolCode: call.Name, Arguments: types.JSONMap(arguments), Status: "success"}
	if be != nil {
		audit.Status = "failed"
		audit.ErrorMessage = strutil.Truncate(be.Msg, 512)
	} else {
		audit.Result = types.JSONMap{"data": result}
	}
	if err := s.db.Create(&audit).Error; err != nil {
		return "", errs.ErrInternal
	}
	if be != nil {
		return "", be
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", errs.ErrInternal
	}
	return string(data), nil
}

func (s *AgentService) Calls(actor Actor, conversationID string) ([]model.AgentCall, *errs.Error) {
	var conversation model.Conversation
	if err := s.db.First(&conversation, "id = ? AND tenant_id = ? AND user_id = ?", conversationID, actor.TenantID, actor.UserID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var rows []model.AgentCall
	if err := s.db.Where("conversation_id = ?", conversationID).Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return rows, nil
}
