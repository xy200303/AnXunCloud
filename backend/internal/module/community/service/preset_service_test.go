package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/community/model"
	"anxuncloud/internal/pkg/types"
)

func validPresetPayload() types.JSONMap {
	flows := make([]any, 0, 5)
	for _, code := range []string{"checkin_review", "maint_review", "workorder_review", "report_review", "acceptance_review"} {
		flows = append(flows, map[string]any{"flow_code": code, "steps": []any{map[string]any{"role_code": "quality_supervisor", "name": "审核", "mode": "any"}}})
	}
	return types.JSONMap{
		"point_types": []any{map[string]any{"type_code": "point_type", "label": "测试点位", "value": "preset_test_point", "status": 1}},
		"templates":   []any{}, "equipment_schemas": []any{}, "flows": flows, "doc_templates": []any{},
	}
}

func TestPresetPackSaveAndPublication(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:preset_pack_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CommunityPresetPack{}); err != nil {
		t.Fatal(err)
	}
	svc := NewCommunityService(db)
	req := PresetPackReq{Name: "测试预置包", Description: "平台维护", Payload: validPresetPayload(), Status: "disabled"}
	if be := svc.SavePresetPack("test_preset", req); be != nil {
		t.Fatal(be.Msg)
	}
	published, be := svc.PresetPacks()
	if be != nil || len(published) != 0 {
		t.Fatalf("停用预置包不应对租户发布: %#v, %v", published, be)
	}
	if _, be := svc.PresetPack("test_preset"); be == nil {
		t.Fatal("租户不应读取停用预置包")
	}
	req.Status = "enabled"
	req.Name = "测试预置包（更新）"
	if be := svc.SavePresetPack("test_preset", req); be != nil {
		t.Fatal(be.Msg)
	}
	published, be = svc.PresetPacks()
	if be != nil || len(published) != 1 || published[0].Name != req.Name {
		t.Fatalf("更新发布失败: %#v, %v", published, be)
	}
	invalid := req
	invalid.Payload = types.JSONMap{"point_types": []any{}, "templates": []any{}, "equipment_schemas": []any{}, "flows": []any{}, "doc_templates": []any{}}
	if be := svc.SavePresetPack("invalid_preset", invalid); be == nil {
		t.Fatal("缺少五类流程的预置包不应保存")
	}
	var count int64
	if err := db.Model(&model.CommunityPresetPack{}).Where("code = ?", "invalid_preset").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("无效预置包被写入: count=%d err=%v", count, err)
	}
}
