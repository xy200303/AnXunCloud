// openai_responses 协议适配器：OpenAI Responses API（POST {base_url}/responses）。
// system 审核规则走官方 instructions 字段（等价于 input 首条 role:system，二选一，此处按官方推荐 instructions）；
// 图片为 input_image（url 或 data URL），响应取 output 数组中 type:message 的 output_text 拼接。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// callOpenAIResponses 执行 Responses API 调用并解析响应。
func (c *Client) callOpenAIResponses(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string, input ReviewInput) (*ReviewResult, error) {
	content := make([]map[string]any, 0, 8)
	for _, p := range c.buildParts(input) {
		if p.img != nil {
			content = append(content, map[string]any{"type": "input_image", "image_url": p.img.ref()})
			continue
		}
		content = append(content, map[string]any{"type": "input_text", "text": p.text})
	}
	payload := map[string]any{
		"model":             model,
		"instructions":      c.rules(), // 官方字段：等价于 system 消息
		"input":             []map[string]any{{"role": "user", "content": content}},
		"max_output_tokens": 1024,
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/responses", map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(apiKey),
	}, payload)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("响应解析失败: %w", err)
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
		return nil, fmt.Errorf("响应无有效内容")
	}
	return parseReview(sb.String())
}
