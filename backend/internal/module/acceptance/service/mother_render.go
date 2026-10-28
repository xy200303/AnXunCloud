package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"anxuncloud/internal/module/acceptance/model"
	inspectionsvc "anxuncloud/internal/module/inspection/service"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/ooxml"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/uploadfile"
)

type acceptanceProfile struct {
	volume, name string
}

var acceptanceProfiles = map[string]acceptanceProfile{
	"documents":  {volume: "acceptance_documents", name: "物业资料承接查验报告"},
	"building":   {volume: "acceptance_building", name: "房屋本体承接查验报告"},
	"outdoor":    {volume: "acceptance_outdoor", name: "室外场地承接查验报告"},
	"facilities": {volume: "acceptance_facilities", name: "设施设备承接查验报告"},
	"garage":     {volume: "acceptance_garage", name: "地下车库承接查验报告"},
	"premises":   {volume: "acceptance_premises", name: "物业用房承接查验报告"},
	"unit":       {volume: "acceptance_unit", name: "物业分户验收台账"},
}

var acceptanceCategoryAliases = map[string]string{
	"documents": "documents", "document": "documents", "docs": "documents",
	"building": "building", "building_matrix": "building", "house": "building", "housing": "building",
	"outdoor": "outdoor", "facilities": "facilities", "facility": "facilities",
	"garage": "garage", "premises": "premises", "unit": "unit", "unit_acceptance": "unit",
}

func (s *BatchService) createMotherReportFiles(tx *gorm.DB, batch *model.Batch, targets []model.Target, records []model.Record, signatures []model.Signature) (*model.ReportFile, error) {
	profiles := map[string]bool{}
	for _, target := range targets {
		category := acceptanceCategoryAliases[target.CategoryCode]
		if category == "" {
			return nil, fmt.Errorf("未配置承接查验母版类别: %s", target.CategoryCode)
		}
		profiles[category] = true
	}
	for _, record := range records {
		category := acceptanceCategoryAliases[record.CategoryCode]
		if category == "" {
			return nil, fmt.Errorf("记录缺少承接查验母版类别: %s", record.CategoryCode)
		}
		profiles[category] = true
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("批次没有可输出的承接查验类别")
	}
	ordered := make([]string, 0, len(profiles))
	for category := range profiles {
		ordered = append(ordered, category)
	}
	sort.Strings(ordered)
	var first *model.ReportFile
	for _, category := range ordered {
		profile := acceptanceProfiles[category]
		file, err := s.createMotherReportFile(tx, batch, profile, targets, records, signatures)
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = file
		}
	}
	return first, nil
}

func (s *BatchService) createMotherReportFile(tx *gorm.DB, batch *model.Batch, profile acceptanceProfile, targets []model.Target, records []model.Record, signatures []model.Signature) (*model.ReportFile, error) {
	var existing model.ReportFile
	stage := "frozen"
	if len(signatures) > 0 {
		stage = "archived"
	}
	if err := tx.Where("batch_id = ? AND revision = ? AND volume_code = ? AND stage = ? AND format = ? AND status = ?", batch.ID, batch.Revision, profile.volume, stage, "docx", "ready").First(&existing).Error; err == nil {
		return &existing, nil
	}
	raw, _, ok := ooxml.BuiltinTemplate(profile.volume)
	if !ok {
		return nil, fmt.Errorf("承接查验母版不存在: %s", profile.volume)
	}
	mother, err := ooxml.OpenTemplate(raw)
	if err != nil {
		return nil, err
	}
	communityName, tenantName := s.acceptanceNames(tx, batch)
	if err := mother.SetCellText(0, 0, 1, communityName, 11); err != nil {
		return nil, err
	}
	if err := mother.SetParagraphText("查验时间：", "查验时间："+batch.PeriodStart.Format("2006-01-02")); err != nil {
		return nil, err
	}
	if err := mother.SetParagraphText("查验单位：", "查验单位："+tenantName); err != nil {
		return nil, err
	}
	category := profileKey(profile)
	targetByID := map[string]model.Target{}
	for _, target := range targets {
		targetByID[target.ID] = target
	}
	names, err := acceptanceSnapshotItemNames(batch)
	if err != nil {
		return nil, err
	}
	if category == "unit" {
		if err := s.fillAcceptanceUnitReport(mother, targets, records, names); err != nil {
			return nil, err
		}
	} else {
		if err := s.fillCoveredNormal(mother, targets, records, names, category); err != nil {
			return nil, err
		}
		groups := map[string][]model.Record{}
		for _, record := range records {
			if acceptanceCategoryAliases[record.CategoryCode] != category {
				continue
			}
			if category == "unit" {
				return nil, fmt.Errorf("分户台账必须使用一户一档渲染器")
			}
			spec, ok := acceptanceTableSlots[record.TableCode]
			if !ok {
				return nil, fmt.Errorf("母版槽位未登记: %s", record.TableCode)
			}
			if _, ok := names[record.TableCode+"\x00"+record.ItemCode]; !ok {
				return nil, fmt.Errorf("记录不在冻结快照: %s/%s", record.TableCode, record.ItemCode)
			}
			if _, ok := targetByID[record.TargetID]; !ok {
				return nil, fmt.Errorf("记录查验对象不存在: %s", record.TargetID)
			}
			groups[record.TableCode] = append(groups[record.TableCode], record)
			if err := s.fillAcceptanceRecord(mother, spec, record, names); err != nil {
				return nil, err
			}
		}
		groupCodes := make([]string, 0, len(groups))
		for code := range groups {
			groupCodes = append(groupCodes, code)
		}
		sort.Strings(groupCodes)
		for _, code := range groupCodes {
			if err := s.fillAcceptanceProblems(mother, acceptanceTableSlots[code], groups[code], targetByID, names); err != nil {
				return nil, err
			}
		}
	}
	for _, signature := range signatures {
		if err := s.fillAcceptanceSignature(mother, category, signature); err != nil {
			return nil, err
		}
	}
	data, err := mother.Build()
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("报告文件存储未配置")
	}
	filename := batch.Name + "-" + profile.volume + "-" + stage + ".docx"
	key, url, err := s.store.SaveGenerated("acceptance", filename, data)
	if err != nil {
		return nil, err
	}
	tenantID := batch.TenantID
	fileID := systemsvc.RegisterGeneratedFileScene(tx, s.store, &tenantID, batch.CommunityID, "acceptance_batch", batch.ID, "acceptance", filename, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", storage.MD5Hex(data), key, url)
	if fileID == "" || fileID == systemsvc.SystemFileOwner {
		return nil, fmt.Errorf("报告文件登记失败")
	}
	digest := sha256.Sum256(data)
	file := model.ReportFile{TenantID: batch.TenantID, BatchID: batch.ID, Revision: batch.Revision, VolumeCode: profile.volume, VolumeName: profile.name, Stage: stage, Format: "docx", FileID: &fileID, SHA256: hex.EncodeToString(digest[:]), Status: "ready"}
	if err := tx.Create(&file).Error; err != nil {
		return nil, err
	}
	if err := s.createAcceptancePDF(tx, batch, profile, stage, data); err != nil {
		return nil, err
	}
	return &file, nil
}

func profileKey(profile acceptanceProfile) string {
	for key, value := range acceptanceProfiles {
		if value == profile {
			return key
		}
	}
	return ""
}

func (s *BatchService) acceptanceNames(tx *gorm.DB, batch *model.Batch) (string, string) {
	var community sysmodel.Community
	_ = tx.Select("name").First(&community, "id = ?", batch.CommunityID).Error
	var tenant sysmodel.Tenant
	_ = tx.Select("name").First(&tenant, "id = ?", batch.TenantID).Error
	if community.Name == "" {
		community.Name = batch.Name
	}
	return community.Name, tenant.Name
}

func acceptanceSnapshotItemNames(batch *model.Batch) (map[string]string, error) {
	out := map[string]string{}
	data, _ := json.Marshal(batch.TemplateSnapshot["schemas"])
	var schemas []struct {
		Code       string          `json:"code"`
		TemplateID *string         `json:"template_id"`
		Rows       []model.ListRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &schemas); err != nil {
		return nil, err
	}
	data, _ = json.Marshal(batch.TemplateSnapshot["verify_templates"])
	var templates []inspectionsvc.VerifyTemplateRef
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, err
	}
	for _, schema := range schemas {
		for _, row := range schema.Rows {
			out[schema.Code+"\x00"+row.Code] = row.Name
		}
		for _, template := range templates {
			if schema.TemplateID == nil || template.ID != *schema.TemplateID {
				continue
			}
			for _, item := range template.Items {
				out[schema.Code+"\x00"+item.Code] = item.Name
			}
		}
	}
	return out, nil
}

func (s *BatchService) fillAcceptanceRecord(mother *ooxml.Template, spec acceptanceTableSpec, record model.Record, names map[string]string) error {
	itemName := names[record.TableCode+"\x00"+record.ItemCode]
	row, column, found, err := mother.FindCell(spec.table, itemName)
	if err != nil || !found {
		return fmt.Errorf("母版找不到检查项 %s/%s（%s）", record.TableCode, record.ItemCode, itemName)
	}
	if spec.kind == "matrix" {
		value := acceptanceResultMark(record.Result)
		if record.ValueText != "" {
			value = record.ValueText
		}
		return mother.SetCellText(spec.table, row+1, column, value, 0)
	}
	if value := acceptanceQuantity(record); value != "" {
		if _, quantityColumn, ok, err := mother.FindCell(spec.table, "数量"); err != nil {
			return err
		} else if ok {
			if err := mother.SetCellText(spec.table, row, quantityColumn, value, 0); err != nil {
				return err
			}
		}
	}
	_, noteColumn, ok, err := findAcceptanceCell(mother, spec.table, "缺项（问题）", "备注（缺项）", "问题", "备注")
	if err != nil || !ok {
		return fmt.Errorf("母版清单 %s 未找到问题或备注列", record.TableCode)
	}
	return mother.SetCellText(spec.table, row, noteColumn, record.Note, 0)
}

func acceptanceQuantity(record model.Record) string {
	if record.ValueText != "" {
		return record.ValueText
	}
	if value, ok := record.Values["quantity"]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func acceptanceResultMark(result string) string {
	if result == "normal" || result == "pass" {
		return "√"
	}
	return "×"
}

func findAcceptanceCell(mother *ooxml.Template, table int, candidates ...string) (int, int, bool, error) {
	for _, candidate := range candidates {
		row, column, found, err := mother.FindCell(table, candidate)
		if err != nil || found {
			return row, column, found, err
		}
	}
	return 0, 0, false, nil
}

func (s *BatchService) fillAcceptanceProblems(mother *ooxml.Template, spec acceptanceTableSpec, records []model.Record, targets map[string]model.Target, names map[string]string) error {
	if spec.twoUp {
		return s.fillAcceptanceDocumentPhotos(mother, spec, records, names)
	}
	table, first, end := spec.photo, 1, 0
	if spec.embedded {
		marker, _, found, err := mother.FindCell(table, "查验照片")
		if err != nil || !found {
			return fmt.Errorf("母版表格 %d 未找到内嵌照片区", table)
		}
		first = marker + 2
	}
	rows, err := mother.TableRowCount(table)
	if err != nil {
		return err
	}
	end = rows
	if spec.embedded {
		end--
	}
	if first >= end {
		return fmt.Errorf("母版表格 %d 照片行范围无效", table)
	}
	_, descriptionColumn, found, err := findAcceptanceCell(mother, table, "问题描述", "问题说明", "问题简述", "说明")
	if err != nil || !found {
		return fmt.Errorf("母版表格 %d 未找到问题描述列", table)
	}
	_, imageColumn, found, err := findAcceptanceCell(mother, table, "现场照片", "照片")
	if err != nil || !found {
		return fmt.Errorf("母版表格 %d 未找到照片列", table)
	}
	_, locationColumn, locationFound, err := findAcceptanceCell(mother, table, "具体位置", "位置/编号", "名称/编号", "房号", "区域")
	if err != nil {
		return err
	}
	_, categoryColumn, categoryFound, err := findAcceptanceCell(mother, table, "问题类别", "问题/类别", "类别")
	if err != nil {
		return err
	}
	if err := mother.ResizeRows(table, first, end, len(records)); err != nil {
		return err
	}
	for i, record := range records {
		row := first + i
		target := targets[record.TargetID]
		if err := mother.SetCellText(table, row, 0, fmt.Sprint(i+1), 0); err != nil {
			return err
		}
		if locationFound {
			location := target.Location
			if location == "" || target.TargetType == "unit" {
				location = target.Name
			}
			if err := mother.SetCellText(table, row, locationColumn, location, 0); err != nil {
				return err
			}
		}
		if categoryFound {
			if err := mother.SetCellText(table, row, categoryColumn, names[record.TableCode+"\x00"+record.ItemCode], 0); err != nil {
				return err
			}
		}
		note := record.Note
		if record.ValueText != "" && spec.kind == "matrix" {
			note = "实测值 " + record.ValueText + "；" + note
		}
		if err := mother.SetCellText(table, row, descriptionColumn, note, 0); err != nil {
			return err
		}
		for _, fileID := range record.Photos {
			file, err := uploadfile.ByID(s.db, fileID)
			if err != nil {
				return err
			}
			data, err := s.store.ReadFile(file.StorageKey)
			if err != nil {
				return err
			}
			if err := mother.SetCellImage(table, row, imageColumn, data, file.MimeType, 288000, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *BatchService) fillAcceptanceSignature(mother *ooxml.Template, category string, signature model.Signature) error {
	file, err := uploadfile.ByID(s.db, signature.FileID)
	if err != nil {
		return err
	}
	data, err := s.store.ReadFile(file.StorageKey)
	if err != nil {
		return err
	}
	column := -1
	switch strings.ToLower(signature.SubjectCode) {
	case "inspector", "transferor":
		column = 0
	case "reviewer", "receiver":
		column = 1
	}
	if column < 0 || (category != "documents" && strings.Contains(strings.ToLower(signature.SubjectCode), "transfer")) {
		return nil
	}
	return mother.SetCellImage(3, 1, column, data, file.MimeType, 360000, true)
}
