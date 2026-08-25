// gemini 协议适配器：Google Gemini generateContent
// （POST {base_url}/models/{model}:generateContent，header x-goog-api-key）。
// 图片一律 inline_data（mime_type + base64）：云存储 URL 也先 HTTP GET 下载字节再内联（8MB 上限）；
// system 审核规则走 system_instruction；响应取 candidates[0].content.parts[] 的 text 拼接。
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// callGemini 执行 Gemini generateContent 调用并解析响应。
func (c *Client) callGemini(ctx context.Context, httpc *http.Client, baseURL, apiKey, model string, input ReviewInput) (*ReviewResult, error) {
	parts := make([]map[string]any, 0, 8)
	for _, p := range c.buildParts(input) {
		if p.img != nil {
			mime, b64, err := p.img.inline(ctx, httpc)
			if err != nil {
				continue // 下载失败跳过该图（与本地读失败跳过同策略）
			}
			parts = append(parts, map[string]any{
				"inline_data": map[string]any{"mime_type": mime, "data": b64},
			})
			continue
		}
		parts = append(parts, map[string]any{"text": p.text})
	}
	payload := map[string]any{
		"system_instruction": map[string]any{"parts": []map[string]any{{"text": c.rules()}}},
		"contents":           []map[string]any{{"role": "user", "parts": parts}},
	}
	respBody, err := postJSON(ctx, httpc, baseURL+"/models/"+model+":generateContent", map[string]string{
		"x-goog-api-key": strings.TrimSpace(apiKey),
	}, payload)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	if len(out.Candidates) == 0 {
		return nil, fmt.Errorf("响应无有效内容")
	}
	var sb strings.Builder
	for _, part := range out.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	if strings.TrimSpace(sb.String()) == "" {
		return nil, fmt.Errorf("响应无有效内容")
	}
	return parseReview(sb.String())
}
