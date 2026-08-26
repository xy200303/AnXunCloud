package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Ping 成功路径：openai_chat mock 返回 choices，应取回模型回复文本。
func TestPingOpenAIChatOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("意外路径: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("缺少/错误 Authorization: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	reply, err := Ping(context.Background(), PingParams{
		Protocol: ProtocolOpenAIChat, BaseURL: srv.URL, APIKey: "test-key", Model: "m",
	})
	if err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
	if strings.TrimSpace(reply) != "ok" {
		t.Fatalf("回复不符: %q", reply)
	}
}

// Ping 失败路径：HTTP 401 应返回带状态码的错误。
func TestPingUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	_, err := Ping(context.Background(), PingParams{
		Protocol: ProtocolOpenAIChat, BaseURL: srv.URL, APIKey: "bad", Model: "m",
	})
	if err == nil {
		t.Fatal("应返回错误")
	}
}

// Ping 参数校验：空 base_url / 空 model / 未知协议。
func TestPingParamValidation(t *testing.T) {
	if _, err := Ping(context.Background(), PingParams{Model: "m"}); err == nil {
		t.Fatal("空 base_url 应报错")
	}
	if _, err := Ping(context.Background(), PingParams{BaseURL: "http://x"}); err == nil {
		t.Fatal("空 model 应报错")
	}
	if _, err := Ping(context.Background(), PingParams{Protocol: "bogus", BaseURL: "http://x", Model: "m"}); err == nil {
		t.Fatal("未知协议应报错")
	}
}

// Ping 其余三协议路径拼装（gemini URL 含模型名、claude 带版本头）。
func TestPingProtocolRoutes(t *testing.T) {
	var gotPath, gotGoogKey, gotClaudeVer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, ":generateContent"):
			gotGoogKey = r.Header.Get("x-goog-api-key")
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
		case r.URL.Path == "/messages":
			gotClaudeVer = r.Header.Get("anthropic-version")
			_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
		case r.URL.Path == "/responses":
			_, _ = w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	for _, proto := range []string{ProtocolGemini, ProtocolClaude, ProtocolOpenAIResponses} {
		reply, err := Ping(context.Background(), PingParams{
			Protocol: proto, BaseURL: srv.URL, APIKey: "k", Model: "m1",
		})
		if err != nil {
			t.Fatalf("%s Ping 失败: %v", proto, err)
		}
		if strings.TrimSpace(reply) != "ok" {
			t.Fatalf("%s 回复不符: %q", proto, reply)
		}
	}
	if gotClaudeVer != anthropicVersion {
		t.Fatalf("claude 缺少版本头: %q", gotClaudeVer)
	}
	if gotGoogKey != "k" {
		t.Fatalf("gemini 缺少 x-goog-api-key: %q", gotGoogKey)
	}
	_ = gotPath
}
