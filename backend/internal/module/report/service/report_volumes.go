package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"anxuncloud/internal/module/report/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/types"
)

func (s *ReportService) monthlyTemplates(tenantID string) ([]model.DocTemplate, error) {
	if s.docTemplates == nil {
		return nil, fmt.Errorf("输出模板服务未配置")
	}
	return s.docTemplates.EffectiveMonthly(tenantID)
}

func (s *ReportService) typeSystems() (map[string]string, map[string]string, error) {
	rows, err := systemsvc.EnabledDictionary(s.db, "point_type")
	if err != nil {
		return nil, nil, err
	}
	byCode, byLabel := map[string]string{}, map[string]string{}
	for _, row := range rows {
		system, _ := row.Attrs["system"].(string)
		byCode[row.Value], byLabel[row.Label] = system, system
	}
	return byCode, byLabel, nil
}

func filterVolumeData(data model.MonthlyReportData, template model.DocTemplate, byCode, byLabel map[string]string) model.MonthlyReportData {
	filtered := data
	filtered.TitleLine = template.Name
	filtered.TypeNames, filtered.Summary, filtered.Details, filtered.PhotoGroups, filtered.Ledger = nil, nil, nil, nil, nil
	for _, name := range data.TypeNames {
		if byLabel[name] == template.SystemFilter {
			filtered.TypeNames = append(filtered.TypeNames, name)
		}
	}
	for _, row := range data.Summary {
		if byLabel[row.TypeName] == template.SystemFilter {
			filtered.Summary = append(filtered.Summary, row)
		}
	}
	for _, detail := range data.Details {
		if byCode[detail.TypeCode] == template.SystemFilter {
			filtered.Details = append(filtered.Details, detail)
		}
	}
	for _, group := range data.PhotoGroups {
		if byLabel[group.Title] == template.SystemFilter {
			filtered.PhotoGroups = append(filtered.PhotoGroups, group)
		}
	}
	for _, row := range data.Ledger {
		if byLabel[row.Category] == template.SystemFilter {
			filtered.Ledger = append(filtered.Ledger, row)
		}
	}
	if rawSlots, ok := template.Layout["sign_slots"].([]any); ok && len(rawSlots) > 0 {
		filtered.ReviewSigns = nil
		allowed := map[int]bool{}
		for _, raw := range rawSlots {
			if slot, ok := raw.(map[string]any); ok {
				if index, ok := slot["step_index"].(float64); ok {
					allowed[int(index)] = true
				}
			}
		}
		for i, group := range data.ReviewSigns {
			if allowed[i] {
				filtered.ReviewSigns = append(filtered.ReviewSigns, group)
			}
		}
	}
	if len(filtered.Summary) == 0 {
		filtered.Summary = append(filtered.Summary, model.SummaryRow{TypeName: template.SystemFilter, Remark: "本月无巡检记录"})
	}
	if len(filtered.Details) == 0 {
		filtered.Details = append(filtered.Details, model.DetailTable{TypeName: template.SystemFilter, Note: "本月无巡检记录"})
	}
	return filtered
}

func (s *ReportService) generateVolume(r *model.InspectionReport, template model.DocTemplate, stage string, byCode, byLabel map[string]string) error {
	if r.TenantID == nil {
		return fmt.Errorf("报告缺少租户")
	}
	var existing model.ReportFile
	if err := s.db.Where("report_id = ? AND volume_code = ? AND stage = ? AND status = ?", r.ID, template.VolumeCode, stage, model.FileStatusReady).First(&existing).Error; err == nil {
		return nil
	}
	if template.Format != "docx" && template.Format != "pdf" {
		return fmt.Errorf("当前分册只支持 docx 或 pdf 输出")
	}
	job := model.ReportFile{TenantID: *r.TenantID, ReportID: r.ID, DocKind: template.DocKind, VolumeCode: template.VolumeCode, VolumeName: template.Name, Stage: stage, Format: template.Format, GenerationBatch: types.NewID(), Status: "running"}
	if err := s.db.Create(&job).Error; err != nil {
		return err
	}
	fail := func(err error) error {
		s.db.Model(&job).Updates(map[string]any{"status": "failed", "error_message": strutil.Truncate(err.Error(), 512)})
		return err
	}
	facts, err := s.pdfData(r, &template)
	if err != nil {
		return fail(err)
	}
	data, err := s.renderMotherDocx(filterVolumeData(facts, template, byCode, byLabel), template, r.ID)
	if err != nil {
		return fail(err)
	}
	mimeType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	if template.Format == "pdf" {
		if s.pdfConverter == nil {
			return fail(fmt.Errorf("PDF 转换器未配置"))
		}
		data, err = s.pdfConverter.Convert(context.Background(), data)
		if err != nil {
			return fail(err)
		}
		mimeType = "application/pdf"
	}
	filename := r.Title + "-" + template.VolumeCode + "-" + stage + "." + template.Format
	key, url, err := s.store.SaveGenerated("reports", filename, data)
	if err != nil {
		return fail(err)
	}
	fileID := systemsvc.RegisterGeneratedFile(s.db, s.store, r.TenantID, r.CommunityID, "inspection_report", r.ID, filename, mimeType, storage.MD5Hex(data), key, url)
	if fileID == "" || fileID == systemsvc.SystemFileOwner {
		return fail(fmt.Errorf("报告文件登记失败"))
	}
	digest := sha256.Sum256(data)
	return s.db.Model(&job).Updates(map[string]any{"status": model.FileStatusReady, "file_id": fileID, "sha256": hex.EncodeToString(digest[:]), "error_message": ""}).Error
}

func (s *ReportService) generateVolumes(r *model.InspectionReport, stage string) error {
	if r.TenantID == nil {
		return fmt.Errorf("报告缺少租户")
	}
	templates, err := s.monthlyTemplates(*r.TenantID)
	if err != nil {
		return err
	}
	byCode, byLabel, err := s.typeSystems()
	if err != nil {
		return err
	}
	for _, template := range templates {
		if err := s.generateVolume(r, template, stage, byCode, byLabel); err != nil {
			return err
		}
	}
	return nil
}
