package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
)

func TestAgentExecutesWhitelistedToolAndPersistsAudit(t *testing.T) {
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []map[string]any `json:"messages"`
			Tools    []any            `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			if len(body.Tools) != 1 {
				t.Fatalf("工具白名单未传递: %#v", body.Tools)
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_summary","type":"function","function":{"name":"inspection.summary","arguments":"{\"start_date\":\"2026-10-01\",\"end_date\":\"2026-10-02\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":3}}`))
			return
		}
		foundTool := false
		for _, message := range body.Messages {
			if message["role"] == "tool" && message["tool_call_id"] == "call_summary" {
				foundTool = true
			}
		}
		if !foundTool {
			t.Fatalf("第二次调用缺少工具结果: %#v", body.Messages)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"已完成真实汇总"}}],"usage":{"prompt_tokens":20,"completion_tokens":5}}`))
	}))
	defer provider.Close()
	db, err := gorm.Open(sqlite.Open("file:agent_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.Model{}, &model.TaskBinding{}, &model.Conversation{}, &model.Message{}, &model.Quota{}, &model.UsageReservation{}, &model.AgentTool{}, &model.AgentCall{}, &sysmodel.SysRole{}, &sysmodel.SysUser{}, &sysmodel.Tenant{}, &sysmodel.SysMenu{}, &sysmodel.SysRoleMenu{}); err != nil {
		t.Fatal(err)
	}
	actor := Actor{TenantID: "01a00000-0000-7000-8000-000000000001", UserID: "01a00000-0000-7000-8000-000000000002"}
	tenant := sysmodel.Tenant{Code: "agent-test", Name: "Agent 测试租户", Status: sysmodel.StatusEnabled}
	tenant.ID = actor.TenantID
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	role := sysmodel.SysRole{Code: sysmodel.SuperAdminCode, Name: "超级管理员", ScopeLevel: sysmodel.ScopePlatform, Status: sysmodel.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	user := sysmodel.SysUser{Username: "agent-admin", Name: "Agent 管理员", TenantID: actor.TenantID, RoleID: &role.ID, Status: sysmodel.StatusEnabled}
	user.ID = actor.UserID
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := authz.Init(db); err != nil {
		t.Fatal(err)
	}
	if err := authz.SyncAll(db); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistryService(db)
	platform := Scope{Name: model.ScopePlatform, ID: model.PlatformScopeID}
	if be := registry.SaveProvider(platform, "test", ProviderReq{Code: "test", Name: "隔离供应商", Protocol: "openai_chat", BaseURL: provider.URL + "/v1", CredentialRef: "AGENT_TEST_KEY", Status: model.Enabled}); be != nil {
		t.Fatal(be.Msg)
	}
	if be := registry.SaveModel(platform, "test", "agent-model", ModelReq{DisplayName: "Agent 模型", ContextWindow: 8192, MaxOutput: 128, Capabilities: []string{"text", "tools"}, Status: model.Enabled}); be != nil {
		t.Fatal(be.Msg)
	}
	if be := registry.SaveBinding(platform, "agent_review", BindingReq{ProviderCode: "test", ModelID: "agent-model", TimeoutSeconds: 5, MaxRetries: 0, Status: model.Enabled}); be != nil {
		t.Fatal(be.Msg)
	}
	t.Setenv("AGENT_TEST_KEY", "isolated-secret")
	mcp := NewMCPService(db, func(c *gin.Context, start, end time.Time) (gin.H, error) {
		return gin.H{"total": 7, "done": 5, "abnormal": 1}, nil
	})
	svc := NewAgentService(db, registry, NewUsageService(db), mcp)
	result, be := svc.Chat(context.Background(), actor, ChatRequest{Content: "查询今天巡检汇总"})
	if be != nil {
		t.Fatal(be.Msg)
	}
	if calls != 2 || result["message"].(model.Message).Content != "已完成真实汇总" {
		t.Fatalf("Agent 未完成真实工具循环: calls=%d result=%#v", calls, result)
	}
	audit, be := svc.Calls(actor, result["conversation_id"].(string))
	if be != nil || len(audit) != 1 || audit[0].Status != "success" || audit[0].ToolCode != "inspection.summary" {
		t.Fatalf("工具审计错误: %#v, %v", audit, be)
	}
	var usage struct {
		Requests int64
		Tokens   int64
	}
	if err := db.Model(&model.UsageReservation{}).Where("tenant_id = ? AND task_type = ? AND status = ?", actor.TenantID, "agent_review", "done").Select("COUNT(*) AS requests, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens").Scan(&usage).Error; err != nil {
		t.Fatal(err)
	}
	if usage.Requests != 2 || usage.Tokens != 38 {
		t.Fatalf("两次模型调用未完整计入用量: %#v", usage)
	}
}
