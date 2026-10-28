package service

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/acceptance/model"
	sysmodel "anxuncloud/internal/module/system/model"
)

func TestDecideReopenRequiresDifferentNaturalPerson(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Batch{}, &model.ReopenRequest{}, &model.Signature{}, &model.HandoverLog{}, &sysmodel.Community{}); err != nil {
		t.Fatal(err)
	}
	community := sysmodel.Community{TenantID: "tenant-1", Name: "测试小区"}
	community.ID = "community-1"
	if err := db.Create(&community).Error; err != nil {
		t.Fatal(err)
	}
	locked := time.Now()
	batch := model.Batch{TenantID: "tenant-1", CommunityID: "community-1", ManagerID: "applicant", Status: model.StatusDone, Revision: 7, LockedAt: &locked}
	batch.ID = "batch-1"
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	request := model.ReopenRequest{TenantID: batch.TenantID, BatchID: batch.ID, RequesterID: "applicant", Reason: "补充房屋事实", Status: "pending"}
	if err := db.Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	signature := model.Signature{TenantID: batch.TenantID, BatchID: batch.ID, Revision: 7, SubjectCode: "inspector", Status: "valid"}
	if err := db.Create(&signature).Error; err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(nil)
	middleware.SetIdentity(c, &middleware.Identity{UserID: "applicant", TenantID: batch.TenantID, DataScopeAll: true})
	svc := &BatchService{db: db}
	if be := svc.DecideReopen(c, batch.ID, request.ID, "approve", ""); be == nil {
		t.Fatal("原申请人不得批准自己的重开申请")
	}
	middleware.SetIdentity(c, &middleware.Identity{UserID: "approver", TenantID: batch.TenantID, DataScopeAll: true})
	if be := svc.DecideReopen(c, batch.ID, request.ID, "approve", "同意补充"); be != nil {
		t.Fatal(be.Msg)
	}
	var saved model.Batch
	if err := db.First(&saved, "id = ?", batch.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.StatusInspecting || saved.LockedAt != nil || saved.Revision != 8 {
		t.Fatalf("重开状态错误: %+v", saved)
	}
	if err := db.First(&signature, "id = ?", signature.ID).Error; err != nil {
		t.Fatal(err)
	}
	if signature.Status != "invalid" {
		t.Fatalf("重开后原签署必须失效: %+v", signature)
	}
}
