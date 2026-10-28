package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/module/acceptance/model"
	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/storage"
)

type acceptanceStubPDFConverter struct{ calls int }

func (c *acceptanceStubPDFConverter) Convert(context.Context, []byte) ([]byte, error) {
	c.calls++
	return []byte("%PDF-1.7\nacceptance\n%%EOF\n"), nil
}

func TestCreateAcceptancePDFRegistersRealFormatOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ReportFile{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}); err != nil {
		t.Fatal(err)
	}
	store := storage.New(config.UploadConfig{Mode: "local", LocalDir: t.TempDir()}, config.OSSConfig{}, config.COSConfig{}, "")
	converter := &acceptanceStubPDFConverter{}
	svc := &BatchService{db: db, store: store, pdfConverter: converter}
	batch := &model.Batch{TenantID: "tenant-1", CommunityID: "community-1", Name: "真实批次", Revision: 3}
	batch.ID = "batch-1"
	profile := acceptanceProfiles["outdoor"]
	if err := svc.createAcceptancePDF(db, batch, profile, "frozen", []byte("PK\x03\x04docx")); err != nil {
		t.Fatal(err)
	}
	if err := svc.createAcceptancePDF(db, batch, profile, "frozen", []byte("PK\x03\x04docx")); err != nil {
		t.Fatal(err)
	}
	if converter.calls != 1 {
		t.Fatalf("相同阶段 PDF 不得重复转换: %d", converter.calls)
	}
	var file model.ReportFile
	if err := db.First(&file, "batch_id = ? AND format = ?", batch.ID, "pdf").Error; err != nil {
		t.Fatal(err)
	}
	if file.Status != "ready" || file.FileID == nil {
		t.Fatalf("PDF 产物未就绪: %+v", file)
	}
	var upload sysmodel.UploadFile
	if err := db.First(&upload, "id = ?", *file.FileID).Error; err != nil {
		t.Fatal(err)
	}
	if upload.MimeType != "application/pdf" {
		t.Fatalf("PDF MIME 错误: %s", upload.MimeType)
	}
}
