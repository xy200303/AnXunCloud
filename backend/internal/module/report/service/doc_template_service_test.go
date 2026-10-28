package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/report/model"
	"anxuncloud/internal/pkg/types"
)

func TestValidateLayoutReadsSnakeCaseSignSlots(t *testing.T) {
	layout := types.JSONMap{
		"blocks": []any{map[string]any{"type": "cover"}, map[string]any{"type": "sign"}},
		"sign_slots": []any{
			map[string]any{"code": "quality", "label": "品质审核", "step_index": 0, "required": true},
			map[string]any{"code": "manager", "label": "项目审核", "step_index": 1, "required": true},
		},
	}
	if be := validateLayout(layout); be != nil {
		t.Fatalf("合法签字位不应被拒绝: %s", be.Msg)
	}
	layout["sign_slots"] = []any{
		map[string]any{"code": "quality", "label": "品质审核", "step_index": 0},
		map[string]any{"code": "quality", "label": "重复签字位", "step_index": 1},
	}
	if be := validateLayout(layout); be == nil {
		t.Fatal("重复签字位编码不应通过")
	}
}

func TestDocTemplateListIncludesIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.DocTemplate{}); err != nil {
		t.Fatal(err)
	}
	tenant := "tenant"
	row := model.DocTemplate{TenantID: &tenant, Code: "monthly", Name: "月报", DocKind: "monthly_report", Engine: "ooxml", Format: "docx", VolumeCode: "fire", Status: "enabled"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewDocTemplateService(db)
	items, be := svc.List(DocTemplateScope{TenantID: tenant})
	if be != nil || len(items) != 1 || items[0]["id"] != row.ID {
		t.Fatalf("输出模板列表缺少真实 ID: %#v, %v", items, be)
	}
	if be := svc.Save(DocTemplateScope{TenantID: tenant}, row.Code, DocTemplateReq{Name: "更新月报", DocKind: row.DocKind, Engine: row.Engine, Format: row.Format, VolumeCode: row.VolumeCode, Status: row.Status, SkeletonFileID: skeletonRef(), Layout: types.JSONMap{"mother": map[string]any{"profile": "fire_equipment"}}}); be != nil {
		t.Fatal(be.Msg)
	}
	var updated model.DocTemplate
	if err := db.First(&updated, "id = ?", row.ID).Error; err != nil || updated.Name != "更新月报" {
		t.Fatalf("同编码更新失败: %+v, %v", updated, err)
	}
	var count int64
	if err := db.Model(&model.DocTemplate{}).Where("tenant_id = ? AND code = ?", tenant, row.Code).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("同编码更新产生重复行: %d, %v", count, err)
	}
	platform := model.DocTemplate{Code: row.Code, Name: "平台月报", DocKind: row.DocKind, Engine: row.Engine, Format: row.Format, VolumeCode: row.VolumeCode, Status: row.Status}
	if err := db.Create(&platform).Error; err != nil {
		t.Fatal(err)
	}
	effective, err := svc.EffectiveMonthly(tenant)
	if err != nil || len(effective) != 1 || effective[0].ID != row.ID || effective[0].TenantID == nil || effective[0].Name != "更新月报" {
		t.Fatalf("租户输出模板未覆盖平台默认: %+v, %v", effective, err)
	}
}
