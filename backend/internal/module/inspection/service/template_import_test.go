package service

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
)

func TestTemplateImportPreviewApplyAndConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CheckTemplate{}, &model.CheckTemplateItem{}, &sysmodel.SysDictData{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&sysmodel.SysDictData{TypeCode: "point_type", Value: "fire_cabinet", Label: "消火栓箱", Status: sysmodel.StatusEnabled})
	svc := NewTemplateService(db)
	tenantID := "tenant"
	rows := []dto.TemplateImportRow{
		{Row: 2, PointType: "fire_cabinet", TemplateName: "消防模板", Purpose: "patrol", ItemCode: "visual_check", ItemName: "外观", ValueType: "ok", PhotoRequired: "必拍", AIPromptKind: "damage"},
		{Row: 3, PointType: "fire_cabinet", TemplateName: "消防模板", Purpose: "patrol", ItemCode: "visual_check", ItemName: "重复编码", ValueType: "ok"},
		{Row: 4, PointType: "fire_cabinet", TemplateName: "消防模板", Purpose: "patrol", ItemCode: "pressure", ItemName: "压力", ValueType: "number"},
	}
	preview, be := svc.previewImport(tenantID, rows)
	if be != nil {
		t.Fatal(be.Msg)
	}
	if !preview[0].Valid || preview[1].Valid || preview[2].Valid {
		t.Fatalf("预览结果错误: %+v", preview)
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("pi_identity", &middleware.Identity{TenantID: tenantID})
	result, be := svc.ApplyImport(ctx, rows)
	if be != nil {
		t.Fatal(be.Msg)
	}
	if result["created_templates"] != 1 || result["created_items"] != 1 || len(result["failed"].([]importResultRow)) != 2 {
		t.Fatalf("部分落库结果错误: %#v", result)
	}
	var template model.CheckTemplate
	if err := db.First(&template, "id = ?", result["template_id"]).Error; err != nil || template.Status != model.TemplateStatusDraft {
		t.Fatalf("新模板应为草稿: %+v, %v", template, err)
	}
	preview, be = svc.previewImport(tenantID, rows[:1])
	if be != nil || preview[0].Valid {
		t.Fatalf("已有编码应拒绝: %+v, %v", preview, be)
	}
	db.Model(&model.CheckTemplateItem{}).Where("code = ?", "visual_check").Update("retired_at", "2026-01-01")
	preview, _ = svc.previewImport(tenantID, rows[:1])
	if preview[0].Valid {
		t.Fatal("退役编码仍被允许复用")
	}
}

func TestTemplateImportWorkbookRoundTrip(t *testing.T) {
	book, err := TemplateImportWorkbook(true)
	if err != nil {
		t.Fatal(err)
	}
	values := []string{"fire_cabinet", "消防模板", "patrol", "visual_check", "外观", "ok", "完好", "必拍", "", "", "", "", "否", "damage", "否", "水带缺失", "拍全景", "1", "箱体"}
	sheet := book.GetSheetName(0)
	for i, value := range values {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		book.SetCellValue(sheet, cell, value)
	}
	var data bytes.Buffer
	if err := book.Write(&data); err != nil {
		t.Fatal(err)
	}
	rows, be := parseTemplateImport(bytes.NewReader(data.Bytes()), ".xlsx")
	if be != nil {
		t.Fatal(be.Msg)
	}
	if len(rows) != 1 || rows[0].ItemCode != "visual_check" || rows[0].Row != 3 {
		t.Fatalf("Excel 解析错误: %+v", rows)
	}
}
