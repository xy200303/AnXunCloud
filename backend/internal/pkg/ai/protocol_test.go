package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockCfg 按 map 提供配置（ai.enabled/api_key/base_url/model 由用例补齐）。
func mockCfg(kv map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := kv[key]
		return v, ok
	}
}

// newMockClient 指向 httptest server 的客户端（enabled+key+model 固定，protocol 由用例给）。
func newMockClient(serverURL, protocol string) *Client {
	return NewClient(mockCfg(map[string]string{
		"ai.enabled":  "true",
		"ai.protocol": protocol,
		"ai.base_url": serverURL,
		"ai.api_key":  "test-key",
		"ai.model":    "test-model",
	}))
}

// reviewJSON 新格式（含 quality + reading）模型输出。
const reviewJSON = `{"quality":{"pass":true,"issue":""},"verdict":"review","reason":"压力表存疑","items":[{"instance_key":"test:item:","name":"压力表指针在绿区","verdict":"abnormal","reason":"指针在红区","reading":"1.8MPa"}]}`

// fakeJPEG 一张伪图片字节（协议层不校验真实图片格式）。
var fakeJPEG = []byte("\xff\xd8\xff\xd9")

// jsonStr 将模型输出转义为合法 JSON 字符串字面量（嵌入 mock 响应体用）。
func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// serveImage 在 mock server 上挂 /img.jpg（gemini/claude 远程 URL 下载内联路径用）。
func serveImage(mux *http.ServeMux) {
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(fakeJPEG)
	})
}

func TestProtocolOpenAIChat(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s，期望 POST", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("Authorization=%q，期望 Bearer test-key", auth)
		}
		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Role    string `json:"role"`
				Content json.RawMessage
			} `json:"messages"`
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if body.Model != "test-model" || body.MaxTokens != 1024 {
			t.Errorf("model/max_tokens 异常: %s", raw)
		}
		if len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" {
			t.Errorf("messages 结构异常: %s", raw)
		}
		// user content 应含 image_url 引用（云存储模式直接传 URL）
		if !strings.Contains(string(body.Messages[1].Content), `"image_url"`) {
			t.Errorf("user content 缺 image_url: %s", body.Messages[1].Content)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonStr(reviewJSON) + `}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := newMockClient(srv.URL, "openai_chat").ReviewCheckin(context.Background(), ReviewInput{
		PointName:  "配电房",
		ItemPhotos: []ItemPhoto{{Name: "灭火器在位", Photos: []PhotoRef{{URL: "http://example.com/a.jpg"}}}},
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	assertReviewResult(t, res)
}

func TestProtocolOpenAIResponses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/responses", func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("Authorization=%q，期望 Bearer test-key", auth)
		}
		var body struct {
			Model           string `json:"model"`
			Instructions    string `json:"instructions"`
			MaxOutputTokens int    `json:"max_output_tokens"`
			Input           []struct {
				Role    string           `json:"role"`
				Content []map[string]any `json:"content"`
			} `json:"input"`
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if body.Model != "test-model" || body.MaxOutputTokens != 1024 {
			t.Errorf("model/max_output_tokens 异常: %s", raw)
		}
		if body.Instructions == "" {
			t.Errorf("instructions（system 规则）为空: %s", raw)
		}
		if len(body.Input) != 1 || body.Input[0].Role != "user" {
			t.Errorf("input 结构异常: %s", raw)
		}
		var hasText, hasImage bool
		for _, part := range body.Input[0].Content {
			if part["type"] == "input_text" {
				hasText = true
			}
			if part["type"] == "input_image" {
				if _, ok := part["image_url"].(string); !ok {
					t.Errorf("input_image 缺 image_url: %v", part)
				}
				hasImage = true
			}
		}
		if !hasText || !hasImage {
			t.Errorf("content 缺 input_text/input_image: %s", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":` + jsonStr(reviewJSON) + `}]}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := newMockClient(srv.URL, "openai_responses").ReviewCheckin(context.Background(), ReviewInput{
		PointName:  "配电房",
		ItemPhotos: []ItemPhoto{{Name: "灭火器在位", Photos: []PhotoRef{{URL: "http://example.com/a.jpg"}}}},
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	assertReviewResult(t, res)
}

func TestProtocolGemini(t *testing.T) {
	mux := http.NewServeMux()
	serveImage(mux)
	var srv *httptest.Server
	mux.HandleFunc("/models/test-model:generateContent", func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("x-goog-api-key"); key != "test-key" {
			t.Errorf("x-goog-api-key=%q，期望 test-key", key)
		}
		var body struct {
			SystemInstruction struct {
				Parts []map[string]any `json:"parts"`
			} `json:"system_instruction"`
			Contents []struct {
				Role  string           `json:"role"`
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if len(body.SystemInstruction.Parts) != 1 {
			t.Errorf("system_instruction 异常: %s", raw)
		}
		if len(body.Contents) != 1 || body.Contents[0].Role != "user" {
			t.Errorf("contents 结构异常: %s", raw)
		}
		var hasText, hasInline bool
		for _, part := range body.Contents[0].Parts {
			if _, ok := part["text"]; ok {
				hasText = true
			}
			if inline, ok := part["inline_data"].(map[string]any); ok {
				if inline["mime_type"] != "image/jpeg" {
					t.Errorf("inline mime_type=%v，期望 image/jpeg", inline["mime_type"])
				}
				data, _ := inline["data"].(string)
				if decoded, err := base64.StdEncoding.DecodeString(data); err != nil || len(decoded) != len(fakeJPEG) {
					t.Errorf("inline data base64 异常")
				}
				hasInline = true
			}
		}
		if !hasText || !hasInline {
			t.Errorf("parts 缺 text/inline_data（远程 URL 应下载内联）: %s", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":` + jsonStr(reviewJSON) + `}]}}]}`))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	res, err := newMockClient(srv.URL, "gemini").ReviewCheckin(context.Background(), ReviewInput{
		PointName:  "配电房",
		ItemPhotos: []ItemPhoto{{Name: "灭火器在位", Photos: []PhotoRef{{URL: srv.URL + "/img.jpg"}}}},
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	assertReviewResult(t, res)
}

func TestProtocolClaude(t *testing.T) {
	mux := http.NewServeMux()
	serveImage(mux)
	var srv *httptest.Server
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("x-api-key"); key != "test-key" {
			t.Errorf("x-api-key=%q，期望 test-key", key)
		}
		if v := r.Header.Get("anthropic-version"); v != "2023-06-01" {
			t.Errorf("anthropic-version=%q，期望 2023-06-01", v)
		}
		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			System    string `json:"system"`
			Messages  []struct {
				Role    string           `json:"role"`
				Content []map[string]any `json:"content"`
			} `json:"messages"`
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if body.Model != "test-model" || body.MaxTokens != 1024 || body.System == "" {
			t.Errorf("model/max_tokens/system 异常: %s", raw)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
			t.Errorf("messages 结构异常: %s", raw)
		}
		var hasText, hasImage bool
		for _, part := range body.Messages[0].Content {
			if part["type"] == "text" {
				hasText = true
			}
			if part["type"] == "image" {
				src, _ := part["source"].(map[string]any)
				if src["type"] != "base64" || src["media_type"] != "image/jpeg" {
					t.Errorf("image source 异常: %v", src)
				}
				if decoded, err := base64.StdEncoding.DecodeString(src["data"].(string)); err != nil || len(decoded) != len(fakeJPEG) {
					t.Errorf("image data base64 异常")
				}
				hasImage = true
			}
		}
		if !hasText || !hasImage {
			t.Errorf("content 缺 text/image（远程 URL 应下载转 base64）: %s", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":` + jsonStr(reviewJSON) + `}]}`))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	res, err := newMockClient(srv.URL, "claude").ReviewCheckin(context.Background(), ReviewInput{
		PointName:  "配电房",
		ItemPhotos: []ItemPhoto{{Name: "灭火器在位", Photos: []PhotoRef{{URL: srv.URL + "/img.jpg"}}}},
	})
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	assertReviewResult(t, res)
}

// TestProtocolUnknown 未知协议报错（不发送请求）。
func TestProtocolUnknown(t *testing.T) {
	if _, err := newMockClient("http://127.0.0.1:0", "unknown_proto").ReviewCheckin(context.Background(), ReviewInput{PointName: "x"}); err == nil {
		t.Fatal("未知协议应报错")
	}
}

// assertReviewResult 校验 reviewJSON 的解析结果（quality/abnormal/reading 新字段）。
func assertReviewResult(t *testing.T, res *ReviewResult) {
	t.Helper()
	if !res.Quality.Pass || res.Quality.Issue != "" {
		t.Errorf("quality 异常: %+v", res.Quality)
	}
	if res.Verdict != VerdictReview || res.Reason != "压力表存疑" {
		t.Errorf("整体结论异常: %+v", res)
	}
	if len(res.Items) != 1 {
		t.Fatalf("逐项结论数=%d，期望 1", len(res.Items))
	}
	it := res.Items[0]
	if it.Name != "压力表指针在绿区" || it.Verdict != VerdictAbnormal || it.Reading != "1.8MPa" {
		t.Errorf("逐项结论异常: %+v", it)
	}
}

// TestParseReviewNewFormat 新格式：quality + abnormal + reading 全字段解析。
func TestParseReviewNewFormat(t *testing.T) {
	res, err := parseReview(`{"quality":{"pass":false,"issue":"照片模糊，请重新拍摄"},"verdict":"review","reason":"质量不达标","items":[
		{"instance_key":"test:item:","name":"灭火器压力正常","verdict":"pass","reason":"正常"},
		{"instance_key":"test:item:","name":"仪表读数","verdict":"abnormal","reason":"超量程","reading":"42.5℃"}
	]}`)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Quality.Pass || res.Quality.Issue != "照片模糊，请重新拍摄" {
		t.Errorf("quality 异常: %+v", res.Quality)
	}
	if len(res.Items) != 2 || res.Items[1].Verdict != VerdictAbnormal || res.Items[1].Reading != "42.5℃" {
		t.Errorf("逐项结论异常: %+v", res.Items)
	}
}

func TestParseReviewRejectsMissingQuality(t *testing.T) {
	if _, err := parseReview(`{"verdict":"pass","reason":"照片清晰，无异常","items":[{"instance_key":"test:item:","name":"消防枪头在位","verdict":"pass","reason":"在位"}]}`); err == nil {
		t.Fatal("缺少 quality 字段应报错")
	}
}

func TestJudgeInstruction(t *testing.T) {
	if got := judgeInstruction(PromptGeneral, "ok", "", nil, nil, nil); got != "" {
		t.Errorf("general 不应生成指令: %q", got)
	}
	min, max := 0.0, 40.0
	metric := judgeInstruction(PromptMetric, "number", "℃", &min, &max, nil)
	if !strings.Contains(metric, "℃") || !strings.Contains(metric, "reading") || !strings.Contains(metric, "[0, 40]") || !strings.Contains(metric, "服务端") {
		t.Errorf("metric 指令异常: %q", metric)
	}
	if got := judgeInstruction(PromptState, "enum", "", nil, nil, []string{"开启", "关闭"}); !strings.Contains(got, "实际状态") {
		t.Errorf("state 指令异常: %q", got)
	}
	for _, kind := range []string{PromptPresence, PromptDamage, PromptLabel, PromptPassage, PromptLeak, PromptTidiness} {
		if got := judgeInstruction(kind, "ok", "", nil, nil, nil); got == "" {
			t.Errorf("%s 应生成指令", kind)
		}
	}
}

func TestBuildMessagesJudgeInstruction(t *testing.T) {
	c := NewClient(func(string) (string, bool) { return "", false })
	min, max := 0.0, 1.0
	msgs := c.buildMessages(ReviewInput{
		PointName: "水泵房",
		ItemPhotos: []ItemPhoto{{
			InstanceKey: "template:meter:", Name: "仪表读数在正常范围", AIPromptKind: PromptMetric,
			ValueType: "number", Unit: "MPa", ValueMin: &min, ValueMax: &max,
			Photos: []PhotoRef{{URL: "http://example.com/meter.jpg"}},
		}},
	})
	content := msgs[1]["content"].([]map[string]any)
	var texts []string
	for _, part := range content {
		if part["type"] == "text" {
			texts = append(texts, part["text"].(string))
		}
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "识别要求：") || !strings.Contains(joined, "MPa") || !strings.Contains(joined, "instance_key") {
		t.Errorf("逐项标注缺识别要求或实例键: %s", joined)
	}
}
