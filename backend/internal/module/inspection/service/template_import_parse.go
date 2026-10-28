package service

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
)

var templateImportHeaders = []string{"点位类型", "模板名", "用途", "项编码", "项名", "值类型", "检查标准", "拍照要求", "单位", "下限", "上限", "枚举选项", "备注必填", "AI提示类型", "启用AI", "观察点", "拍照引导", "标准编号", "AI识别要点"}

type importIssue struct {
	Cell    string `json:"cell"`
	Message string `json:"message"`
}

func parseTemplateImport(r io.Reader, extension string) ([]dto.TemplateImportRow, *errs.Error) {
	var records [][]string
	var err error
	switch strings.ToLower(extension) {
	case ".xlsx":
		book, openErr := excelize.OpenReader(r)
		if openErr != nil {
			return nil, errs.ErrParam.WithMsg("无法读取 xlsx 文件，请使用未加密的标准工作簿")
		}
		defer book.Close()
		records, err = book.GetRows(book.GetSheetName(0))
	case ".csv":
		records, err = csv.NewReader(r).ReadAll()
	default:
		return nil, errs.ErrParam.WithMsg("只支持 .xlsx 或 .csv 文件")
	}
	if err != nil || len(records) == 0 {
		return nil, errs.ErrParam.WithMsg("文件为空或格式不正确")
	}
	if len(records) > 501 {
		return nil, errs.ErrParam.WithMsg("单次最多导入 500 行")
	}
	for i, expected := range templateImportHeaders {
		if i >= len(records[0]) || strings.TrimSpace(strings.TrimPrefix(records[0][i], "\ufeff")) != expected {
			return nil, errs.ErrParam.WithMsg("表头不符合模板，请下载最新导入模板")
		}
	}
	start := 1
	if len(records) > 1 && len(records[1]) > 0 && strings.TrimSpace(records[1][0]) == "示例" {
		start = 2
	}
	rows := make([]dto.TemplateImportRow, 0, len(records)-start)
	for i := start; i < len(records); i++ {
		values := make([]string, len(templateImportHeaders))
		for j := range values {
			if j < len(records[i]) {
				values[j] = strings.TrimSpace(records[i][j])
			}
		}
		empty := true
		for _, value := range values {
			if value != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}
		rows = append(rows, dto.TemplateImportRow{
			Row: i + 1, PointType: values[0], TemplateName: values[1], Purpose: values[2], ItemCode: values[3], ItemName: values[4],
			ValueType: values[5], Standard: values[6], PhotoRequired: values[7], Unit: values[8], ValueMin: values[9], ValueMax: values[10],
			EnumOptions: values[11], NoteRequired: values[12], AIPromptKind: values[13], AIEnabled: values[14], Tags: values[15],
			Guide: values[16], StandardNo: values[17], AIHint: values[18],
		})
	}
	if len(rows) == 0 {
		return nil, errs.ErrParam.WithMsg("没有可导入的数据行")
	}
	return rows, nil
}

func importCode(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:24]
}

func importTemplateCode(tenantID string, row dto.TemplateImportRow) string {
	return "tpl_" + importCode(tenantID, row.PointType, row.TemplateName, row.Purpose)
}

func importItemCode(tenantID string, row dto.TemplateImportRow) string {
	if row.ItemCode != "" {
		return strings.ToLower(row.ItemCode)
	}
	return "item_" + importCode(tenantID, row.PointType, row.TemplateName, row.ItemName)
}

func importBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "否", "关闭":
		return false, true
	case "1", "true", "是", "启用":
		return true, true
	}
	return false, false
}

func importNumber(value string) (*float64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("须为数字")
	}
	return &parsed, nil
}

func importEnumOptions(value string) (model.EnumOptionArray, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '；' || r == ';' || r == '\n' })
	options := make(model.EnumOptionArray, 0, len(parts))
	for i, part := range parts {
		label, result, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return nil, fmt.Errorf("选项须写成“显示文案=normal/abnormal”")
		}
		switch strings.TrimSpace(result) {
		case "正常":
			result = model.ItemResultNormal
		case "异常":
			result = model.ItemResultAbnormal
		}
		if result != model.ItemResultNormal && result != model.ItemResultAbnormal {
			return nil, fmt.Errorf("选项结论只能是 normal 或 abnormal")
		}
		options = append(options, model.EnumOption{Value: fmt.Sprintf("option_%d", i+1), Label: strings.TrimSpace(label), Result: result})
	}
	return options, nil
}

func importRowRequest(db *gorm.DB, tenantID string, row dto.TemplateImportRow) (*dto.TemplateItemSaveReq, []importIssue) {
	row.PointType = strings.TrimSpace(row.PointType)
	row.TemplateName = strings.TrimSpace(row.TemplateName)
	row.ItemName = strings.TrimSpace(row.ItemName)
	row.Purpose = strings.TrimSpace(row.Purpose)
	if row.Purpose == "" {
		row.Purpose = model.TemplatePurposePatrol
	}
	issues := []importIssue{}
	if row.PointType == "" || !validPointTypeForImport(db, row.PointType) {
		issues = append(issues, importIssue{Cell: "A", Message: "点位类型不存在或已停用"})
	}
	if row.TemplateName == "" {
		issues = append(issues, importIssue{Cell: "B", Message: "模板名不能为空"})
	}
	if row.Purpose != model.TemplatePurposePatrol && row.Purpose != model.TemplatePurposeSpot && row.Purpose != model.TemplatePurposeVerify {
		issues = append(issues, importIssue{Cell: "C", Message: "用途只能是 patrol、spot 或 verify"})
	}
	if row.ItemName == "" {
		issues = append(issues, importIssue{Cell: "E", Message: "项名不能为空"})
	}
	valueTypes := map[string]string{"ok": model.ValueOK, "正常异常": model.ValueOK, "number": model.ValueNumber, "数值": model.ValueNumber,
		"date": model.ValueDate, "日期": model.ValueDate, "text": model.ValueText, "文本": model.ValueText, "enum": model.ValueEnum, "枚举": model.ValueEnum}
	valueType, ok := valueTypes[strings.ToLower(row.ValueType)]
	if row.ValueType == "" {
		valueType, ok = model.ValueOK, true
	}
	if !ok {
		issues = append(issues, importIssue{Cell: "F", Message: "值类型只能是 ok、number、date、text 或 enum"})
	}
	photos := map[string]string{"": "none", "无需拍照": "none", "选拍": "optional", "必拍": "required", "none": "none", "optional": "optional", "required": "required"}
	photoRequired, ok := photos[strings.ToLower(row.PhotoRequired)]
	if !ok {
		issues = append(issues, importIssue{Cell: "H", Message: "拍照要求只能是无需拍照、选拍或必拍"})
	}
	min, minErr := importNumber(row.ValueMin)
	if minErr != nil {
		issues = append(issues, importIssue{Cell: "J", Message: "下限须为数字"})
	}
	max, maxErr := importNumber(row.ValueMax)
	if maxErr != nil {
		issues = append(issues, importIssue{Cell: "K", Message: "上限须为数字"})
	}
	if valueType == model.ValueNumber && min == nil && max == nil {
		issues = append(issues, importIssue{Cell: "J", Message: "数值项至少填写一个上下限"})
	}
	enumOptions, enumErr := importEnumOptions(row.EnumOptions)
	if enumErr != nil {
		issues = append(issues, importIssue{Cell: "L", Message: enumErr.Error()})
	}
	if valueType == model.ValueEnum && len(enumOptions) < 2 {
		issues = append(issues, importIssue{Cell: "L", Message: "枚举项至少两个选项"})
	}
	noteRequired, valid := importBool(row.NoteRequired)
	if !valid {
		issues = append(issues, importIssue{Cell: "M", Message: "备注必填只能填写是或否"})
	}
	aiEnabled, valid := importBool(row.AIEnabled)
	if !valid {
		issues = append(issues, importIssue{Cell: "O", Message: "启用AI只能填写是或否"})
	}
	promptKind := row.AIPromptKind
	if promptKind == "" {
		promptKind = "general"
	}
	tags := strings.FieldsFunc(row.Tags, func(r rune) bool { return r == '，' || r == ',' || r == '；' || r == ';' })
	req := &dto.TemplateItemSaveReq{
		Code: importItemCode(tenantID, row), Name: row.ItemName, Standard: row.Standard, StandardNo: row.StandardNo,
		ValueType: valueType, Unit: row.Unit, ValueMin: min, ValueMax: max, EnumOptions: enumOptions,
		AIHint: row.AIHint, AIPromptKind: promptKind, AIEnabled: aiEnabled, PhotoRequired: photoRequired,
		NoteRequired: noteRequired, Tags: tags, Guide: row.Guide,
	}
	if _, be := templateItemFromRequest(req); be != nil {
		issues = append(issues, importIssue{Cell: "D", Message: be.Msg})
	}
	return req, issues
}

func validPointTypeForImport(db *gorm.DB, pointType string) bool {
	return validPointType(db, pointType)
}
