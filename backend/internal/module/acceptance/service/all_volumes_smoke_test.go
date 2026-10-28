package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/pkg/libreoffice"
	"anxuncloud/internal/pkg/ooxml"
	"anxuncloud/internal/pkg/types"
)

func TestAcceptanceAllVolumesRealPDFSmoke(t *testing.T) {
	dir := os.Getenv("ACCEPTANCE_RENDER_DIR")
	if dir == "" {
		t.Skip("未指定真实 LibreOffice 输出目录")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		volume, tableCode, itemCode, itemName string
	}{
		{"acceptance_documents", "documents_project", "land_certificate", "国有土地使用证"},
		{"acceptance_building", "building_1_exterior", "wall_finish", "外墙饰面"},
		{"acceptance_outdoor", "outdoor_recreation", "full_court", "全场篮球场"},
		{"acceptance_facilities", "facilities_fire_control", "environment", "环境"},
		{"acceptance_garage", "garage_camera", "camera_count", "摄像头数量"},
		{"acceptance_premises", "premises_assets", "desk", "办公桌"},
	}
	converter, err := libreoffice.NewWorkerClient(dir, 120*time.Second, libreoffice.DefaultMaxInputBytes, libreoffice.DefaultMaxOutputBytes)
	if err != nil {
		t.Fatal(err)
	}
	svc := &BatchService{}
	for _, tc := range cases {
		raw, _, _ := ooxml.BuiltinTemplate(tc.volume)
		mother, err := ooxml.OpenTemplate(raw)
		if err != nil {
			t.Fatal(err)
		}
		record := model.Record{TargetID: "target-1", TableCode: tc.tableCode, ItemCode: tc.itemCode, Result: "problem", Note: "真实烟测问题", ValueText: "2"}
		names := map[string]string{tc.tableCode + "\x00" + tc.itemCode: tc.itemName}
		if err := svc.fillAcceptanceRecord(mother, acceptanceTableSlots[tc.tableCode], record, names); err != nil {
			t.Fatal(err)
		}
		if err := svc.fillAcceptanceProblems(mother, acceptanceTableSlots[tc.tableCode], []model.Record{record}, map[string]model.Target{"target-1": {Name: "烟测对象", Location: "1栋"}}, names); err != nil {
			t.Fatal(err)
		}
		assertAcceptancePDFConversion(t, converter, mother, dir, tc.volume)
	}
	raw, _, _ := ooxml.BuiltinTemplate("acceptance_unit")
	mother, err := ooxml.OpenTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	buildingID := "building-1"
	units := []model.Target{{TargetType: "unit", CategoryCode: "unit", Name: "1栋101", Location: "1栋", BuildingID: &buildingID}, {TargetType: "unit", CategoryCode: "unit", Name: "1栋102", Location: "1栋", BuildingID: &buildingID}}
	units[0].ID, units[1].ID = "home-1", "home-2"
	records := []model.Record{{TargetID: "home-1", CategoryCode: "unit", TableCode: "unit_door", ItemCode: "frame", Result: "problem", Note: "门框破损", Values: types.JSONMap{"water_meter_number": "W-101", "water_reading": "12.5", "electric_meter_number": "E-101", "electric_reading": "108.2"}}}
	names := map[string]string{"unit_door\x00frame": "门框密实"}
	if err := svc.fillAcceptanceUnitReport(mother, units, records, names); err != nil {
		t.Fatal(err)
	}
	assertAcceptancePDFConversion(t, converter, mother, dir, "acceptance_unit")
}

func assertAcceptancePDFConversion(t *testing.T, converter libreoffice.Converter, mother *ooxml.Template, dir, volume string) {
	t.Helper()
	docx, err := mother.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, volume+".docx"), docx, 0o600); err != nil {
		t.Fatal(err)
	}
	pdf, err := converter.Convert(t.Context(), docx)
	if err != nil {
		t.Fatalf("%s 真实 PDF 转换失败: %v", volume, err)
	}
	if err := libreoffice.ValidatePDF(pdf, libreoffice.DefaultMaxOutputBytes); err != nil {
		t.Fatalf("%s PDF 校验失败: %v", volume, err)
	}
	if err := os.WriteFile(filepath.Join(dir, volume+".pdf"), pdf, 0o600); err != nil {
		t.Fatal(err)
	}
}
