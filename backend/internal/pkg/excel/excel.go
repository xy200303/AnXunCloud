// Package excel 用户批量导入/导出的 Excel 处理（xuri/excelize）。
package excel

import (
	"io"

	"github.com/xuri/excelize/v2"
)

// 用户导入模板表头（列序与接口文档 §2.3.8 一致）。
var importHeaders = []string{"姓名", "手机号", "角色", "所属小区", "初始密码", "状态", "备注"}

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

// UserImportTemplate 生成导入模板：首行表头，第二行示例（导入时跳过）。
func UserImportTemplate() (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range importHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	example := []any{"张三", "13900001111", "巡检员", "翡翠湾小区", "", "启用", "示例行，导入时自动跳过"}
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
	return f, nil
}

// ParseUserImport 解析导入文件，返回数据行（已跳过表头与示例行，每行 7 列内按索引取值）。
func ParseUserImport(r io.Reader) ([][]string, error) {
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
