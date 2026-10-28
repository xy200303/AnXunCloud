package service

import (
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

func (s *PlanService) validateTemplateSelection(req *dto.PlanSaveReq, tenantID *string) (types.IDArray, *errs.Error) {
	ids := uniqueIDs(req.TemplateIDs)
	purpose := model.TemplatePurposePatrol
	if req.PlanKind == model.PlanKindSpotcheck {
		purpose = model.TemplatePurposeSpot
	}
	var templates []model.CheckTemplate
	if err := s.db.Where("id IN ? AND tenant_id = ? AND status = ? AND purpose = ?", ids, *tenantID, model.TemplateStatusEnabled, purpose).Find(&templates).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(templates) != len(ids) {
		return nil, errs.ErrParam.WithMsg("template_ids 包含不存在、跨租户、未启用或用途不符的模板")
	}
	for _, id := range ids {
		var count int64
		if err := s.db.Model(&model.CheckTemplateItem{}).Where("template_id = ? AND retired_at IS NULL", id).Count(&count).Error; err != nil {
			return nil, errs.ErrInternal
		}
		if count == 0 {
			return nil, errs.ErrConflict.WithMsg("所选模板没有有效检查项")
		}
	}
	if req.SelectionMode == model.SelectionByPointTypes {
		available := map[string]bool{}
		for _, template := range templates {
			available[template.PointType] = true
		}
		for _, pointType := range req.PointTypes {
			if !available[pointType] {
				return nil, errs.ErrParam.WithMsg("点位类型「" + pointType + "」没有匹配的检查项模板")
			}
		}
		return types.IDArray(ids), nil
	}
	var points []model.InspectionPoint
	if err := s.db.Select("id", "name", "type").Where("id IN ? AND community_id = ?", req.PointIDs, req.CommunityID).Find(&points).Error; err != nil {
		return nil, errs.ErrInternal
	}
	refs := model.LoadPointTemplateRefs(s.db, req.PointIDs)
	for _, point := range points {
		matched := false
		for _, template := range templates {
			if template.PointType == point.Type {
				matched = true
				break
			}
			for _, ref := range refs[point.ID] {
				if ref.TemplateID == template.ID {
					matched = true
					break
				}
			}
		}
		if !matched {
			return nil, errs.ErrParam.WithMsg("点位「" + point.Name + "」与所选模板类型不匹配")
		}
	}
	return types.IDArray(ids), nil
}
