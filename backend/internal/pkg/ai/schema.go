// 结构化输出（治本：生成侧约束模型输出，消除"正则找 JSON + 手搓解析"的脆弱性）。
// 各协议方言落地：openai_chat=response_format json_object（仅约束合法 JSON，兼容性最好，
// qwen/deepseek/openai 兼容网关都支持）；openai_responses=text.format json_schema；
// gemini=generationConfig.responseMimeType+responseSchema（方言：type 大写）；
// claude=submit_review 工具强制调用（tool_choice），解析读 tool_use 块 input。
// 网关不识结构化参数时（400 + 参数名关键词）去掉参数降级重试一次，parseReview 文本提取兜底。
package ai

import (
	"context"
	"net/http"
	"strings"

	"anxuncloud/internal/pkg/logger"

	"go.uber.org/zap"
)

// reviewSchemaOpenAI 审核结论 JSON Schema（OpenAI 方言：openai_responses json_schema / claude tool input_schema 共用）。
// 必填 quality+verdict；items 可空数组（模型无法逐项判断时省略）。
func reviewSchemaOpenAI() map[string]any {
	verdictEnum := map[string]any{"type": "string", "enum": []string{VerdictPass, VerdictReview, VerdictAbnormal}}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"quality": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pass":  map[string]any{"type": "boolean"},
					"issue": map[string]any{"type": "string"},
				},
				"required": []string{"pass", "issue"},
			},
			"verdict": verdictEnum,
			"reason":  map[string]any{"type": "string"},
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":          map[string]any{"type": "string"},
						"verdict":       verdictEnum,
						"reason":        map[string]any{"type": "string"},
						"reading":       map[string]any{"type": "string"},
						"abnormal_tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required": []string{"name", "verdict", "reason", "reading", "abnormal_tags"},
				},
			},
		},
		"required": []string{"quality", "verdict", "reason", "items"},
	}
}

// reviewSchemaGemini Gemini responseSchema 方言：与 OpenAI 方言同构，仅 type 值大写（OBJECT/ARRAY/STRING/BOOLEAN）。
func reviewSchemaGemini() map[string]any {
	return upperSchemaTypes(reviewSchemaOpenAI()).(map[string]any)
}

// upperSchemaTypes 递归大写 schema 中的 type 值（required/enum 等字符串数组原样保留）。
func upperSchemaTypes(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		if k == "type" {
			if s, ok := val.(string); ok {
				out[k] = strings.ToUpper(s)
				continue
			}
		}
		out[k] = upperSchemaTypes(val)
	}
	return out
}

// structuredOutputUnsupported 判定报错是否为网关不识结构化参数（400 + 参数名关键词）：
// 命中则由调用方去掉结构化参数降级重试一次，parseReview 文本提取兜底。
func structuredOutputUnsupported(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "大模型返回 400") {
		return false
	}
	for _, kw := range []string{"response_format", "responsemime", "json_schema", "tool_choice", "tools", "schema"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// postWithStructuredFallback 带结构化参数的 POST：网关不识参数时 strip 去掉结构化字段后重试一次（记 warn 日志）。
func postWithStructuredFallback(ctx context.Context, httpc *http.Client, url string, headers map[string]string, payload map[string]any, strip func()) ([]byte, error) {
	respBody, err := postJSON(ctx, httpc, url, headers, payload)
	if err == nil || !structuredOutputUnsupported(err) {
		return respBody, err
	}
	if logger.L != nil {
		logger.L.Warn("网关不支持结构化输出参数，降级为纯文本重试", zap.String("url", url), zap.Error(err))
	}
	strip()
	return postJSON(ctx, httpc, url, headers, payload)
}
