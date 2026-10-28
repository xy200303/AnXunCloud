package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
)

func TestRegistryInheritanceAndCapabilityValidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Provider{}, &model.Model{}, &model.TaskBinding{}); err != nil {
		t.Fatal(err)
	}
	svc := NewRegistryService(db)
	platform := Scope{Name: model.ScopePlatform, ID: model.PlatformScopeID}
	provider := ProviderReq{Code: "bailian", Name: "百炼", Protocol: "openai_chat", BaseURL: "https://example.com/v1", CredentialRef: "TEST_CREDENTIAL_REF", Status: model.Enabled}
	if be := svc.SaveProvider(platform, "", provider); be != nil {
		t.Fatal(be.Msg)
	}
	provider.Name = "百炼（更新）"
	if be := svc.SaveProvider(platform, "bailian", provider); be != nil {
		t.Fatal(be.Msg)
	}
	platformProviders, be := svc.Providers(platform)
	if be != nil || len(platformProviders) != 1 || platformProviders[0]["name"] != "百炼（更新）" {
		t.Fatalf("provider update did not persist: %#v, %v", platformProviders, be)
	}
	textModel := ModelReq{DisplayName: "文本模型", ContextWindow: 8192, MaxOutput: 2048, Capabilities: []string{"text"}, Status: model.Enabled}
	if be := svc.SaveModel(platform, "bailian", "text-model", textModel); be != nil {
		t.Fatal(be.Msg)
	}
	binding := BindingReq{ProviderCode: "bailian", ModelID: "text-model", TimeoutSeconds: 60, MaxRetries: 1, Status: model.Enabled}
	if be := svc.SaveBinding(platform, "image_inspection", binding); be == nil {
		t.Fatal("vision task must reject a text-only model")
	}
	visionModel := ModelReq{DisplayName: "视觉模型", ContextWindow: 8192, MaxOutput: 2048, Capabilities: []string{"text", "vision"}, Status: model.Enabled}
	if be := svc.SaveModel(platform, "bailian", "vision-model", visionModel); be != nil {
		t.Fatal(be.Msg)
	}
	binding.ModelID = "vision-model"
	if be := svc.SaveBinding(platform, "image_inspection", binding); be != nil {
		t.Fatal(be.Msg)
	}
	tenant := Scope{Name: model.ScopeTenant, ID: "01a00000-0000-7000-8000-000000000001"}
	provider.Name = "租户百炼"
	if be := svc.SaveProvider(tenant, "", provider); be != nil {
		t.Fatal(be.Msg)
	}
	providers, be := svc.Providers(tenant)
	if be != nil || len(providers) != 1 || providers[0]["source"] != model.ScopeTenant || providers[0]["name"] != "租户百炼" {
		t.Fatalf("provider override was not applied: %#v, %v", providers, be)
	}
	models, be := svc.Models(tenant)
	if be != nil || len(models) != 2 {
		t.Fatalf("platform models must be inherited independently: %#v, %v", models, be)
	}
	if be := svc.SaveBinding(tenant, "image_inspection", binding); be != nil {
		t.Fatal(be.Msg)
	}
	provider.CredentialRef = "plaintext-not-an-env-ref"
	if be := svc.SaveProvider(tenant, "bailian", provider); be == nil {
		t.Fatal("provider must reject a non-environment credential reference")
	}
}
