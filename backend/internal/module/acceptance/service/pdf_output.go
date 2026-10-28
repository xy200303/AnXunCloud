package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/module/acceptance/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/storage"
)

func (s *BatchService) createAcceptancePDF(tx *gorm.DB, batch *model.Batch, profile acceptanceProfile, stage string, docx []byte) error {
	var existing model.ReportFile
	if err := tx.Where("batch_id = ? AND revision = ? AND volume_code = ? AND stage = ? AND format = ? AND status = ?", batch.ID, batch.Revision, profile.volume, stage, "pdf", "ready").First(&existing).Error; err == nil {
		return nil
	}
	if s.pdfConverter == nil {
		return fmt.Errorf("PDF 转换器未配置")
	}
	data, err := s.pdfConverter.Convert(context.Background(), docx)
	if err != nil {
		return err
	}
	filename := batch.Name + "-" + profile.volume + "-" + stage + ".pdf"
	key, url, err := s.store.SaveGenerated("acceptance", filename, data)
	if err != nil {
		return err
	}
	tenantID := batch.TenantID
	fileID := systemsvc.RegisterGeneratedFileScene(tx, s.store, &tenantID, batch.CommunityID, "acceptance_batch", batch.ID, "acceptance", filename, "application/pdf", storage.MD5Hex(data), key, url)
	if fileID == "" || fileID == systemsvc.SystemFileOwner {
		return fmt.Errorf("PDF 文件登记失败")
	}
	digest := sha256.Sum256(data)
	file := model.ReportFile{TenantID: batch.TenantID, BatchID: batch.ID, Revision: batch.Revision, VolumeCode: profile.volume, VolumeName: profile.name, Stage: stage, Format: "pdf", FileID: &fileID, SHA256: hex.EncodeToString(digest[:]), Status: "ready"}
	return tx.Create(&file).Error
}
