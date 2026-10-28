package service

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/uploadfile"
)

func (s *ReportService) reportFiles(reportID string) []model.ReportFile {
	var files []model.ReportFile
	s.db.Where("report_id = ?", reportID).Order("created_at, id").Find(&files)
	return files
}

func reportFileViews(files []model.ReportFile) []gin.H {
	items := make([]gin.H, 0, len(files))
	for _, file := range files {
		items = append(items, gin.H{
			"id": file.ID, "doc_kind": file.DocKind, "volume_code": file.VolumeCode, "volume_name": file.VolumeName,
			"stage": file.Stage, "format": file.Format, "file_id": file.FileID, "sha256": file.SHA256,
			"generation_batch": file.GenerationBatch, "status": file.Status, "error_message": file.ErrorMessage,
			"created_at": timefmt.T(file.CreatedAt),
		})
	}
	return items
}

func (s *ReportService) hasArchivedFile(reportID string) bool {
	var count int64
	s.db.Model(&model.ReportFile{}).Where("report_id = ? AND status = ? AND stage IN ?", reportID, model.FileStatusReady, []string{model.FileStageArchived, model.FileStageLegacyArchive}).Count(&count)
	return count > 0
}

func (s *ReportService) archivedFile(reportID string) (*model.ReportFile, *sysmodel.UploadFile, error) {
	var file model.ReportFile
	if err := s.db.Where("report_id = ? AND status = ? AND stage IN ?", reportID, model.FileStatusReady, []string{model.FileStageArchived, model.FileStageLegacyArchive}).
		Order("CASE stage WHEN 'archived' THEN 0 ELSE 1 END").Order("created_at DESC").First(&file).Error; err != nil {
		return nil, nil, err
	}
	if file.FileID == nil {
		return nil, nil, fmt.Errorf("归档文件未登记文件 ID")
	}
	upload, err := uploadfile.ByID(s.db, *file.FileID)
	return &file, &upload, err
}

func (s *ReportService) hasFrozenFile(reportID string) bool {
	var report model.InspectionReport
	if err := s.db.Select("tenant_id").First(&report, "id = ?", reportID).Error; err != nil || report.TenantID == nil {
		return false
	}
	templates, err := s.monthlyTemplates(*report.TenantID)
	if err != nil || len(templates) == 0 {
		return false
	}
	var count int64
	s.db.Model(&model.ReportFile{}).Where("report_id = ? AND stage = ? AND status = ?", reportID, model.FileStageFrozen, model.FileStatusReady).Count(&count)
	return int(count) == len(templates)
}

func (s *ReportService) frozenFile(reportID string) (*model.ReportFile, *sysmodel.UploadFile, error) {
	var file model.ReportFile
	if err := s.db.Where("report_id = ? AND stage = ? AND status = ?", reportID, model.FileStageFrozen, model.FileStatusReady).Order("created_at DESC").First(&file).Error; err != nil {
		return nil, nil, err
	}
	if file.FileID == nil {
		return nil, nil, fmt.Errorf("冻结文件未登记文件 ID")
	}
	upload, err := uploadfile.ByID(s.db, *file.FileID)
	return &file, &upload, err
}

func (s *ReportService) freezePDF(reportID string) {
	if err := s.enqueueRender(context.Background(), reportID, model.FileStageFrozen); err != nil {
		logger.L.Warn("月报冻结任务入队失败", zap.String("report_id", reportID), zap.Error(err))
	}
}

func (s *ReportService) ResumePendingFrozenFiles() {
	var ids []string
	s.db.Model(&model.InspectionReport{}).Where("status = ?", model.StatusPendingReview).Pluck("id", &ids)
	for _, id := range ids {
		s.freezePDF(id)
	}
}

func (s *ReportService) RetryFrozenFile(c *gin.Context, reportID, fileID string) *errs.Error {
	if _, be := s.getWithScope(c, reportID); be != nil {
		return be
	}
	var count int64
	if err := s.db.Model(&model.ReportFile{}).Where("id = ? AND report_id = ? AND stage = ? AND status = ?", fileID, reportID, model.FileStageFrozen, "failed").Count(&count).Error; err != nil {
		return errs.ErrInternal
	}
	if count != 1 {
		return errs.ErrConflict.WithMsg("只有失败的冻结文件可以重试")
	}
	if err := s.enqueueRender(c.Request.Context(), reportID, model.FileStageFrozen); err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *ReportService) DownloadFile(c *gin.Context, reportID, reportFileID string) ([]byte, string, string, *errs.Error) {
	r, be := s.getWithScope(c, reportID)
	if be != nil {
		return nil, "", "", be
	}
	if be := s.checkPDFAccess(c, r); be != nil {
		return nil, "", "", be
	}
	var reportFile model.ReportFile
	if err := s.db.First(&reportFile, "id = ? AND report_id = ? AND status = ?", reportFileID, reportID, model.FileStatusReady).Error; err != nil || reportFile.FileID == nil {
		return nil, "", "", errs.ErrNotFound
	}
	file, err := uploadfile.ByID(s.db, *reportFile.FileID)
	if err != nil || file.TenantID == nil || r.TenantID == nil || *file.TenantID != *r.TenantID {
		return nil, "", "", errs.ErrDataScope
	}
	data, err := s.store.ReadFile(file.StorageKey)
	if err != nil {
		return nil, "", "", errs.ErrInternal.WithMsg("报告文件读取失败")
	}
	return data, file.Name, file.MimeType, nil
}
