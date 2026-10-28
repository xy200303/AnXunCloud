package model

import "gorm.io/gorm"

func TaskTemplatePurpose(db *gorm.DB, task *InspectionTask) string {
	var plan InspectionPlan
	if err := db.Select("plan_kind").First(&plan, "id = ?", task.PlanID).Error; err == nil && plan.PlanKind == PlanKindSpotcheck {
		return TemplatePurposeSpot
	}
	return TemplatePurposePatrol
}

func LoadTaskTemplateSets(db *gorm.DB, task *InspectionTask, pointIDs []string) map[string]*PointTemplateSet {
	out := map[string]*PointTemplateSet{}
	if len(task.TemplateIDs) == 0 || len(pointIDs) == 0 {
		return out
	}
	purpose := TaskTemplatePurpose(db, task)
	templateQuery := db.Where("id IN ? AND status = ? AND purpose = ?", []string(task.TemplateIDs), TemplateStatusEnabled, purpose)
	if task.TenantID != nil {
		templateQuery = templateQuery.Where("tenant_id = ?", *task.TenantID)
	}
	var templates []CheckTemplate
	templateQuery.Find(&templates)
	byID := map[string]CheckTemplate{}
	ids := make([]string, 0, len(templates))
	for _, template := range templates {
		byID[template.ID] = template
		ids = append(ids, template.ID)
	}
	if len(ids) == 0 {
		return out
	}
	itemsByTemplate := map[string][]CheckTemplateItem{}
	var items []CheckTemplateItem
	db.Where("template_id IN ? AND retired_at IS NULL", ids).Order("template_id ASC, sort ASC, id ASC").Find(&items)
	for _, item := range items {
		itemsByTemplate[item.TemplateID] = append(itemsByTemplate[item.TemplateID], item)
	}
	pointsByID := map[string]InspectionPoint{}
	var points []InspectionPoint
	db.Select("id", "type").Where("id IN ?", pointIDs).Find(&points)
	for _, point := range points {
		pointsByID[point.ID] = point
	}
	refs := LoadPointTemplateRefs(db, pointIDs)
	for _, pointID := range pointIDs {
		point, ok := pointsByID[pointID]
		if !ok {
			continue
		}
		exceptions := map[string]bool{}
		for _, ref := range refs[pointID] {
			exceptions[ref.TemplateID] = true
		}
		set := &PointTemplateSet{}
		for _, templateID := range task.TemplateIDs {
			template, ok := byID[templateID]
			if !ok || (template.PointType != point.Type && !exceptions[templateID]) {
				continue
			}
			set.TemplateIDs = append(set.TemplateIDs, templateID)
			for _, item := range itemsByTemplate[templateID] {
				set.Items = append(set.Items, UnionItem{CheckTemplateItem: item, TemplateName: template.Name})
			}
		}
		out[pointID] = set
	}
	return out
}
