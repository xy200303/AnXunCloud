package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
)

func TestTenantOverrideRulesComeFromPlatformConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SysConfig{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []model.SysConfig{
		{Scope: model.ScopePlatform, ScopeID: model.PlatformScopeID, Key: "site.theme_color", AllowTenantOverride: true},
		{Scope: model.ScopePlatform, ScopeID: model.PlatformScopeID, Key: "site.company_name", AllowTenantOverride: true},
		{Scope: model.ScopePlatform, ScopeID: model.PlatformScopeID, Key: "ai.api_key", AllowTenantOverride: true, IsSecret: true},
		{Scope: model.ScopePlatform, ScopeID: model.PlatformScopeID, Key: "auth.register_enabled"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := NewTenantService(db, nil, nil)
	for _, key := range []string{"site.theme_color", "site.company_name"} {
		if !svc.tenantOverrideAllowed(key) {
			t.Errorf("平台允许覆盖的 key 应放行: %s", key)
		}
	}
	for _, key := range []string{"ai.api_key", "auth.register_enabled", "site.unknown", ""} {
		if svc.tenantOverrideAllowed(key) {
			t.Errorf("密钥或未授权 key 应拒绝: %s", key)
		}
	}
	if be := svc.SaveConfig("tenant-1", map[string]string{"ai.api_key": "must-not-save"}); be == nil {
		t.Fatal("密钥不得写入租户配置")
	}
}
