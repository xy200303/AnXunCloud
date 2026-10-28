package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	filemodel "anxuncloud/internal/module/file/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/storage"
)

func TestSaveArchivedPDFIsImmutableAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.InspectionReport{}, &model.ReportFile{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}, &sysmodel.Community{}, &sysmodel.Tenant{}, &sysmodel.SysDictData{}, &insmodel.InspectionPoint{}, &insmodel.InspectionTask{}, &insmodel.CheckinRecord{}, &insmodel.CheckinRecordItem{}); err != nil {
		t.Fatal(err)
	}
	store := storage.New(config.UploadConfig{Mode: "local", LocalDir: t.TempDir()}, config.OSSConfig{}, config.COSConfig{}, "")
	svc := &ReportService{db: db, store: store}
	tenantID := "01a00000-0000-7000-8000-000000000001"
	communityID := "01a00000-0000-7000-8000-000000000002"
	report := model.InspectionReport{TenantID: &tenantID, CommunityID: communityID, Period: "2026-10", Title: "归档幂等测试", Status: model.StatusApproved}
	if err := db.Create(&report).Error; err != nil {
		t.Fatal(err)
	}
	skeletonID := "93000000-0000-0000-0000-000000000001"
	volume := model.DocTemplate{DocKind: "monthly_report", VolumeCode: "fire_equipment", Name: "测试分册", Format: "docx", SystemFilter: "test_system", SkeletonFileID: &skeletonID}
	if err := svc.generateVolume(&report, volume, model.FileStageArchived, map[string]string{}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.generateVolume(&report, volume, model.FileStageArchived, map[string]string{}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.ReportFile{}).Where("report_id = ? AND stage = ?", report.ID, model.FileStageArchived).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one archive, got %d", count)
	}
	file, upload, err := svc.archivedFile(report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upload.TenantID == nil || *upload.TenantID != tenantID {
		t.Fatalf("unexpected file tenant: %v", upload.TenantID)
	}
	data, err := store.ReadFile(upload.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || string(data[:2]) != "PK" {
		t.Fatalf("archive is not a valid DOCX: %q", data)
	}
	if file.SHA256 == "" || file.GenerationBatch == "" {
		t.Fatal("archive metadata is incomplete")
	}
}
