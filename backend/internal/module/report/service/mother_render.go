package service

import (
	"fmt"
	"time"

	"anxuncloud/internal/module/report/model"
	"anxuncloud/internal/pkg/ooxml"
	"anxuncloud/internal/pkg/uploadfile"
)

type motherRecordProfile struct {
	table, photoTable           int
	pointTypes                  map[string]bool
	itemColumns                 map[string]int
	locationColumn              int
	inspectorColumn, timeColumn int
	keepLast                    bool
}

var motherRecordProfiles = map[string][]motherRecordProfile{
	"fire_equipment": {
		{table: 4, photoTable: 5, pointTypes: typeSet("fire_extinguisher"), itemColumns: map[string]int{"pressure": 2, "body": 3, "hose": 4, "seal": 5, "validity": 6}, locationColumn: 1, inspectorColumn: 7, timeColumn: 8, keepLast: true},
		{table: 6, photoTable: 7, pointTypes: typeSet("fire_cabinet"), itemColumns: map[string]int{"visual_check": 2, "water_pressure": 6, "location_note": 7}, locationColumn: 1, inspectorColumn: 8, timeColumn: 9, keepLast: true},
		{table: 8, photoTable: 9, pointTypes: typeSet("fire_door"), itemColumns: map[string]int{"visual_check": 10, "intercom_state": 9, "location_note": 1}, locationColumn: 1, inspectorColumn: 16, timeColumn: 17, keepLast: true},
	},
	"supply": {
		{table: 4, photoTable: 5, pointTypes: typeSet("supply_room"), locationColumn: -1, inspectorColumn: 10, timeColumn: 11, keepLast: true},
		{table: 6, photoTable: 7, pointTypes: typeSet("water_pump"), locationColumn: -1, inspectorColumn: 9, timeColumn: 10, keepLast: true},
		{table: 8, photoTable: 9, pointTypes: typeSet("fire_pump"), locationColumn: -1, inspectorColumn: 9, timeColumn: 10, keepLast: true},
		{table: 10, photoTable: -1, pointTypes: typeSet("water_meter"), locationColumn: 1, inspectorColumn: 7, timeColumn: 8},
		{table: 11, photoTable: -1, pointTypes: typeSet("sub_meter"), locationColumn: 1, inspectorColumn: 7, timeColumn: 8},
	},
	"power": {
		{table: 4, photoTable: 5, pointTypes: typeSet("power_room"), locationColumn: 1, inspectorColumn: 9, timeColumn: 10, keepLast: true},
		{table: 6, photoTable: -1, pointTypes: typeSet("generator"), locationColumn: -1, inspectorColumn: 15, timeColumn: 16, keepLast: true},
		{table: 7, photoTable: -1, pointTypes: typeSet("distribution_box"), locationColumn: 1, inspectorColumn: 8, timeColumn: 9, keepLast: true},
	},
	"special": {
		{table: 4, photoTable: 5, pointTypes: typeSet("elevator_talk"), itemColumns: map[string]int{"intercom_state": 4}, locationColumn: 3, inspectorColumn: 6, timeColumn: 1, keepLast: true},
		{table: 8, photoTable: -1, pointTypes: typeSet("car_park"), locationColumn: -1, inspectorColumn: 7, timeColumn: 8, keepLast: true},
	},
	"fire_system": {
		{table: 4, photoTable: 5, pointTypes: typeSet("control_room"), locationColumn: -1, inspectorColumn: 9, timeColumn: 10, keepLast: true},
		{table: 6, photoTable: 7, pointTypes: typeSet("control_room"), locationColumn: -1, inspectorColumn: 7, timeColumn: 8, keepLast: true},
		{table: 8, photoTable: 9, pointTypes: typeSet("hydrant_test"), locationColumn: -1, inspectorColumn: 9, timeColumn: 10, keepLast: true},
		{table: 10, photoTable: 11, pointTypes: typeSet("sprinkler_test"), locationColumn: -1, inspectorColumn: 10, timeColumn: 11, keepLast: true},
		{table: 12, photoTable: 13, pointTypes: typeSet("broadcast_test"), locationColumn: 2, inspectorColumn: 11, timeColumn: 12, keepLast: true},
		{table: 14, photoTable: 15, pointTypes: typeSet("phone_test"), locationColumn: 2, inspectorColumn: 10, timeColumn: 11, keepLast: true},
		{table: 16, photoTable: 17, pointTypes: typeSet("fan_test"), locationColumn: 2, inspectorColumn: 11, timeColumn: 12, keepLast: true},
	},
}

func typeSet(values ...string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func (s *ReportService) openMother(template model.DocTemplate) (*ooxml.Template, error) {
	if template.SkeletonFileID == nil || *template.SkeletonFileID == "" {
		return nil, fmt.Errorf("输出模板未绑定甲方母版")
	}
	if raw, fileID, ok := ooxml.BuiltinTemplate(template.VolumeCode); ok && *template.SkeletonFileID == fileID {
		return ooxml.OpenTemplate(raw)
	}
	file, err := uploadfile.ByID(s.db, *template.SkeletonFileID)
	if err != nil {
		return nil, fmt.Errorf("甲方母版文件不存在")
	}
	data, err := s.store.ReadFile(file.StorageKey)
	if err != nil {
		return nil, err
	}
	return ooxml.OpenTemplate(data)
}

func (s *ReportService) renderMotherDocx(data model.MonthlyReportData, template model.DocTemplate, reportID string) ([]byte, error) {
	mother, err := s.openMother(template)
	if err != nil {
		return nil, err
	}
	if err := fillMotherCover(mother, data, reportID); err != nil {
		return nil, err
	}
	if err := fillMotherSummary(mother, data.Summary); err != nil {
		return nil, err
	}
	if err := fillMotherSignatures(mother, data); err != nil {
		return nil, err
	}
	profiles := motherRecordProfiles[template.VolumeCode]
	for _, detail := range data.Details {
		for _, profile := range profiles {
			if profile.pointTypes[detail.TypeCode] {
				if err := fillMotherRecords(mother, profile, detail, data.ImageLoader); err != nil {
					return nil, err
				}
			}
		}
	}
	return mother.Build()
}

func fillMotherCover(mother *ooxml.Template, data model.MonthlyReportData, reportID string) error {
	shortID := reportID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	values := []struct {
		prefix, text string
	}{{"巡检月度：", "巡检月度：" + data.Period}, {"编制单位：", "编制单位：" + data.CompanyName}, {"编制日期：", "编制日期：" + time.Now().Format("2006-01-02")}}
	if err := mother.SetCellText(0, 0, 1, data.CommunityName, 11); err != nil {
		return err
	}
	if err := mother.SetCellText(0, 1, 1, shortID, 16); err != nil {
		return err
	}
	for _, value := range values {
		if err := mother.SetParagraphText(value.prefix, value.text); err != nil {
			return err
		}
	}
	return nil
}

func fillMotherSummary(mother *ooxml.Template, rows []model.SummaryRow) error {
	rowCount, err := mother.TableRowCount(3)
	if err != nil {
		return err
	}
	for _, summary := range rows {
		for motherRow := 1; motherRow < rowCount; motherRow++ {
			label, err := mother.CellText(3, motherRow, 0)
			if err != nil {
				return err
			}
			if label != summary.TypeName {
				continue
			}
			values := []string{fmt.Sprint(summary.Total), fmt.Sprint(summary.Normal), fmt.Sprint(summary.Problems), fmt.Sprint(summary.Rectified), rectifyRateText(summary), summary.Remark}
			for column, value := range values {
				if err := mother.SetCellText(3, motherRow, column+1, value, 0); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func rectifyRateText(row model.SummaryRow) string {
	if row.RectifyRate > 0 {
		return fmt.Sprintf("%.1f%%", row.RectifyRate)
	}
	if row.Problems > 0 {
		return "0%"
	}
	return "—"
}

func fillMotherSignatures(mother *ooxml.Template, data model.MonthlyReportData) error {
	for index, group := range data.ReviewSigns {
		if index >= 3 {
			break
		}
		for _, sign := range group.Signs {
			if sign.SignatureFileID == "" || data.ImageLoader == nil {
				continue
			}
			content, imageType, err := data.ImageLoader(sign.SignatureFileID)
			if err != nil {
				return err
			}
			if err := mother.SetCellImage(2, 1, index, content, imageType, 360000); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func fillMotherRecords(mother *ooxml.Template, profile motherRecordProfile, detail model.DetailTable, loader func(string) ([]byte, string, error)) error {
	values := make([]map[int]string, 0, len(detail.Rows))
	for index, row := range detail.Rows {
		value := map[int]string{0: fmt.Sprint(index + 1), profile.inspectorColumn: row.Inspector, profile.timeColumn: row.Time}
		if profile.locationColumn >= 0 {
			value[profile.locationColumn] = row.Location
		}
		for itemCode, column := range profile.itemColumns {
			for keyIndex, key := range detail.ColumnKeys {
				if key == itemCode && keyIndex < len(row.Marks) {
					if itemCode == "location_note" && column == profile.locationColumn {
						if value[column] != "" && row.Marks[keyIndex] != "" {
							value[column] += " · "
						}
						value[column] += row.Marks[keyIndex]
					} else {
						value[column] = row.Marks[keyIndex]
					}
				}
			}
		}
		values = append(values, value)
	}
	if err := mother.SetRecordRows(profile.table, 1, profile.keepLast, values, 14); err != nil {
		return err
	}
	if profile.photoTable >= 0 && loader != nil {
		for _, row := range detail.Rows {
			for _, fileID := range row.PhotoIDs {
				content, imageType, err := loader(fileID)
				if err != nil {
					return err
				}
				if err := mother.SetCellImage(profile.photoTable, 1, 3, content, imageType, 288000); err != nil {
					return err
				}
				return nil
			}
		}
	}
	return nil
}
