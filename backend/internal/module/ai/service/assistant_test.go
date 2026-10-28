package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
)

func TestAssistantCallsBoundProviderAndPersistsUsage(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer isolated-secret" {
			t.Fatalf("unexpected provider request: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "assistant-model" {
			t.Fatalf("wrong model: %v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"来自隔离供应商的回答"}}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`))
	}))
	defer providerServer.Close()
	db, err := gorm.Open(sqlite.Open("file:assistant_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.Model{}, &model.TaskBinding{}, &model.Conversation{}, &model.Message{}, &model.Quota{}, &model.UsageReservation{}); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&model.Quota{}) || !db.Migrator().HasTable(&model.UsageReservation{}) {
		t.Fatal("AI usage tables were not migrated")
	}
	registry := NewRegistryService(db)
	platform := Scope{Name: model.ScopePlatform, ID: model.PlatformScopeID}
	if be := registry.SaveProvider(platform, "test", ProviderReq{Code: "test", Name: "隔离供应商", Protocol: "openai_chat", BaseURL: providerServer.URL + "/v1", CredentialRef: "ASSISTANT_TEST_KEY", Status: model.Enabled}); be != nil {
		t.Fatal(be.Msg)
	}
	if be := registry.SaveModel(platform, "test", "assistant-model", ModelReq{DisplayName: "助手模型", ContextWindow: 8192, MaxOutput: 128, Capabilities: []string{"text", "tools"}, Status: model.Enabled}); be != nil {
		t.Fatal(be.Msg)
	}
	if be := registry.SaveBinding(platform, "assistant", BindingReq{ProviderCode: "test", ModelID: "assistant-model", TimeoutSeconds: 5, MaxRetries: 0, Status: model.Enabled}); be != nil {
		t.Fatal(be.Msg)
	}
	t.Setenv("ASSISTANT_TEST_KEY", "isolated-secret")
	svc := NewAssistantService(db, registry, NewUsageService(db))
	actor := Actor{TenantID: "01a00000-0000-7000-8000-000000000001", UserID: "01a00000-0000-7000-8000-000000000002"}
	result, be := svc.Chat(context.Background(), actor, ChatRequest{Content: "你好"})
	if be != nil {
		t.Fatal(be.Msg)
	}
	message := result["message"].(model.Message)
	if message.Content != "来自隔离供应商的回答" || message.PromptTokens != 11 || message.CompletionTokens != 7 {
		t.Fatalf("unexpected assistant message: %#v", message)
	}
	messages, be := svc.Messages(actor, result["conversation_id"].(string))
	if be != nil || len(messages) != 2 {
		t.Fatalf("conversation was not persisted: %v, %#v", be, messages)
	}
	other := Actor{TenantID: actor.TenantID, UserID: "01a00000-0000-7000-8000-000000000003"}
	if _, be := svc.Messages(other, result["conversation_id"].(string)); be == nil {
		t.Fatal("another user must not read this conversation")
	}
}
