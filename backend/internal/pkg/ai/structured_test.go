package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// captureServer 拦截请求体的协议模拟服务：handler 返回各协议的合法响应，
// 请求体逐次留档（适配器结构化参数断言用）。
func captureServer(t *testing.T, handler func(body map[string]any) []byte) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求体非 JSON: %v", err)
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(handler(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func testClient() *Client {
	return NewClient(func(string) (string, bool) { return "", false })
}

func testInput() ReviewInput {
	return ReviewInput{PointName: "配电房", PointType: "fire_control", CheckItems: []string{"压力表指针在绿区"}}
}

// TestOpenAIChatStructuredOutput openai_chat 请求体带 response_format=json_object。
func TestOpenAIChatStructuredOutput(t *testing.T) {
	srv, bodies := captureServer(t, func(map[string]any) []byte {
		return []byte(`{"choices":[{"message":{"content":"{\"quality\":{\"pass\":true,\"issue\":\"\"},\"verdict\":\"pass\",\"reason\":\"正常\"}"}}]}`)
	})
	res, err := testClient().callOpenAIChat(context.Background(), srv.Client(), srv.URL, "k", "m", testInput())
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict=%q，期望 pass", res.Verdict)
	}
	rf, ok := (*bodies)[0]["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Fatalf("请求体缺 response_format=json_object: %v", (*bodies)[0]["response_format"])
	}
}

// TestOpenAIResponsesStructuredOutput openai_responses 请求体带 text.format=json_schema（含 schema 结构）。
func TestOpenAIResponsesStructuredOutput(t *testing.T) {
	srv, bodies := captureServer(t, func(map[string]any) []byte {
		return []byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"{\"quality\":{\"pass\":true,\"issue\":\"\"},\"verdict\":\"pass\",\"reason\":\"正常\"}"}]}]}`)
	})
	res, err := testClient().callOpenAIResponses(context.Background(), srv.Client(), srv.URL, "k", "m", testInput())
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict=%q，期望 pass", res.Verdict)
	}
	text, ok := (*bodies)[0]["text"].(map[string]any)
	if !ok {
		t.Fatalf("请求体缺 text 段: %v", (*bodies)[0])
	}
	format, ok := text["format"].(map[string]any)
	if !ok || format["type"] != "json_schema" || format["name"] != "review" {
		t.Fatalf("text.format 异常: %v", text["format"])
	}
	schema, ok := format["schema"].(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("schema 异常: %v", format["schema"])
	}
}

// TestGeminiStructuredOutput gemini 请求体 generationConfig 带 responseMimeType+responseSchema（方言 type 大写）。
func TestGeminiStructuredOutput(t *testing.T) {
	srv, bodies := captureServer(t, func(map[string]any) []byte {
		return []byte(`{"candidates":[{"content":{"parts":[{"text":"{\"quality\":{\"pass\":true,\"issue\":\"\"},\"verdict\":\"pass\",\"reason\":\"正常\"}"}]}}]}`)
	})
	res, err := testClient().callGemini(context.Background(), srv.Client(), srv.URL, "k", "m", testInput())
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict=%q，期望 pass", res.Verdict)
	}
	genCfg, ok := (*bodies)[0]["generationConfig"].(map[string]any)
	if !ok {
		t.Fatalf("请求体缺 generationConfig: %v", (*bodies)[0])
	}
	if genCfg["responseMimeType"] != "application/json" {
		t.Fatalf("responseMimeType 异常: %v", genCfg["responseMimeType"])
	}
	schema, ok := genCfg["responseSchema"].(map[string]any)
	if !ok || schema["type"] != "OBJECT" {
		t.Fatalf("responseSchema 方言应为 type 大写 OBJECT: %v", genCfg["responseSchema"])
	}
	// 关思考的 thinkingConfig 与结构化参数共存（不互斥）
	if _, ok := genCfg["thinkingConfig"]; !ok {
		t.Errorf("thinkingConfig 丢失: %v", genCfg)
	}
}

// TestClaudeToolUseStructuredOutput claude 请求体带 submit_review 工具 + tool_choice 强制调用；
// 响应解析读 tool_use 块 input（不走文本提取）。
func TestClaudeToolUseStructuredOutput(t *testing.T) {
	srv, bodies := captureServer(t, func(map[string]any) []byte {
		return []byte(`{"content":[
			{"type":"text","text":"好的，审核完成。"},
			{"type":"tool_use","name":"submit_review","input":{"quality":{"pass":true,"issue":""},"verdict":"review","reason":"保险销疑似脱落","items":[{"name":"压力表指针在绿区","verdict":"pass","reason":"正常","reading":"1.2MPa","abnormal_tags":[]}]}}
		]}`)
	})
	res, err := testClient().callClaude(context.Background(), srv.Client(), srv.URL, "k", "m", testInput())
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if res.Verdict != VerdictReview || res.Reason != "保险销疑似脱落" {
		t.Errorf("tool_use input 解析异常: %+v", res)
	}
	if len(res.Items) != 1 || res.Items[0].Reading != "1.2MPa" {
		t.Errorf("逐项结论解析异常: %+v", res.Items)
	}
	tools, ok := (*bodies)[0]["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("请求体缺 tools: %v", (*bodies)[0])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["name"] != "submit_review" {
		t.Fatalf("工具名异常: %v", tool["name"])
	}
	if _, ok := tool["input_schema"].(map[string]any); !ok {
		t.Fatalf("工具缺 input_schema: %v", tool)
	}
	tc, ok := (*bodies)[0]["tool_choice"].(map[string]any)
	if !ok || tc["type"] != "tool" || tc["name"] != "submit_review" {
		t.Fatalf("tool_choice 异常: %v", (*bodies)[0]["tool_choice"])
	}
}

// TestStructuredOutputFallback 网关不识结构化参数（400 + 参数名关键词）时去参数重试一次并放行；
// 第二次请求体不含结构化字段。
func TestStructuredOutputFallback(t *testing.T) {
	var calls atomic.Int32
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid parameter: response_format is not supported"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"quality\":{\"pass\":true,\"issue\":\"\"},\"verdict\":\"pass\",\"reason\":\"正常\"}"}}]}`))
	}))
	defer srv.Close()

	res, err := testClient().callOpenAIChat(context.Background(), srv.Client(), srv.URL, "k", "m", testInput())
	if err != nil {
		t.Fatalf("降级重试后仍失败: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict=%q，期望 pass", res.Verdict)
	}
	if calls.Load() != 2 {
		t.Fatalf("调用次数=%d，期望 2（首次 400 + 降级重试）", calls.Load())
	}
	if _, has := bodies[0]["response_format"]; !has {
		t.Error("首次请求应带 response_format")
	}
	if _, has := bodies[1]["response_format"]; has {
		t.Error("降级重试请求不应带 response_format")
	}
}

// TestStructuredOutputUnsupportedDetector 降级触发条件：400 + 结构化参数名关键词才降级；
// 其他错误（401 鉴权/500/429）不误判。
func TestStructuredOutputUnsupportedDetector(t *testing.T) {
	for _, msg := range []string{
		"大模型返回 400: Invalid parameter: response_format",
		"大模型返回 400: responseMimeType is not supported",
		"大模型返回 400: unknown field tool_choice",
		"大模型返回 400: json_schema not supported",
	} {
		if !structuredOutputUnsupported(errForTest(msg)) {
			t.Errorf("%q 应判为不识结构化参数", msg)
		}
	}
	for _, msg := range []string{
		"大模型返回 401: unauthorized",
		"大模型返回 500: boom",
		"大模型返回 400: model not found",
		"大模型请求失败: connection refused",
	} {
		if structuredOutputUnsupported(errForTest(msg)) {
			t.Errorf("%q 不应判为不识结构化参数", msg)
		}
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }

func errForTest(msg string) error { return testErr(msg) }
