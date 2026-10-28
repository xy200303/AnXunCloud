package service

import (
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
)

type importResultRow struct {
	dto.TemplateImportRow
	Valid      bool          `json:"valid"`
	Errors     []importIssue `json:"errors"`
	TemplateID string        `json:"template_id,omitempty"`
}

func (s *TemplateService) importTenant(c *gin.Context) (string, *errs.Error) {
	return middleware.TenantScopeOrDefault(c, s.db)
}

func (s *TemplateService) ParseImport(reader io.Reader, extension string) ([]dto.TemplateImportRow, *errs.Error) {
	return parseTemplateImport(reader, extension)
}

func (s *TemplateService) previewImport(tenantID string, rows []dto.TemplateImportRow) ([]importResultRow, *errs.Error) {
	var templates []model.CheckTemplate
	if err := s.db.Unscoped().Where("tenant_id = ?", tenantID).Find(&templates).Error; err != nil {
		return nil, errs.ErrInternal
	}
	byIdentity := map[string]model.CheckTemplate{}
	itemCodes := map[string]bool{}
	for _, template := range templates {
		key := template.PointType + "\x00" + template.Name + "\x00" + template.Purpose
		byIdentity[key] = template
		var codes []string
		s.db.Model(&model.CheckTemplateItem{}).Where("template_id = ?", template.ID).Pluck("code", &codes)
		for _, code := range codes {
			itemCodes[template.ID+":"+code] = true
		}
	}
	seen := map[string]bool{}
	results := make([]importResultRow, 0, len(rows))
	for _, row := range rows {
		result := importResultRow{TemplateImportRow: row, Valid: true, Errors: []importIssue{}}
		req, issues := importRowRequest(s.db, tenantID, row)
		result.Errors = append(result.Errors, issues...)
		templateCode := importTemplateCode(tenantID, row)
		itemKey := templateCode + ":" + req.Code
		if seen[itemKey] {
			result.Errors = append(result.Errors, importIssue{Cell: "D", Message: "文件内模板与项编码重复"})
		}
		seen[itemKey] = true
		identity := row.PointType + "\x00" + row.TemplateName + "\x00" + row.Purpose
		if template, exists := byIdentity[identity]; exists {
			result.TemplateID = template.ID
			if template.Code != templateCode {
				result.Errors = append(result.Errors, importIssue{Cell: "B", Message: "同名模板的稳定编码冲突，请更换模板名"})
			}
			if template.DeletedAt.Valid {
				result.Errors = append(result.Errors, importIssue{Cell: "B", Message: "模板已删除，编码不可复用"})
			}
			if itemCodes[template.ID+":"+req.Code] {
				result.Errors = append(result.Errors, importIssue{Cell: "D", Message: "检查项编码已存在（含退役项），不能覆盖"})
			}
		}
		result.Valid = len(result.Errors) == 0
		results = append(results, result)
	}
	return results, nil
}

func (s *TemplateService) PreviewImport(c *gin.Context, rows []dto.TemplateImportRow) (gin.H, *errs.Error) {
	tenantID, be := s.importTenant(c)
	if be != nil {
		return nil, be
	}
	results, be := s.previewImport(tenantID, rows)
	if be != nil {
		return nil, be
	}
	valid, invalid := 0, 0
	for _, row := range results {
		if row.Valid {
			valid++
		} else {
			invalid++
		}
	}
	return gin.H{"rows": results, "valid": valid, "invalid": invalid}, nil
}

func (s *TemplateService) ApplyImport(c *gin.Context, rows []dto.TemplateImportRow) (gin.H, *errs.Error) {
	tenantID, be := s.importTenant(c)
	if be != nil {
		return nil, be
	}
	results, be := s.previewImport(tenantID, rows)
	if be != nil {
		return nil, be
	}
	createdTemplates, createdItems := 0, 0
	firstTemplateID := ""
	err := s.db.Transaction(func(tx *gorm.DB) error {
		sortByTemplate := map[string]int{}
		for _, result := range results {
			if !result.Valid {
				continue
			}
			row := result.TemplateImportRow
			templateID := result.TemplateID
			if templateID == "" {
				template := model.CheckTemplate{
					TenantID: &tenantID, Code: importTemplateCode(tenantID, row), Name: row.TemplateName,
					PointType: row.PointType, Purpose: row.Purpose, Status: model.TemplateStatusDraft,
				}
				if err := tx.Create(&template).Error; err != nil {
					return fmt.Errorf("创建模板 %s: %w", row.TemplateName, err)
				}
				templateID = template.ID
				createdTemplates++
			}
			if firstTemplateID == "" {
				firstTemplateID = templateID
			}
			req, _ := importRowRequest(s.db, tenantID, row)
			item, be := templateItemFromRequest(req)
			if be != nil {
				return fmt.Errorf("第 %d 行: %s", row.Row, be.Msg)
			}
			item.TenantID, item.TemplateID = &tenantID, templateID
			if _, ok := sortByTemplate[templateID]; !ok {
				maxSort := -1
				tx.Model(&model.CheckTemplateItem{}).Where("template_id = ?", templateID).
					Select("COALESCE(MAX(sort), -1)").Scan(&maxSort)
				sortByTemplate[templateID] = maxSort
			}
			sortByTemplate[templateID]++
			item.Sort = sortByTemplate[templateID]
			if err := tx.Create(item).Error; err != nil {
				return fmt.Errorf("创建第 %d 行检查项: %w", row.Row, err)
			}
			createdItems++
		}
		return nil
	})
	if err != nil {
		return nil, errs.ErrConflict.WithMsg(err.Error())
	}
	failed := []importResultRow{}
	for _, result := range results {
		if !result.Valid {
			failed = append(failed, result)
		}
	}
	return gin.H{"created_templates": createdTemplates, "created_items": createdItems, "failed": failed, "template_id": firstTemplateID}, nil
}

func TemplateImportWorkbook(example bool) (*excelize.File, error) {
	book := excelize.NewFile()
	sheet := book.GetSheetName(0)
	for i, header := range templateImportHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			return nil, err
		}
	}
	if example {
		values := []string{"示例", "消火栓箱巡检", "patrol", "box_visual", "箱体外观", "ok", "箱体无破损", "必拍", "", "", "", "", "否", "damage", "否", "水带缺失,枪头缺失", "打开箱门拍全景", "1", "箱体、水带、枪头"}
		for i, value := range values {
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	style, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	end, _ := excelize.ColumnNumberToName(len(templateImportHeaders))
	if err := book.SetCellStyle(sheet, "A1", end+"1", style); err != nil {
		return nil, err
	}
	book.SetColWidth(sheet, "A", end, 18)
	return book, nil
}

func TemplateImportFailureWorkbook(rows []dto.TemplateImportFailureRow) (*excelize.File, error) {
	book, err := TemplateImportWorkbook(false)
	if err != nil {
		return nil, err
	}
	sheet := book.GetSheetName(0)
	book.SetCellValue(sheet, "T1", "错误说明")
	for i, row := range rows {
		values := []string{row.PointType, row.TemplateName, row.Purpose, row.ItemCode, row.ItemName, row.ValueType, row.Standard,
			row.PhotoRequired, row.Unit, row.ValueMin, row.ValueMax, row.EnumOptions, row.NoteRequired, row.AIPromptKind,
			row.AIEnabled, row.Tags, row.Guide, row.StandardNo, row.AIHint, strings.Join(row.Errors, "；")}
		for j, value := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	return book, nil
}
