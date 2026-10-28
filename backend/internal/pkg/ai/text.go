package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type TextMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type TextChatParams struct {
	Protocol       string
	BaseURL        string
	APIKey         string
	Model          string
	TimeoutSeconds int
	MaxOutput      int
	Messages       []TextMessage
	Tools          []ToolDefinition
}

type TextChatResult struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
	Calls            []ToolCall
	Assistant        TextMessage
}

type chatUsage struct {
	PromptTokens         int `json:"prompt_tokens"`
	CompletionTokens     int `json:"completion_tokens"`
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

func ChatText(ctx context.Context, p TextChatParams) (TextChatResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(p.Model) == "" || len(p.Messages) == 0 {
		return TextChatResult{}, fmt.Errorf("接口地址、模型和消息不能为空")
	}
	timeout := p.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	maxOutput := p.MaxOutput
	if maxOutput <= 0 {
		maxOutput = 32
	}
	if len(p.Tools) > 0 && p.Protocol != "" && p.Protocol != ProtocolOpenAIChat {
		return TextChatResult{}, fmt.Errorf("当前供应商协议不支持原生工具调用")
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	switch p.Protocol {
	case "", ProtocolOpenAIChat:
		return chatOpenAI(ctx, client, baseURL, p, maxOutput)
	case ProtocolOpenAIResponses:
		return chatResponses(ctx, client, baseURL, p, maxOutput)
	case ProtocolGemini:
		return chatGemini(ctx, client, baseURL, p, maxOutput)
	case ProtocolClaude:
		return chatClaude(ctx, client, baseURL, p, maxOutput)
	default:
		return TextChatResult{}, fmt.Errorf("未知协议类型: %q", p.Protocol)
	}
}

func chatOpenAI(ctx context.Context, client *http.Client, baseURL string, p TextChatParams, maxOutput int) (TextChatResult, error) {
	payload := map[string]any{"model": p.Model, "messages": p.Messages, "max_tokens": maxOutput}
	if len(p.Tools) > 0 {
		tools := make([]map[string]any, 0, len(p.Tools))
		for _, tool := range p.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": tool.InputSchema}})
		}
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}
	body, err := postJSON(ctx, client, baseURL+"/chat/completions", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload)
	if err != nil {
		return TextChatResult{}, err
	}
	var out struct {
		Choices []struct {
			Message TextMessage `json:"message"`
		} `json:"choices"`
		Usage chatUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return TextChatResult{}, fmt.Errorf("响应解析失败: %w", err)
	}
	if len(out.Choices) == 0 {
		return TextChatResult{}, fmt.Errorf("响应无有效内容")
	}
	message := out.Choices[0].Message
	if strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
		return TextChatResult{}, fmt.Errorf("响应无有效内容")
	}
	return TextChatResult{Text: message.Content, PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, Calls: message.ToolCalls, Assistant: message}, nil
}

func (c *ToolCall) UnmarshalJSON(data []byte) error {
	var value struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	c.ID, c.Name, c.Arguments = value.ID, value.Function.Name, value.Function.Arguments
	return nil
}

func (c ToolCall) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"id": c.ID, "type": "function",
		"function": map[string]any{"name": c.Name, "arguments": c.Arguments},
	})
}

func chatResponses(ctx context.Context, client *http.Client, baseURL string, p TextChatParams, maxOutput int) (TextChatResult, error) {
	input := make([]map[string]any, 0, len(p.Messages))
	for _, message := range p.Messages {
		partType := "input_text"
		if message.Role == "assistant" {
			partType = "output_text"
		}
		input = append(input, map[string]any{"role": message.Role, "content": []map[string]any{{"type": partType, "text": message.Content}}})
	}
	body, err := postJSON(ctx, client, baseURL+"/responses", map[string]string{"Authorization": "Bearer " + p.APIKey}, map[string]any{"model": p.Model, "input": input, "max_output_tokens": maxOutput})
	if err != nil {
		return TextChatResult{}, err
	}
	var out struct {
		Output []struct {
			Type    string                        `json:"type"`
			Content []struct{ Type, Text string } `json:"content"`
		} `json:"output"`
		Usage chatUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return TextChatResult{}, fmt.Errorf("响应解析失败: %w", err)
	}
	var text strings.Builder
	for _, item := range out.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text.WriteString(part.Text)
			}
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return TextChatResult{}, fmt.Errorf("响应无有效内容")
	}
	return TextChatResult{Text: text.String(), PromptTokens: out.Usage.InputTokens, CompletionTokens: out.Usage.OutputTokens}, nil
}

func chatGemini(ctx context.Context, client *http.Client, baseURL string, p TextChatParams, maxOutput int) (TextChatResult, error) {
	contents := make([]map[string]any, 0, len(p.Messages))
	for _, message := range p.Messages {
		role := "user"
		if message.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{"role": role, "parts": []map[string]any{{"text": message.Content}}})
	}
	body, err := postJSON(ctx, client, baseURL+"/models/"+p.Model+":generateContent", map[string]string{"x-goog-api-key": p.APIKey}, map[string]any{"contents": contents, "generationConfig": map[string]any{"maxOutputTokens": maxOutput}})
	if err != nil {
		return TextChatResult{}, err
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct{ Text string } `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata chatUsage `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return TextChatResult{}, fmt.Errorf("响应解析失败: %w", err)
	}
	var text strings.Builder
	if len(out.Candidates) > 0 {
		for _, part := range out.Candidates[0].Content.Parts {
			text.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return TextChatResult{}, fmt.Errorf("响应无有效内容")
	}
	return TextChatResult{Text: text.String(), PromptTokens: out.UsageMetadata.PromptTokenCount, CompletionTokens: out.UsageMetadata.CandidatesTokenCount}, nil
}

func chatClaude(ctx context.Context, client *http.Client, baseURL string, p TextChatParams, maxOutput int) (TextChatResult, error) {
	messages := make([]TextMessage, 0, len(p.Messages))
	var system strings.Builder
	for _, message := range p.Messages {
		if message.Role == "system" {
			system.WriteString(message.Content + "\n")
		} else {
			messages = append(messages, message)
		}
	}
	body, err := postJSON(ctx, client, baseURL+"/messages", map[string]string{"x-api-key": p.APIKey, "anthropic-version": anthropicVersion}, map[string]any{"model": p.Model, "system": strings.TrimSpace(system.String()), "messages": messages, "max_tokens": maxOutput})
	if err != nil {
		return TextChatResult{}, err
	}
	var out struct {
		Content []struct{ Type, Text string } `json:"content"`
		Usage   chatUsage                     `json:"usage"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return TextChatResult{}, fmt.Errorf("响应解析失败: %w", err)
	}
	var text strings.Builder
	for _, part := range out.Content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return TextChatResult{}, fmt.Errorf("响应无有效内容")
	}
	return TextChatResult{Text: text.String(), PromptTokens: out.Usage.InputTokens, CompletionTokens: out.Usage.OutputTokens}, nil
}
