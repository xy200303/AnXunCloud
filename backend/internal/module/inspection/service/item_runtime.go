package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/strutil"
)

func LoadItemDefinitions(db *gorm.DB, items []model.CheckinRecordItem) {
	templateIDs, codes := []string{}, []string{}
	seen := map[string]bool{}
	for _, item := range items {
		if item.TemplateID == nil || seen[*item.TemplateID+":"+item.ItemCode] {
			continue
		}
		seen[*item.TemplateID+":"+item.ItemCode] = true
		templateIDs = append(templateIDs, *item.TemplateID)
		codes = append(codes, item.ItemCode)
	}
	if len(templateIDs) == 0 {
		return
	}
	var definitions []model.CheckTemplateItem
	db.Where("template_id IN ? AND code IN ?", templateIDs, codes).Find(&definitions)
	byKey := map[string]*model.CheckTemplateItem{}
	for i := range definitions {
		byKey[definitions[i].TemplateID+":"+definitions[i].Code] = &definitions[i]
	}
	for i := range items {
		if items[i].TemplateID != nil {
			items[i].Definition = byKey[*items[i].TemplateID+":"+items[i].ItemCode]
		}
	}
}

func ItemPhoto(item model.CheckinRecordItem, photos []ai.PhotoRef) ai.ItemPhoto {
	out := ai.ItemPhoto{InstanceKey: item.InstanceKey(), Name: item.Name, Standard: item.Standard, Photos: photos}
	definition := item.Definition
	if definition == nil {
		return out
	}
	out.AIHint = strutil.StrVal(definition.AIHint)
	out.AIPromptKind = definition.AIPromptKind
	out.ValueType = definition.ValueType
	out.Unit = definition.Unit
	out.ValueMin = definition.ValueMin
	out.ValueMax = definition.ValueMax
	out.Tags = definition.Tags
	for _, option := range definition.EnumOptions {
		out.EnumOptions = append(out.EnumOptions, option.Label)
	}
	return out
}
