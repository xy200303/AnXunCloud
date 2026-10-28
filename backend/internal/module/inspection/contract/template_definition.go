package contract

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
)

func TemplateDefinition(db *gorm.DB, tenantID, templateID string) (*dto.TemplateDefinition, *errs.Error) {
	if tenantID == "" || templateID == "" {
		return nil, errs.ErrParam
	}
	var template model.CheckTemplate
	if err := db.First(&template, "id = ? AND tenant_id = ?", templateID, tenantID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	definition := &dto.TemplateDefinition{ID: template.ID, Code: template.Code, Name: template.Name, PointType: template.PointType, Purpose: template.Purpose, Status: template.Status, Items: []dto.TemplateDefinitionItem{}}
	var items []model.CheckTemplateItem
	if err := db.Where("template_id = ? AND retired_at IS NULL", templateID).Order("sort, id").Find(&items).Error; err != nil {
		return nil, errs.ErrInternal
	}
	for _, item := range items {
		definition.Items = append(definition.Items, dto.TemplateDefinitionItem{
			Code: item.Code, Name: item.Name, Standard: item.Standard, StandardNo: item.StandardNo, ValueType: item.ValueType,
			Unit: item.Unit, ValueMin: item.ValueMin, ValueMax: item.ValueMax,
			EnumOptions: item.EnumOptions, Tags: item.Tags,
		})
	}
	return definition, nil
}
