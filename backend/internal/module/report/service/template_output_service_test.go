package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	inscontract "anxuncloud/internal/module/inspection/contract"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/report/model"
)

func TestTemplateOutputBindingAppliesColumnsAndRowMode(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.DocTemplate{}, &model.TemplateOutput{}, &model.StylePack{}, &insmodel.CheckTemplate{}, &insmodel.CheckTemplateItem{}); err != nil {
		t.Fatal(err)
	}
	tenant := "tenant"
	doc := model.DocTemplate{TenantID: &tenant, Code: "monthly", Name: "月报", DocKind: "monthly_report", Engine: "ooxml", Format: "docx", VolumeCode: "fire", Status: "enabled"}
	template := insmodel.CheckTemplate{TenantID: &tenant, Code: "fire", Name: "消防", PointType: "fire", Purpose: insmodel.TemplatePurposePatrol, Status: insmodel.TemplateStatusEnabled}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	db.Create(&insmodel.CheckTemplateItem{TemplateID: template.ID, Code: "first", Name: "第一项", ValueType: insmodel.ValueOK})
	db.Create(&insmodel.CheckTemplateItem{TemplateID: template.ID, Code: "second", Name: "第二项", ValueType: insmodel.ValueOK})
	svc := NewTemplateOutputService(db)
	scope := DocTemplateScope{TenantID: tenant}
	if be := svc.Save(scope, doc.ID, []TemplateOutputBindingReq{{
		TemplateID: template.ID, RowMode: "equipment", Columns: []OutputColumn{{ItemCode: "second"}},
	}}); be != nil {
		t.Fatal(be.Msg)
	}
	report := &ReportService{db: db, docTemplates: NewDocTemplateService(db)}
	columns := outputTemplateColumns(report, nil, &doc, nil)
	if len(columns.Columns) != 2 || columns.Columns[0].itemCode != "second" || columns.Columns[1].itemCode != "first" {
		t.Fatalf("事实项顺序错误: %+v", columns)
	}
	if mode := outputRowMode(report, &doc, "fire"); mode != "equipment" {
		t.Fatalf("行模式错误: %s", mode)
	}
	other := insmodel.CheckTemplate{TenantID: &tenant, Code: "other", Name: "其他", PointType: "other", Purpose: insmodel.TemplatePurposePatrol, Status: insmodel.TemplateStatusEnabled}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&insmodel.CheckTemplateItem{TemplateID: other.ID, Code: "other_item", Name: "其他项", ValueType: insmodel.ValueOK}).Error; err != nil {
		t.Fatal(err)
	}
	if be := svc.Save(scope, doc.ID, []TemplateOutputBindingReq{{TemplateID: template.ID, RowMode: "equipment"}, {TemplateID: other.ID, RowMode: "point"}}); be != nil {
		t.Fatal(be.Msg)
	}
	otherColumns := outputTemplateColumns(report, []*inscontract.PointFact{{Type: "other"}}, &doc, nil)
	if len(otherColumns.Columns) != 1 || otherColumns.Columns[0].itemCode != "other_item" {
		t.Fatalf("绑定未按 point_type 选择: %+v", otherColumns.Columns)
	}
	if be := svc.Save(scope, doc.ID, []TemplateOutputBindingReq{{TemplateID: template.ID, RowMode: "point", Columns: []OutputColumn{{ItemCode: "missing"}}}}); be == nil {
		t.Fatal("不存在的 item_code 未被拒绝")
	}
}
