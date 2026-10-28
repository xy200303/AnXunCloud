package service

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/module/acceptance/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ooxml"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/types"
)

func TestAcceptanceBusinessSlotsAcrossOriginalMothers(t *testing.T) {
	cases := []struct {
		volume, tableCode, itemCode, itemName string
		wantTable, wantRow, wantColumn        int
	}{
		{"acceptance_documents", "documents_project", "land_certificate", "国有土地使用证", 4, 2, 3},
		{"acceptance_building", "building_1_exterior", "wall_finish", "外墙饰面", 4, 7, 1},
		{"acceptance_outdoor", "outdoor_recreation", "full_court", "全场篮球场", 8, 1, 5},
		{"acceptance_facilities", "facilities_fire_control", "environment", "环境", 12, 2, 1},
		{"acceptance_garage", "garage_camera", "camera_count", "摄像头数量", 11, 3, 2},
		{"acceptance_premises", "premises_assets", "desk", "办公桌", 8, 1, 6},
	}
	for _, tc := range cases {
		t.Run(tc.volume, func(t *testing.T) {
			raw, _, ok := ooxml.BuiltinTemplate(tc.volume)
			if !ok {
				t.Fatal("母版不存在")
			}
			mother, err := ooxml.OpenTemplate(raw)
			if err != nil {
				t.Fatal(err)
			}
			svc := &BatchService{}
			record := model.Record{TableCode: tc.tableCode, ItemCode: tc.itemCode, Result: "problem", Note: "现场问题", ValueText: "2"}
			err = svc.fillAcceptanceRecord(mother, acceptanceTableSlots[tc.tableCode], record, map[string]string{tc.tableCode + "\x00" + tc.itemCode: tc.itemName})
			if err != nil {
				t.Fatal(err)
			}
			if tc.tableCode == "documents_project" || tc.tableCode == "outdoor_recreation" || tc.tableCode == "premises_assets" {
				got, err := mother.CellText(tc.wantTable, tc.wantRow, tc.wantColumn)
				if err != nil || got != "现场问题" {
					t.Fatalf("清单事实写入错误: got=%q err=%v", got, err)
				}
			} else {
				got, err := mother.CellText(tc.wantTable, tc.wantRow, tc.wantColumn)
				if err != nil || got != "2" {
					t.Fatalf("矩阵实测值写入错误: got=%q err=%v", got, err)
				}
			}
		})
	}
}

func TestAcceptanceProblemRowsAndAllPhotos(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&sysmodel.UploadFile{}); err != nil {
		t.Fatal(err)
	}
	store := storage.New(config.UploadConfig{Mode: "local", LocalDir: t.TempDir()}, config.OSSConfig{}, config.COSConfig{}, "")
	var pixel bytes.Buffer
	if err := png.Encode(&pixel, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	photoIDs := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		key, _, err := store.SaveGenerated("test", "photo.png", pixel.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		file := sysmodel.UploadFile{StorageKey: key, MimeType: "image/png"}
		if err := db.Create(&file).Error; err != nil {
			t.Fatal(err)
		}
		photoIDs = append(photoIDs, file.ID)
	}
	target := model.Target{Name: "中心球场", Location: "东区"}
	targetID := target.ID
	records := []model.Record{
		{TargetID: targetID, TableCode: "outdoor_recreation", ItemCode: "full_court", Result: "problem", Note: "地面开裂", Photos: types.StringArray(photoIDs[:2])},
		{TargetID: targetID, TableCode: "outdoor_recreation", ItemCode: "full_court", Result: "problem", Note: "篮架松动", Photos: types.StringArray(photoIDs[2:])},
	}
	raw, _, _ := ooxml.BuiltinTemplate("acceptance_outdoor")
	mother, err := ooxml.OpenTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	svc := &BatchService{db: db, store: store}
	names := map[string]string{"outdoor_recreation\x00full_court": "全场篮球场"}
	if err := svc.fillAcceptanceProblems(mother, acceptanceTableSlots["outdoor_recreation"], records, map[string]model.Target{targetID: target}, names); err != nil {
		t.Fatal(err)
	}
	rows, err := mother.TableRowCount(9)
	if err != nil || rows != 3 {
		t.Fatalf("两个独立问题必须动态生成两行: rows=%d err=%v", rows, err)
	}
	for row, want := range map[int]string{1: "地面开裂", 2: "篮架松动"} {
		got, err := mother.CellText(9, row, 2)
		if err != nil || got != want {
			t.Fatalf("问题行 %d 内容错误: %q %v", row, got, err)
		}
	}
	built, err := mother.Build()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(built), int64(len(built)))
	if err != nil {
		t.Fatal(err)
	}
	images := 0
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "word/media/mother-image-") {
			images++
		}
	}
	if images != 4 {
		t.Fatalf("四个真实照片必须全部内嵌: %d", images)
	}
}

func TestAcceptanceUnitReportUsesOneTablePerHome(t *testing.T) {
	buildingID := "building-1"
	units := []model.Target{
		{TargetType: "unit", CategoryCode: "unit", Name: "1栋101", Location: "1栋", BuildingID: &buildingID},
		{TargetType: "unit", CategoryCode: "unit", Name: "1栋102", Location: "1栋", BuildingID: &buildingID},
	}
	units[0].ID, units[1].ID = "home-1", "home-2"
	records := []model.Record{
		{TargetID: "home-1", CategoryCode: "unit", TableCode: "unit_door", ItemCode: "frame", Result: "problem", Note: "门框破损", Values: types.JSONMap{"water_meter_number": "W-101", "water_reading": "12.5", "electric_meter_number": "E-101", "electric_reading": "108.2"}},
		{TargetID: "home-2", CategoryCode: "unit", TableCode: "unit_building", ItemCode: "wall", Result: "problem", Note: "墙面空鼓"},
	}
	names := map[string]string{"unit_door\x00frame": "门框密实", "unit_building\x00wall": "墙面平整"}
	raw, _, _ := ooxml.BuiltinTemplate("acceptance_unit")
	mother, err := ooxml.OpenTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	svc := &BatchService{}
	if err := svc.fillAcceptanceUnitReport(mother, units, records, names); err != nil {
		t.Fatal(err)
	}
	for table, room := range map[int]string{5: "1栋101", 7: "1栋102"} {
		got, err := mother.CellText(table, 0, 1)
		if err != nil || got != room {
			t.Fatalf("一户一档房号错误: table=%d got=%q err=%v", table, got, err)
		}
	}
	if got, err := mother.CellText(4, 1, 2); err != nil || got != "1" {
		t.Fatalf("楼栋入户门问题汇总错误: %q %v", got, err)
	}
	if got, err := mother.CellText(14, 1, 3); err != nil || got != "12.5" {
		t.Fatalf("水表真实底数错误: %q %v", got, err)
	}
	if got, err := mother.CellText(15, 1, 3); err != nil || got != "108.2" {
		t.Fatalf("电表真实底数错误: %q %v", got, err)
	}
	built, err := mother.Build()
	if err != nil {
		t.Fatal(err)
	}
	validated, err := ooxml.OpenTemplate(built)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := validated.TableRowCount(14)
	if err != nil || rows != 3 {
		t.Fatalf("两户水表底数应保留两行: rows=%d err=%v", rows, err)
	}
}
