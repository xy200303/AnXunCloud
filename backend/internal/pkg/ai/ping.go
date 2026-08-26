// ping 连接性测试：用表单当前值（可未保存）向模型发一个最小文本请求，
// 验证协议/接口地址/API Key/模型名是否可用。不经过 sys_config，不影响业务配置。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PingParams 连接测试参数（来自配置表单，显式传入而非读配置）。
type PingParams struct {
	Protocol       string // openai_chat / openai_responses / gemini / claude（空=openai_chat）
	BaseURL        string
	APIKey         string
	Model          string
	TimeoutSeconds int // <=0 默认 30
}

// pingPrompt 最小测试提示词：要求模型简短回复，仅用于验证链路。
const pingPrompt = "ping，请回复 ok"

// Ping 按协议分发执行最小文本请求，返回模型回复文本（截断 200 字符）。
func Ping(ctx context.Context, p PingParams) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("接口地址不能为空")
	}
	if strings.TrimSpace(p.Model) == "" {
		return "", fmt.Errorf("模型名不能为空")
	}
	timeout := p.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	httpc := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	apiKey := strings.TrimSpace(p.APIKey)

	var reply string
	var err error
	switch strings.ToLower(strings.TrimSpace(p.Protocol)) {
	case "", ProtocolOpenAIChat:
		reply, err = pingOpenAIChat(ctx, httpc, baseURL, apiKey, p.Model)
	case ProtocolOpenAIResponses:
		reply, err = pingOpenAIResponses(ctx, httpc, baseURL, apiKey, p.Model)
	case ProtocolGemini:
		reply, err = pingGemini(ctx, httpc, baseURL, apiKey, p.Model)
	case ProtocolClaude:
		reply, err = pingClaude(ctx, httpc, baseURL, apiKey, p.Model)
	default:
		return "", fmt.Errorf("未知协议类型: %q", p.Protocol)
	}
	if err != nil {
		return "", err
	}
	return truncate(reply, 200), nil
}

func pingOpenAIChat(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string) (string, error) {
	payload := map[string]any{
		"model":      model,
		"messages":   []map[string]any{{"role": "user", "content": pingPrompt}},
		"max_tokens": 32,
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/chat/completions", map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("响应无有效内容")
	}
	return out.Choices[0].Message.Content, nil
}

func pingOpenAIResponses(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string) (string, error) {
	payload := map[string]any{
		"model": model,
		"input": []map[string]any{{
			"role":    "user",
			"content": []map[string]any{{"type": "input_text", "text": pingPrompt}},
		}},
		"max_output_tokens": 32,
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/responses", map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	var sb strings.Builder
	for _, item := range out.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				sb.WriteString(part.Text)
			}
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("响应无有效内容")
	}
	return sb.String(), nil
}

func pingGemini(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string) (string, error) {
	payload := map[string]any{
		"contents": []map[string]any{{
			"role":  "user",
			"parts": []map[string]any{{"text": pingPrompt}},
		}},
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/models/"+model+":generateContent", map[string]string{
		"x-goog-api-key": apiKey,
	}, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("响应无有效内容")
	}
	var sb strings.Builder
	for _, part := range out.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("响应无有效内容")
	}
	return sb.String(), nil
}

func pingClaude(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string) (string, error) {
	payload := map[string]any{
		"model":      model,
		"max_tokens": 32,
		"messages":   []map[string]any{{"role": "user", "content": pingPrompt}},
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/messages", map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": anthropicVersion,
	}, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}
	var sb strings.Builder
	for _, part := range out.Content {
		if part.Type == "text" {
			sb.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return "", fmt.Errorf("响应无有效内容")
	}
	return sb.String(), nil
}
