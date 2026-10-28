package service

import (
	"context"
	"fmt"
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

type stubPDFConverter struct {
	data []byte
	err  error
}

func (c stubPDFConverter) Convert(context.Context, []byte) ([]byte, error) {
	return c.data, c.err
}

func skeletonRef() *string {
	id := "93000000-0000-0000-0000-000000000001"
	return &id
}

func TestGenerateVolumePDFRegistersOnlyValidatedOutput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.InspectionReport{}, &model.ReportFile{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}, &sysmodel.Community{}, &sysmodel.Tenant{}, &sysmodel.SysDictData{}, &insmodel.InspectionPoint{}, &insmodel.InspectionTask{}, &insmodel.CheckinRecord{}, &insmodel.CheckinRecordItem{}); err != nil {
		t.Fatal(err)
	}
	store := storage.New(config.UploadConfig{Mode: "local", LocalDir: t.TempDir()}, config.OSSConfig{}, config.COSConfig{}, "")
	tenantID := "01a00000-0000-7000-8000-000000000001"
	report := model.InspectionReport{TenantID: &tenantID, CommunityID: "01a00000-0000-7000-8000-000000000002", Period: "2026-10", Title: "PDF 测试", Status: model.StatusApproved}
	if err := db.Create(&report).Error; err != nil {
		t.Fatal(err)
	}
	volume := model.DocTemplate{DocKind: "monthly_report", VolumeCode: "fire_equipment", Name: "PDF 分册", Format: "pdf", SystemFilter: "test_system", SkeletonFileID: skeletonRef()}
	svc := &ReportService{db: db, store: store, pdfConverter: stubPDFConverter{data: []byte("%PDF-1.7\ncontrolled\n%%EOF\n")}}
	if err := svc.generateVolume(&report, volume, model.FileStageFrozen, map[string]string{}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.generateVolume(&report, volume, model.FileStageFrozen, map[string]string{}, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	var file model.ReportFile
	if err := db.First(&file, "report_id = ? AND stage = ?", report.ID, model.FileStageFrozen).Error; err != nil {
		t.Fatal(err)
	}
	if file.Status != model.FileStatusReady || file.Format != "pdf" || file.FileID == nil {
		t.Fatalf("PDF 未正确登记: %+v", file)
	}
	data, err := store.ReadFile(func() string {
		var upload sysmodel.UploadFile
		if err := db.First(&upload, "id = ?", *file.FileID).Error; err != nil {
			t.Fatal(err)
		}
		if upload.MimeType != "application/pdf" {
			t.Fatalf("PDF MIME 错误: %s", upload.MimeType)
		}
		return upload.StorageKey
	}())
	if err != nil || len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatalf("PDF 文件内容错误: %v, %q", err, data)
	}
	var count int64
	if err := db.Model(&model.ReportFile{}).Where("report_id = ? AND stage = ?", report.ID, model.FileStageFrozen).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("PDF 幂等登记失败: %d, %v", count, err)
	}
}

func TestGenerateVolumePDFFailureIsNotReady(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.InspectionReport{}, &model.ReportFile{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}, &sysmodel.Community{}, &sysmodel.Tenant{}, &sysmodel.SysDictData{}, &insmodel.InspectionPoint{}, &insmodel.InspectionTask{}, &insmodel.CheckinRecord{}, &insmodel.CheckinRecordItem{}); err != nil {
		t.Fatal(err)
	}
	store := storage.New(config.UploadConfig{Mode: "local", LocalDir: t.TempDir()}, config.OSSConfig{}, config.COSConfig{}, "")
	tenantID := "01a00000-0000-7000-8000-000000000001"
	report := model.InspectionReport{TenantID: &tenantID, CommunityID: "01a00000-0000-7000-8000-000000000002", Period: "2026-10", Title: "PDF 失败测试"}
	if err := db.Create(&report).Error; err != nil {
		t.Fatal(err)
	}
	svc := &ReportService{db: db, store: store, pdfConverter: stubPDFConverter{err: fmt.Errorf("controlled conversion failure")}}
	volume := model.DocTemplate{DocKind: "monthly_report", VolumeCode: "fire_equipment", Name: "失败分册", Format: "pdf", SkeletonFileID: skeletonRef()}
	if err := svc.generateVolume(&report, volume, model.FileStageFrozen, map[string]string{}, map[string]string{}); err == nil {
		t.Fatal("转换失败不应返回成功")
	}
	var file model.ReportFile
	if err := db.First(&file, "report_id = ? AND stage = ?", report.ID, model.FileStageFrozen).Error; err != nil {
		t.Fatal(err)
	}
	if file.Status != "failed" || file.FileID != nil || file.ErrorMessage == "" {
		t.Fatalf("失败 PDF 被错误登记: %+v", file)
	}
}
