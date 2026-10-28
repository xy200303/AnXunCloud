package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/inspection/model"
)

func TestReadApprovedFactsUsesTenantScopeAndApprovedOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.InspectionPoint{}, &model.InspectionTask{}, &model.CheckinRecord{}, &model.CheckinRecordItem{}); err != nil {
		t.Fatal(err)
	}
	tenantID := "01a00000-0000-7000-8000-000000000001"
	otherTenant := "01a00000-0000-7000-8000-000000000099"
	communityID := "01a00000-0000-7000-8000-000000000002"
	point := model.InspectionPoint{TenantID: &tenantID, CommunityID: communityID, Name: "灭火器", Type: "fire_extinguisher"}
	if err := db.Create(&point).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, row := range []model.CheckinRecord{
		{TenantID: &tenantID, PointID: point.ID, CommunityID: communityID, InspectorID: "inspector", CheckinTime: now, AuditStatus: model.AuditPass},
		{TenantID: &tenantID, PointID: point.ID, CommunityID: communityID, InspectorID: "inspector", CheckinTime: now.Add(time.Second), AuditStatus: model.AuditPending},
		{TenantID: &otherTenant, PointID: point.ID, CommunityID: communityID, InspectorID: "other", CheckinTime: now.Add(2 * time.Second), AuditStatus: model.AuditPass},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	var approved model.CheckinRecord
	if err := db.First(&approved, "audit_status = ? AND tenant_id = ?", model.AuditPass, tenantID).Error; err != nil {
		t.Fatal(err)
	}
	item := model.CheckinRecordItem{TenantID: &tenantID, RecordID: approved.ID, ItemCode: "pressure", Name: "压力", Result: model.ItemResultNormal}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	var pages []contract.ApprovedFactPage
	err = ReadApprovedFacts(db, contract.ApprovedFactQuery{TenantID: tenantID, CommunityID: communityID, Start: now.Add(-time.Minute), End: now.Add(time.Minute)}, func(page contract.ApprovedFactPage) error {
		pages = append(pages, page)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var records []contract.RecordFact
	var items []contract.ItemFact
	for _, page := range pages {
		records = append(records, page.Records...)
		items = append(items, page.Items...)
	}
	if len(records) != 1 || records[0].ID != approved.ID {
		t.Fatalf("只应返回本租户已审核记录: %+v", records)
	}
	if len(items) != 1 || items[0].ItemCode != "pressure" {
		t.Fatalf("已审核项事实读取错误: %+v", items)
	}
}
