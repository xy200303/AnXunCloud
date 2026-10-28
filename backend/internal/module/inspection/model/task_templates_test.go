package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

func TestLoadTaskTemplateSetsUsesAuthorizationTypePurposeAndRetirement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&CheckTemplate{}, &CheckTemplateItem{}, &InspectionPoint{}, &PointTemplate{}, &InspectionPlan{}, &InspectionTask{}); err != nil {
		t.Fatal(err)
	}
	tenant := "tenant"
	fire := CheckTemplate{TenantID: &tenant, Code: "fire", Name: "消防", PointType: "fire_cabinet", Purpose: TemplatePurposePatrol, Status: TemplateStatusEnabled}
	pump := CheckTemplate{TenantID: &tenant, Code: "pump", Name: "水泵", PointType: "pump_room", Purpose: TemplatePurposePatrol, Status: TemplateStatusEnabled}
	spot := CheckTemplate{TenantID: &tenant, Code: "spot", Name: "抽查", PointType: "fire_cabinet", Purpose: TemplatePurposeSpot, Status: TemplateStatusEnabled}
	db.Create(&fire).Create(&pump).Create(&spot)
	db.Create(&CheckTemplateItem{TemplateID: fire.ID, Code: "active", Name: "有效项", ValueType: ValueOK, Sort: 1})
	db.Create(&CheckTemplateItem{TemplateID: fire.ID, Code: "retired", Name: "退役项", ValueType: ValueOK, Sort: 2})
	db.Model(&CheckTemplateItem{}).Where("code = ?", "retired").Update("retired_at", "2026-01-01")
	db.Create(&CheckTemplateItem{TemplateID: pump.ID, Code: "pump_item", Name: "水泵项", ValueType: ValueOK})
	db.Create(&CheckTemplateItem{TemplateID: spot.ID, Code: "spot_item", Name: "抽查项", ValueType: ValueOK})
	db.Create(&InspectionPoint{UUIDModel: types.UUIDModel{ID: "fire-point"}, TenantID: &tenant, Type: "fire_cabinet"})
	db.Create(&InspectionPoint{UUIDModel: types.UUIDModel{ID: "pump-point"}, TenantID: &tenant, Type: "pump_room"})
	plan := InspectionPlan{PlanKind: PlanKindPatrol}
	db.Create(&plan)
	task := InspectionTask{TenantID: &tenant, PlanID: plan.ID, TemplateIDs: types.IDArray{fire.ID, pump.ID, spot.ID}}
	sets := LoadTaskTemplateSets(db, &task, []string{"fire-point", "pump-point"})
	if len(sets["fire-point"].Items) != 1 || sets["fire-point"].Items[0].Code != "active" {
		t.Fatalf("消防点位应只有未退役巡检项: %+v", sets["fire-point"].Items)
	}
	if len(sets["pump-point"].Items) != 1 || sets["pump-point"].Items[0].TemplateID != pump.ID {
		t.Fatalf("水泵点位模板匹配错误: %+v", sets["pump-point"].Items)
	}
	plan.PlanKind = PlanKindSpotcheck
	db.Save(&plan)
	sets = LoadTaskTemplateSets(db, &task, []string{"fire-point"})
	if len(sets["fire-point"].Items) != 1 || sets["fire-point"].Items[0].TemplateID != spot.ID {
		t.Fatalf("抽查计划只能展开 spot 模板: %+v", sets["fire-point"].Items)
	}
}
