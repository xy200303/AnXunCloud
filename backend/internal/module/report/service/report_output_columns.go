package service

import (
	"encoding/json"
	"fmt"
	"sort"

	contract "anxuncloud/internal/module/inspection/contract"
	inscontract "anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/report/model"
)

type detailColumnSet struct {
	Columns []detailColumn
}

func outputBindingFor(s *ReportService, docTemplate *model.DocTemplate, pointType string) *model.TemplateOutput {
	if docTemplate == nil || docTemplate.TenantID == nil {
		return nil
	}
	bindings, err := s.docTemplates.outputBindings(*docTemplate.TenantID, docTemplate.ID)
	if err != nil {
		return nil
	}
	for i := range bindings {
		definition, be := contract.TemplateDefinition(s.db, *docTemplate.TenantID, bindings[i].TemplateID)
		if be == nil && len(definition.Items) > 0 && (pointType == "" || definition.PointType == pointType) {
			return &bindings[i]
		}
	}
	return nil
}

func outputTemplateColumns(s *ReportService, points []*inscontract.PointFact, docTemplate *model.DocTemplate, itemsByRec map[string][]inscontract.ItemFact) detailColumnSet {
	pointType := ""
	if len(points) > 0 {
		pointType = points[0].Type
	}
	binding := outputBindingFor(s, docTemplate, pointType)
	if binding == nil {
		return selectedTemplateColumns(s, docTemplate, pointType, itemsByRec)
	}
	definition, be := contract.TemplateDefinition(s.db, *docTemplate.TenantID, binding.TemplateID)
	if be != nil {
		return detailColumnSet{}
	}
	seen := map[string]bool{}
	order := []string{}
	raw, _ := json.Marshal(binding.Columns["items"])
	var configured []OutputColumn
	_ = json.Unmarshal(raw, &configured)
	for _, column := range configured {
		if !seen[column.ItemCode] {
			seen[column.ItemCode] = true
			order = append(order, column.ItemCode)
		}
	}
	for _, item := range definition.Items {
		if !seen[item.Code] {
			seen[item.Code] = true
			order = append(order, item.Code)
		}
	}
	columns := []detailColumn{}
	for _, code := range order {
		for _, item := range definition.Items {
			if item.Code != code {
				continue
			}
			if len(item.Tags) > 0 {
				for _, tag := range item.Tags {
					columns = append(columns, detailColumn{label: tag, item: item.Name, tag: tag, templateID: definition.ID, itemCode: item.Code, unit: item.Unit, standard: item.Standard, standardNo: item.StandardNo})
				}
			} else {
				columns = append(columns, detailColumn{label: item.Name, item: item.Name, templateID: definition.ID, itemCode: item.Code, unit: item.Unit, standard: item.Standard, standardNo: item.StandardNo})
			}
		}
	}
	return detailColumnSet{Columns: columns}
}

func selectedTemplateColumns(s *ReportService, docTemplate *model.DocTemplate, pointType string, itemsByRec map[string][]inscontract.ItemFact) detailColumnSet {
	if docTemplate == nil || docTemplate.TenantID == nil {
		return detailColumnSet{}
	}
	ids := map[string]bool{}
	for _, items := range itemsByRec {
		for _, item := range items {
			if item.TemplateID != nil {
				ids[*item.TemplateID] = true
			}
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		definition, be := contract.TemplateDefinition(s.db, *docTemplate.TenantID, id)
		if be != nil || definition.PointType != pointType || len(definition.Items) == 0 {
			continue
		}
		columns := make([]detailColumn, 0, len(definition.Items))
		for _, item := range definition.Items {
			if len(item.Tags) > 0 {
				for _, tag := range item.Tags {
					columns = append(columns, detailColumn{label: tag, item: item.Name, tag: tag, templateID: definition.ID, itemCode: item.Code, unit: item.Unit, standard: item.Standard, standardNo: item.StandardNo})
				}
			} else {
				columns = append(columns, detailColumn{label: item.Name, item: item.Name, templateID: definition.ID, itemCode: item.Code, unit: item.Unit, standard: item.Standard, standardNo: item.StandardNo})
			}
		}
		return detailColumnSet{Columns: columns}
	}
	return detailColumnSet{}
}

func outputRowMode(s *ReportService, docTemplate *model.DocTemplate, pointType string) string {
	binding := outputBindingFor(s, docTemplate, pointType)
	if binding == nil {
		return "point"
	}
	return binding.RowMode
}

type outputRecordRow struct {
	record      inscontract.RecordFact
	suffix      string
	equipmentID *string
}

func expandOutputRecords(mode string, records []inscontract.RecordFact, items map[string][]inscontract.ItemFact, equipmentNames map[string]string) []outputRecordRow {
	if len(records) == 0 {
		return nil
	}
	if mode == "grid" || mode == "date" {
		rows := make([]outputRecordRow, 0, len(records))
		for _, record := range records {
			rows = append(rows, outputRecordRow{record: record})
		}
		return rows
	}
	if mode == "equipment" {
		rows := []outputRecordRow{}
		for _, record := range records {
			keys := map[string]bool{}
			for _, item := range items[record.ID] {
				key := ""
				if item.EquipmentID != nil {
					key = *item.EquipmentID
				}
				keys[key] = true
			}
			if len(keys) == 0 {
				keys[""] = true
			}
			ordered := make([]string, 0, len(keys))
			for key := range keys {
				ordered = append(ordered, key)
			}
			sort.Strings(ordered)
			for _, key := range ordered {
				entry := outputRecordRow{record: record}
				if key != "" {
					equipmentID := key
					entry.equipmentID = &equipmentID
					name := equipmentNames[key]
					if name == "" {
						name = key
					}
					entry.suffix = " · 设备 " + name
				}
				rows = append(rows, entry)
			}
		}
		return rows
	}
	return []outputRecordRow{{record: records[len(records)-1]}}
}

func pointAxisLabel(point *inscontract.PointFact, axis string, buildingNames map[string]string) string {
	if point == nil {
		return ""
	}
	switch axis {
	case "floor":
		if point.Floor != nil {
			if *point.Floor < 0 {
				return fmt.Sprintf("B%d", -*point.Floor)
			}
			return fmt.Sprintf("%dF", *point.Floor)
		}
	case "unit":
		if point.UnitNo != nil {
			return fmt.Sprintf("%d单元", *point.UnitNo)
		}
	case "building":
		if point.BuildingID != nil {
			return buildingNames[*point.BuildingID]
		}
	}
	return ""
}
