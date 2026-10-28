package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/acceptance/model"
)

func TestTransitionTxCommitsSuccessfulUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Batch{}); err != nil {
		t.Fatal(err)
	}
	batch := model.Batch{TenantID: "01a00000-0000-7000-8000-000000000001", CommunityID: "01a00000-0000-7000-8000-000000000002", Name: "事务测试", LedgerType: "institution_change", ManagerID: "01a00000-0000-7000-8000-000000000003", PeriodStart: time.Now(), PeriodEnd: time.Now().Add(time.Hour), Status: model.StatusReviewing, Revision: 1}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	svc := &BatchService{db: db}
	if be := acceptanceTransaction(db, func(tx *gorm.DB) error {
		return svc.transitionTx(tx, &batch, model.StatusHandover, 1)
	}); be != nil {
		t.Fatal(be.Msg)
	}
	var saved model.Batch
	if err := db.First(&saved, "id = ?", batch.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.StatusHandover || saved.Revision != 2 || saved.ReviewStep != 1 {
		t.Fatalf("transaction was not committed: %#v", saved)
	}
}
