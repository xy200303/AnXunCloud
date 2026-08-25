// openai_chat 协议适配器（默认）：OpenAI 兼容 Chat Completions（POST {base_url}/chat/completions）。
// 现状逻辑原样保留：system 审核规则 + user 多模态 content（text / image_url），
// local 图片转 base64 data URL，云存储直接传 URL。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// callOpenAIChat 执行 Chat Completions 调用并解析响应。
func (c *Client) callOpenAIChat(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string, input ReviewInput) (*ReviewResult, error) {
	payload := map[string]any{
		"model":      model,
		"messages":   c.buildMessages(input),
		"max_tokens": 1024,
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/chat/completions", map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(apiKey),
	}, payload)
	if err != nil {
		return nil, err
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("响应无有效内容")
	}
	return parseReview(out.Choices[0].Message.Content)
}

// buildMessages 组装 vision messages：system 审核规则 + user 文本上下文与图片（OpenAI content 格式）。
func (c *Client) buildMessages(input ReviewInput) []map[string]any {
	content := make([]map[string]any, 0, 8)
	for _, p := range c.buildParts(input) {
		if p.img != nil {
			content = append(content, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": p.img.ref()},
			})
			continue
		}
		content = append(content, map[string]any{"type": "text", "text": p.text})
	}
	return []map[string]any{
		{"role": "system", "content": c.rules()},
		{"role": "user", "content": content},
	}
}
