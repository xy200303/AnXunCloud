package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
)

func TestUnifiedConfigResolveAndClearOverride(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SysConfig{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []model.SysConfig{
		{Scope: model.ScopePlatform, ScopeID: model.PlatformScopeID, Key: "site.theme_color", Value: "#000000", AllowTenantOverride: true},
		{Scope: model.ScopeTenant, ScopeID: "tenant-1", Key: "site.theme_color", Value: "#123456"},
		{Scope: model.ScopeTenant, ScopeID: "tenant-2", Key: "site.theme_color", Value: "#654321"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer rdb.Close()
	cfg := NewConfigService(db, rdb)
	if value, ok := cfg.Resolve("tenant-1", "site.theme_color"); !ok || value != "#123456" {
		t.Fatalf("租户覆盖应优先: %q %v", value, ok)
	}
	if value, ok := cfg.Resolve("tenant-3", "site.theme_color"); !ok || value != "#000000" {
		t.Fatalf("无覆盖应回落平台: %q %v", value, ok)
	}
	if value, ok := cfg.Resolve("tenant-2", "site.theme_color"); !ok || value != "#654321" {
		t.Fatalf("不得串用其他租户配置: %q %v", value, ok)
	}
	tenants := NewTenantService(db, nil, cfg)
	if be := tenants.SaveConfig("tenant-1", map[string]string{"site.theme_color": ""}); be != nil {
		t.Fatal(be.Msg)
	}
	if value, ok := cfg.Resolve("tenant-1", "site.theme_color"); !ok || value != "#000000" {
		t.Fatalf("清除覆盖后应恢复平台值: %q %v", value, ok)
	}
}
