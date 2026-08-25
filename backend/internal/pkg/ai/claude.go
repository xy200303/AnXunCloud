// claude 协议适配器：Anthropic Messages API（POST {base_url}/messages，
// headers x-api-key + anthropic-version: 2023-06-01）。
// 图片一律 base64 source（与 gemini 共用下载 helper：云存储 URL 先下载再内联）；
// system 审核规则走顶层 system 字段；响应取 content[] 中 type:text 拼接。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// anthropicVersion Anthropic API 版本头（固定 2023-06-01）。
const anthropicVersion = "2023-06-01"

// callClaude 执行 Anthropic Messages 调用并解析响应。
func (c *Client) callClaude(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string, input ReviewInput) (*ReviewResult, error) {
	content := make([]map[string]any, 0, 8)
	for _, p := range c.buildParts(input) {
		if p.img != nil {
			mime, b64, err := p.img.inline(ctx, httpc)
			if err != nil {
				continue // 下载失败跳过该图（与本地读失败跳过同策略）
			}
			content = append(content, map[string]any{
				"type":   "image",
				"source": map[string]any{"type": "base64", "media_type": mime, "data": b64},
			})
			continue
		}
		content = append(content, map[string]any{"type": "text", "text": p.text})
	}
	payload := map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"system":     c.rules(),
		"messages":   []map[string]any{{"role": "user", "content": content}},
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/messages", map[string]string{
		"x-api-key":         strings.TrimSpace(apiKey),
		"anthropic-version": anthropicVersion,
	}, payload)
	if err != nil {
		return nil, err
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	var sb strings.Builder
	for _, part := range out.Content {
		if part.Type == "text" {
			sb.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return nil, fmt.Errorf("响应无有效内容")
	}
	return parseReview(sb.String())
}
