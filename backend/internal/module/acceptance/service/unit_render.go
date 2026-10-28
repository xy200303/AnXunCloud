package service

import (
	"fmt"
	"sort"
	"strings"

	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/pkg/ooxml"
)

var unitCategoryColumns = map[string]int{
	"unit_door":     2,
	"unit_building": 3,
	"unit_windows":  4,
	"unit_kitchen":  5,
	"unit_meter":    6,
	"unit_electric": 7,
}

var unitPhotoOffsets = map[string]int{
	"unit_door":     0,
	"unit_building": 1,
	"unit_windows":  2,
	"unit_kitchen":  3,
	"unit_electric": 4,
}

func (s *BatchService) fillAcceptanceUnitReport(mother *ooxml.Template, targets []model.Target, records []model.Record, names map[string]string) error {
	units := make([]model.Target, 0, len(targets))
	for _, target := range targets {
		if acceptanceCategoryAliases[target.CategoryCode] == "unit" {
			if target.BuildingID == nil || target.Name == "" {
				return fmt.Errorf("分户对象缺少房号或所属楼栋")
			}
			units = append(units, target)
		}
	}
	if len(units) == 0 {
		return fmt.Errorf("分户台账没有房屋对象")
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].Location != units[j].Location {
			return units[i].Location < units[j].Location
		}
		if units[i].Name != units[j].Name {
			return units[i].Name < units[j].Name
		}
		return units[i].ID < units[j].ID
	})
	nextTable, err := mother.ReplaceTableSections([][2]int{{5, 6}, {7, 8}, {9, 10}, {11, 12}}, len(units))
	if err != nil {
		return err
	}
	recordsByTarget := map[string][]model.Record{}
	for _, record := range records {
		if acceptanceCategoryAliases[record.CategoryCode] != "unit" {
			continue
		}
		if _, ok := unitCategoryColumns[record.TableCode]; !ok {
			return fmt.Errorf("分户母版槽位未登记: %s", record.TableCode)
		}
		if _, ok := names[record.TableCode+"\x00"+record.ItemCode]; !ok {
			return fmt.Errorf("分户记录不在冻结快照: %s/%s", record.TableCode, record.ItemCode)
		}
		recordsByTarget[record.TargetID] = append(recordsByTarget[record.TargetID], record)
	}
	type buildingSummary struct {
		name   string
		counts [6]int
	}
	buildings := map[string]*buildingSummary{}
	areas := map[string]bool{}
	photoRecords := map[int][]model.Record{}
	targetByID := map[string]model.Target{}
	for i, target := range units {
		targetByID[target.ID] = target
		table, photo := 5+i*2, 6+i*2
		if err := mother.SetCellText(table, 0, 1, target.Name, 0); err != nil {
			return err
		}
		buildingID := *target.BuildingID
		summary := buildings[buildingID]
		if summary == nil {
			summary = &buildingSummary{name: target.Location}
			buildings[buildingID] = summary
		}
		areas[target.Location] = true
		targetRecords := recordsByTarget[target.ID]
		if target.CoverageConfirmed {
			for _, tableCode := range target.TableCodes {
				if _, ok := unitCategoryColumns[tableCode]; !ok {
					return fmt.Errorf("分户覆盖专项未登记: %s", tableCode)
				}
				if err := s.fillCoveredTableNormal(mother, acceptanceTableSpec{table: table, photo: photo, kind: "matrix"}, tableCode, targetRecords, names); err != nil {
					return err
				}
			}
		}
		for _, record := range targetRecords {
			summary.counts[unitCategoryColumns[record.TableCode]-2]++
			if err := s.fillAcceptanceRecord(mother, acceptanceTableSpec{table: table, photo: photo, kind: "matrix"}, record, names); err != nil {
				return err
			}
			if offset, ok := unitPhotoOffsets[record.TableCode]; ok {
				photoRecords[offset] = append(photoRecords[offset], record)
			}
		}
		if len(targetRecords) > 0 {
			if err := s.fillAcceptanceProblems(mother, acceptanceTableSpec{table: table, photo: photo, kind: "matrix"}, targetRecords, map[string]model.Target{target.ID: target}, names); err != nil {
				return err
			}
		}
	}
	for offset, group := range photoRecords {
		if err := s.fillAcceptanceProblems(mother, acceptanceTableSpec{table: nextTable + offset, photo: nextTable + offset, kind: "list"}, group, targetByID, names); err != nil {
			return err
		}
	}
	buildingIDs := make([]string, 0, len(buildings))
	for id := range buildings {
		buildingIDs = append(buildingIDs, id)
	}
	sort.Slice(buildingIDs, func(i, j int) bool { return buildings[buildingIDs[i]].name < buildings[buildingIDs[j]].name })
	summaryRows := make([]map[int]string, 0, len(buildingIDs))
	totals := [6]int{}
	for i, id := range buildingIDs {
		summary := buildings[id]
		row := map[int]string{0: fmt.Sprint(i + 1), 1: summary.name}
		for category, count := range summary.counts {
			totals[category] += count
			row[category+2] = fmt.Sprint(count)
		}
		summaryRows = append(summaryRows, row)
	}
	if err := mother.SetRecordRows(4, 1, true, summaryRows, 0); err != nil {
		return err
	}
	totalRow, err := mother.TableRowCount(4)
	if err != nil {
		return err
	}
	totalRow--
	for category, count := range totals {
		if err := mother.SetCellText(4, totalRow, category+2, fmt.Sprint(count), 0); err != nil {
			return err
		}
	}
	areaNames := make([]string, 0, len(areas))
	for area := range areas {
		areaNames = append(areaNames, area)
	}
	sort.Strings(areaNames)
	if err := mother.SetCellText(0, 1, 1, strings.Join(areaNames, "、"), 0); err != nil {
		return err
	}
	waterRows := make([]map[int]string, 0, len(units))
	electricRows := make([]map[int]string, 0, len(units))
	for i, target := range units {
		values := map[string]any{}
		for _, record := range recordsByTarget[target.ID] {
			for key, value := range record.Values {
				values[key] = value
			}
		}
		waterRows = append(waterRows, map[int]string{0: fmt.Sprint(i + 1), 1: target.Name, 2: unitValueText(values, "water_meter_number"), 3: unitValueText(values, "water_reading")})
		electricRows = append(electricRows, map[int]string{0: fmt.Sprint(i + 1), 1: target.Name, 2: unitValueText(values, "electric_meter_number"), 3: unitValueText(values, "electric_reading")})
	}
	if err := mother.SetRecordRows(nextTable+5, 1, false, waterRows, 0); err != nil {
		return err
	}
	return mother.SetRecordRows(nextTable+6, 1, false, electricRows, 0)
}

func unitValueText(values map[string]any, key string) string {
	if value, ok := values[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}
