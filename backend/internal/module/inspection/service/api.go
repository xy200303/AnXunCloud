package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
)

type VerifyTemplateItemRef struct {
	Code string
	Name string
}

type VerifyTemplateRef struct {
	ID    string
	Name  string
	Items []VerifyTemplateItemRef
}

func (s *TemplateService) VerifyTemplates(tenantID string) ([]VerifyTemplateRef, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("查验模板查询缺少租户范围")
	}
	var templates []model.CheckTemplate
	if err := s.db.Where("purpose = ? AND status = ? AND (tenant_id IS NULL OR tenant_id = ?)", model.TemplatePurposeVerify, model.TemplateStatusEnabled, tenantID).Order("sort, id").Find(&templates).Error; err != nil {
		return nil, err
	}
	items := s.loadItems(templateIDs(templates))
	refs := make([]VerifyTemplateRef, 0, len(templates))
	for _, template := range templates {
		ref := VerifyTemplateRef{ID: template.ID, Name: template.Name}
		for _, item := range items[template.ID] {
			ref.Items = append(ref.Items, VerifyTemplateItemRef{Code: item.Code, Name: item.Name})
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

type BuildingRef struct {
	ID       string
	Name     string
	Location string
}

func BuildingRefs(db *gorm.DB, tenantID, communityID string) ([]BuildingRef, error) {
	if tenantID == "" || communityID == "" {
		return nil, fmt.Errorf("楼栋查询缺少租户或小区范围")
	}
	var buildings []model.Building
	if err := db.Select("id", "name").Where("tenant_id = ? AND community_id = ?", tenantID, communityID).Order("sort, id").Find(&buildings).Error; err != nil {
		return nil, err
	}
	refs := make([]BuildingRef, 0, len(buildings))
	for _, building := range buildings {
		refs = append(refs, BuildingRef{ID: building.ID, Name: building.Name, Location: building.Name})
	}
	return refs, nil
}

func (s *TemplateService) TemplateDefinition(tenantID, templateID string) (*dto.TemplateDefinition, *errs.Error) {
	return contract.TemplateDefinition(s.db, tenantID, templateID)
}
