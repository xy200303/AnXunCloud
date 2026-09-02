// Package excel 批量导入/导出的 Excel 处理（xuri/excelize）：用户、点位。
package excel

import (
	"io"
	"strconv"

	"github.com/xuri/excelize/v2"
)

// 用户导入模板表头（列序与接口文档 §2.3.8 一致）。
var importHeaders = []string{"姓名", "手机号", "角色", "所属小区", "初始密码", "状态", "备注"}

// 点位导入模板表头（列序与 ParsePointImport 取值索引一致；「必拍项」列 v21 起废弃，导入时忽略）。
var pointImportHeaders = []string{"小区", "楼栋", "点位名称", "点位类型", "检查项模板", "NFC卡号", "经度", "纬度", "围栏半径(米)", "打卡方式", "必拍项(废弃)", "状态", "备注"}

// UserExportRow 用户导出行。
type UserExportRow struct {
	Username    string
	Name        string
	Phone       string
	Roles       string
	Communities string
	Status      string
	LastLoginAt string
	CreatedAt   string
}

// UserImportTemplate 生成用户导入模板：首行表头，第二行示例（导入时跳过）。
// 角色/小区为软下拉（可选列表值，也允许手工输入英文逗号分隔的多值）；状态为硬下拉。数据源按调用方租户动态生成。
func UserImportTemplate(roleNames, commNames []string) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range importHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	// 示例行动态取数据源首项（空则回落占位文案）
	exampleRole, exampleComm := "巡检员", ""
	if len(roleNames) > 0 {
		exampleRole = roleNames[0]
	}
	if len(commNames) > 0 {
		exampleComm = commNames[0]
	}
	example := []any{"张三", "13900001111", exampleRole, exampleComm, "", "启用", "示例行，导入时自动跳过"}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return nil, err
		}
	}
	// 表头加粗 + 列宽
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "G1", style); err != nil {
		return nil, err
	}
	f.SetColWidth(sheet, "A", "G", 16)

	// 隐藏引用表：A=角色名称 B=小区名称（软下拉数据源）
	const refSheet = "_ref"
	if _, err := f.NewSheet(refSheet); err != nil {
		return nil, err
	}
	for ci, col := range [][]string{roleNames, commNames} {
		for ri, v := range col {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
			if err := f.SetCellValue(refSheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	f.SetSheetVisible(refSheet, false)

	const lastRow = 502 // 数据行第 3 行起（1 表头 + 1 示例），至导入上限+2
	refRange := func(colIdx, n int) string {
		if n == 0 {
			return ""
		}
		col, _ := excelize.ColumnNumberToName(colIdx)
		return refSheet + "!$" + col + "$1:$" + col + "$" + strconv.Itoa(n)
	}
	// 软下拉：可选列表值但不阻止手工输入（角色/小区支持英文逗号分隔多值，硬拦会破坏合法输入）
	softList := func(sqref, formula, inputTitle, inputMsg string) {
		if formula == "" {
			return
		}
		dv := excelize.NewDataValidation(true)
		dv.Sqref = sqref
		dv.SetSqrefDropList(formula)
		dv.ShowErrorMessage = false // 软下拉：不阻止手工输入
		dv.SetInput(inputTitle, inputMsg)
		f.AddDataValidation(sheet, dv)
	}
	softList("C3:C"+strconv.Itoa(lastRow), refRange(1, len(roleNames)), "角色", "从下拉选择或手工填写；多个角色用英文逗号分隔，如：巡检员,维修人员")
	softList("D3:D"+strconv.Itoa(lastRow), refRange(2, len(commNames)), "所属小区", "选填；从下拉选择或手工填写，多个小区用英文逗号分隔")
	// 状态：硬下拉
	{
		dv := excelize.NewDataValidation(true)
		dv.Sqref = "F3:F" + strconv.Itoa(lastRow)
		dv.SetDropList([]string{"启用", "停用"})
		dv.SetError(excelize.DataValidationErrorStyleStop, "状态非法", "状态只能从下拉选择：启用 / 停用")
		f.AddDataValidation(sheet, dv)
	}
	return f, nil
}

// ParseUserImport 解析导入文件，返回数据行（已跳过表头与示例行，每行 7 列内按索引取值）。
func ParseUserImport(r io.Reader) ([][]string, error) {
	return parseImportRows(r)
}

// parseImportRows 解析导入文件：跳过表头与示例行，返回原始数据行。
func parseImportRows(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) <= 2 {
		return [][]string{}, nil
	}
	return rows[2:], nil
}

// PointImportTemplate 生成点位导入模板：首行表头，第二行示例（导入时跳过）。
// 选项类字段带下拉数据验证（小区/点位类型/模板名称引用隐藏表 _ref 的值列表，按调用方数据范围动态生成；
// 打卡方式/状态为固定内联列表；围栏半径为 10–2000 整数校验），防止手工乱填。
func PointImportTemplate(commNames, typeLabels, tplNames []string) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range pointImportHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	// 示例行动态取数据源首项（保证示例值在下拉列表内；空则回落占位文案）
	exampleComm, exampleType, exampleTpl := "某小区", "普通点位", "某检查模板"
	if len(commNames) > 0 {
		exampleComm = commNames[0]
	}
	if len(typeLabels) > 0 {
		exampleType = typeLabels[0]
	}
	if len(tplNames) > 0 {
		exampleTpl = tplNames[0]
	}
	example := []any{exampleComm, "1栋", "1栋3楼通道灭火器", exampleType, exampleTpl, "", "120.212001", "30.208112", "100", "扫码+围栏", "", "启用", "示例行，导入时自动跳过"}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return nil, err
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "M1", style); err != nil {
		return nil, err
	}
	f.SetColWidth(sheet, "A", "M", 16)

	// 隐藏引用表：A=小区名称 B=点位类型标签 C=检查项模板名称（下拉数据源，列内引用避免内联 255 字符上限）
	const refSheet = "_ref"
	refCols := [][]string{commNames, typeLabels, tplNames}
	if _, err := f.NewSheet(refSheet); err != nil {
		return nil, err
	}
	for ci, col := range refCols {
		for ri, v := range col {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
			if err := f.SetCellValue(refSheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	f.SetSheetVisible(refSheet, false)

	// 数据行范围：第 3 行至导入上限+2（1 表头 + 1 示例）
	lastRow := 502
	refList := func(colIdx, n int) string {
		if n == 0 {
			return ""
		}
		col, _ := excelize.ColumnNumberToName(colIdx)
		return refSheet + "!$" + col + "$1:$" + col + "$" + strconv.Itoa(n)
	}
	addList := func(sqref, formula, errTitle string) {
		if formula == "" {
			return
		}
		dv := excelize.NewDataValidation(true)
		dv.Sqref = sqref
		dv.SetSqrefDropList(formula)
		dv.SetError(excelize.DataValidationErrorStyleStop, errTitle, "请从下拉列表中选择，不要手工填写")
		f.AddDataValidation(sheet, dv)
	}
	addInline := func(sqref string, items []string, errTitle string) {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = sqref
		dv.SetDropList(items)
		dv.SetError(excelize.DataValidationErrorStyleStop, errTitle, "请从下拉列表中选择，不要手工填写")
		f.AddDataValidation(sheet, dv)
	}
	addList("A3:A"+strconv.Itoa(lastRow), refList(1, len(commNames)), "小区须从列表选择")
	addList("D3:D"+strconv.Itoa(lastRow), refList(2, len(typeLabels)), "点位类型须从列表选择")
	addList("E3:E"+strconv.Itoa(lastRow), refList(3, len(tplNames)), "检查项模板须从列表选择")
	addInline("J3:J"+strconv.Itoa(lastRow), []string{"扫码", "NFC", "任一", "围栏", "扫码+围栏", "NFC+围栏", "任一+围栏"}, "打卡方式须从列表选择")
	addInline("L3:L"+strconv.Itoa(lastRow), []string{"启用", "停用"}, "状态须从列表选择")
	// 围栏半径：10–2000 整数
	{
		dv := excelize.NewDataValidation(true)
		dv.Sqref = "I3:I" + strconv.Itoa(lastRow)
		dv.SetRange(10, 2000, excelize.DataValidationTypeWhole, excelize.DataValidationOperatorBetween)
		dv.AllowBlank = true
		dv.SetError(excelize.DataValidationErrorStyleStop, "围栏半径非法", "围栏半径须为 10–2000 的整数（可留空取系统默认）")
		f.AddDataValidation(sheet, dv)
	}
	return f, nil
}

// ParsePointImport 解析点位导入文件，返回数据行（已跳过表头与示例行）。
func ParsePointImport(r io.Reader) ([][]string, error) {
	return parseImportRows(r)
}

// CheckinExportRow 巡检记录导出行。
type CheckinExportRow struct {
	CheckinTime   string
	CommunityName string
	BuildingName  string
	PointName     string
	PointType     string
	InspectorName string
	Result        string
	Remark        string
	CheckinType   string
	Distance      string
	AIVerdict     string
	AIReason      string
	AuditStatus   string
	ForceSubmit   string
	IsSuspect     string
}

// ExportCheckins 生成巡检记录导出文件（列序与接口文档一致；中文枚举由调用方折算）。
func ExportCheckins(list []CheckinExportRow) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	headers := []string{"打卡时间", "小区", "楼栋/区域", "点位名称", "点位类型", "巡检员", "结果", "备注", "打卡方式", "距点位(米)", "AI 结论", "AI 说明", "复核状态", "强制提交", "疑似作弊"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "O1", style); err != nil {
		return nil, err
	}
	for r, row := range list {
		vals := []string{row.CheckinTime, row.CommunityName, row.BuildingName, row.PointName, row.PointType, row.InspectorName,
			row.Result, row.Remark, row.CheckinType, row.Distance, row.AIVerdict, row.AIReason, row.AuditStatus, row.ForceSubmit, row.IsSuspect}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	f.SetColWidth(sheet, "A", "O", 18)
	return f, nil
}

// ExportUsers 生成用户导出文件（不含密码、openid 等敏感字段）。
func ExportUsers(list []UserExportRow) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	headers := []string{"用户名", "姓名", "手机号", "角色", "所属小区", "状态", "最后登录时间", "创建时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", "H1", style); err != nil {
		return nil, err
	}
	for r, row := range list {
		vals := []string{row.Username, row.Name, row.Phone, row.Roles, row.Communities, row.Status, row.LastLoginAt, row.CreatedAt}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}
	f.SetColWidth(sheet, "A", "H", 18)
	return f, nil
}
