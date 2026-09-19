// claude 协议适配器：Anthropic Messages API（POST {base_url}/messages，
// headers x-api-key + anthropic-version: 2023-06-01）。
// 图片一律 base64 source（与 gemini 共用下载 helper：云存储 URL 先下载再内联）；
// system 审核规则走顶层 system 字段；结构化输出走 submit_review 工具强制调用
// （tool_choice），响应读 content[] 中 type:tool_use 块的 input（不再走文本提取）；
// 网关不识 tools/tool_choice 时降级为纯文本（parseReview 提取兜底）。
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

// reviewToolName 结构化输出工具名（tool_choice 强制调用，input 即审核结论）。
const reviewToolName = "submit_review"

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
		// 结构化输出：工具强制调用，模型结论装进 submit_review 的 input（语义校验仍由 validateReviewJSON 兜底）
		"tools": []map[string]any{{
			"name": reviewToolName, "description": "提交打卡审核结论（照片质量 + 整体结论 + 逐项结论）",
			"input_schema": reviewSchemaOpenAI(),
		}},
		"tool_choice": map[string]any{"type": "tool", "name": reviewToolName},
	}
	respBody, err := postWithStructuredFallback(ctx, httpc, baseURL+"/messages", map[string]string{
		"x-api-key":         strings.TrimSpace(apiKey),
		"anthropic-version": anthropicVersion,
	}, payload, func() {
		delete(payload, "tools")
		delete(payload, "tool_choice")
	})
	if err != nil {
		return nil, err
	}

	var out struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	// 优先读 tool_use 块（结构化输出）；降级响应（无 tools）回落文本提取
	var sb strings.Builder
	for _, part := range out.Content {
		switch {
		case part.Type == "tool_use" && part.Name == reviewToolName && len(part.Input) > 0:
			return validateReviewJSON(part.Input)
		case part.Type == "text":
			sb.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(sb.String()) == "" {
		return nil, fmt.Errorf("响应无有效内容")
	}
	return parseReview(sb.String())
}
