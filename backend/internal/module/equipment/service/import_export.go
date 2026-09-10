package service

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

// 甲方台账列结构（§3.2.1，导入模板直接兼容，按列头名识别、兼容顺序变化）。
var equipmentImportHeaders = []string{
	"序号", "机房名称", "设备等级", "设备分类", "设备名称", "设备编号", "设备品牌", "管控区域",
	"规格型号", "设备原值", "设备数量", "出厂日期", "竣工验收日", "投运日期", "安装位置", "异动状态",
	"维保状态", "运行状态", "备注", "产地", "厂家联系人", "安装单位联系人电话", "维保单位",
	"维保单位联系人电话", "其他信息",
}

// typeAlias 设备分类别名表：甲方台账分类名 → equipment_type 字典 value。
var typeAlias = map[string]string{
	"灭火器设施": "extinguisher",
	"消火栓设施": "hydrant",
	"电梯轿厢":  "elevator",
	"电梯机房":  "elevator",
}

// statusAlias 异动状态 → 台账状态（未识别按在用）。
var statusAlias = map[string]string{
	"启用": model.StatusInService,
	"停用": model.StatusStopped,
	"报废": model.StatusScrapped,
}

// equipmentImportMaxRows 单次导入数据行上限（甲方真实台账 1986 行，放宽到 2000）。
const equipmentImportMaxRows = 2000

// equipmentExportMaxRows 导出上限。
const equipmentExportMaxRows = 5000

// serialDateRe Excel 序列日期（纯数字/小数，合理区间 1954–2119 年）。
var serialDateRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

// ParseFlexibleDate 导入日期解析（纯函数）：兼容 2018-05-01 / 2018/5/1 / 2018年5月 / Excel 序列日期等。
// 空串、占位垃圾值（"0"、"/"、"-" 等）与无法解析的值一律返回 nil（甲方台账日期列允许为空/乱填，不为单行垃圾失败）。
func ParseFlexibleDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Excel 序列日期（1900 日期系统，原点 1899-12-30）
	if serialDateRe.MatchString(s) {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 20000 && f <= 80000 {
			t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.Local).AddDate(0, 0, int(f))
			return &t
		}
		return nil
	}
	layouts := []string{
		"2006-01-02", "2006-1-2",
		"2006/01/02", "2006/1/2",
		"2006.01.02", "2006.1.2",
		"2006年1月2日", "2006年01月02日",
		"2006年1月", "2006年01月",
		"2006-01", "2006/01", "2006.01",
		"2006-01-02 15:04:05", "2006/1/2 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			d := truncateDay(t)
			return &d
		}
	}
	return nil
}

// ImportTemplate 台账导入模板：列头即甲方台账列结构（第 1 行表头 + 第 2 行示例，导入按列头名识别可跳过）。
func (s *EquipmentService) ImportTemplate() (*excelize.File, *errs.Error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range equipmentImportHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, errs.ErrInternal
		}
	}
	example := []any{
		1, "消防水泵房", "一级", "灭火器设施", "1栋3楼灭火器", "XCCT-MH-0001", "某品牌", "1栋",
		"MFZ/ABC4", "", 1, "2023-05-01", "", "2023-06-01", "1栋3楼通道", "启用",
		"自行维保", "正常", "", "湖北", "", "", "某维保公司", "", "示例行，导入时自动跳过",
	}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return nil, errs.ErrInternal
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, errs.ErrInternal
	}
	if err := f.SetCellStyle(sheet, "A1", "Y1", style); err != nil {
		return nil, errs.ErrInternal
	}
	f.SetColWidth(sheet, "A", "Y", 14)
	return f, nil
}

// Import 逐行导入台账到指定小区：设备编号必填，同租户重复按更新（幂等重导）；
// 设备分类先按字典 label 精确匹配，再按别名表，都不中自动扩充字典（零值规则=不自动判到期）。
func (s *EquipmentService) Import(c *gin.Context, communityID string, r io.Reader) (*dto.ImportResult, string, *errs.Error) {
	if strings.TrimSpace(communityID) == "" {
		return nil, "", errs.ErrParam.WithMsg("community_id 为必填项（整个文件导入到指定小区）")
	}
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, "", be
	}
	tenantID := middleware.CommunityTenantID(s.db, communityID)
	if tenantID == nil {
		return nil, "", errs.ErrCommunityNotExist
	}
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, "", errs.ErrImportFileType
	}
	defer f.Close()
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, "", errs.ErrImportFileType
	}
	// 找列表头行：前若干行可能是大标题（甲方台账第 1 行为项目名大标题、第 2 行才是列头）
	headerIdx, colOf := locateHeader(rows)
	if headerIdx < 0 {
		return nil, "", errs.ErrParam.WithMsg("未找到列表头（需包含「设备编号」「设备名称」列）")
	}
	dataRows := make([][]string, 0, len(rows))
	rowNums := make([]int, 0, len(rows))
	for i := headerIdx + 1; i < len(rows); i++ {
		if isEmptyRow(rows[i]) {
			continue
		}
		dataRows = append(dataRows, rows[i])
		rowNums = append(rowNums, i+1) // Excel 实际行号
	}
	if len(dataRows) == 0 {
		return nil, "", errs.ErrImportEmpty
	}
	if len(dataRows) > equipmentImportMaxRows {
		return nil, "", errs.ErrParam.WithMsg(fmt.Sprintf("超过单次导入上限（%d 行）", equipmentImportMaxRows))
	}

	// 预载：equipment_type 字典（label/value 双映射）+ 本租户已有编号（重复=更新）
	typeByText := map[string]string{} // label/value → value
	{
		var dds []sysmodel.SysDictData
		s.db.Select("label", "value").Where("type_code = 'equipment_type'").Find(&dds)
		for _, dd := range dds {
			typeByText[dd.Label] = dd.Value
			typeByText[dd.Value] = dd.Value
		}
	}
	existingByCode := map[string]model.Equipment{}
	{
		var es []model.Equipment
		s.db.Where("tenant_id = ?", *tenantID).Find(&es)
		for i := range es {
			existingByCode[es[i].Code] = es[i]
		}
	}
	rules, _ := s.typeRules()

	result := &dto.ImportResult{Total: len(dataRows), FailDetails: []dto.ImportFail{}}
	for i, row := range dataRows {
		cell := func(header string) string {
			idx, ok := colOf[header]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		code := cell("设备编号")
		fail := func(reason string) {
			result.FailDetails = append(result.FailDetails, dto.ImportFail{Row: rowNums[i], Code: code, Reason: reason})
		}
		if code == "" {
			fail("设备编号必填")
			continue
		}
		name := cell("设备名称")
		if name == "" {
			name = code // 名称为空以编号兜底（name 列 NOT NULL）
		}
		// 设备分类 → type：字典精确匹配 → 别名表 → 自动扩充字典
		typeText := cell("设备分类")
		if typeText == "" {
			fail("设备分类必填")
			continue
		}
		typeValue, be := s.resolveType(typeText, typeByText)
		if be != nil {
			fail(be.Msg)
			continue
		}
		// 出厂日期优先，投运日期兜底（甲方台账出厂日期常空）；投运日期原值留 extra
		manufacture := ParseFlexibleDate(cell("出厂日期"))
		putIntoServiceRaw := cell("投运日期")
		if manufacture == nil {
			manufacture = ParseFlexibleDate(putIntoServiceRaw)
		}
		extra := types.JSONMap{}
		putExtra := func(key, v string) {
			if v != "" {
				extra[key] = v
			}
		}
		putExtra("level", cell("设备等级"))
		putExtra("brand", cell("设备品牌"))
		putExtra("spec", cell("规格型号"))
		putExtra("maint_status", cell("维保状态"))
		putExtra("vendor", cell("维保单位"))
		putExtra("origin", cell("产地"))
		putExtra("put_into_service", putIntoServiceRaw)
		// 安装位置 + 管控区域 + 备注 → remark 拼接（点位无对应关系，point_id 留空待 App 扫码绑定）
		remark := joinRemark(cell("安装位置"), cell("管控区域"), cell("备注"))
		status := model.StatusInService
		if v, ok := statusAlias[cell("异动状态")]; ok {
			status = v
		}
		rule := rules[typeValue]
		nextDue := CalcNextDueDate(manufacture, nil, rule.FirstMonths, rule.CycleMonths)
		scrapDate := CalcScrapDate(manufacture, rule.ScrapMonths)

		if existing, ok := existingByCode[code]; ok {
			// 同编号按更新（幂等重导）：不覆盖人工已改的状态/点位绑定/到期日覆盖
			updates := map[string]any{
				"name": name, "type": typeValue,
				"manufacture_date": manufacture, "extra": extra, "remark": remark,
			}
			if existing.NextDueDate == nil {
				updates["next_due_date"] = nextDue
			}
			if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
				fail("更新失败：" + err.Error())
				continue
			}
			result.UpdatedCount++
			continue
		}
		e := model.Equipment{
			TenantID: tenantID, CommunityID: communityID,
			Type: typeValue, Code: code, Name: name,
			ManufactureDate: manufacture,
			NextDueDate:     nextDue,
			ScrapDate:       scrapDate,
			Status:          status,
			Extra:           extra, Remark: remark,
		}
		if err := s.db.Create(&e).Error; err != nil {
			fail("写入失败：" + err.Error())
			continue
		}
		existingByCode[code] = e
		result.CreatedCount++
	}
	result.FailCount = len(result.FailDetails)
	msg := fmt.Sprintf("导入完成：新增 %d 条，更新 %d 条，失败 %d 条", result.CreatedCount, result.UpdatedCount, result.FailCount)
	return result, msg, nil
}

// locateHeader 在前 5 行内定位列表头行（含「设备编号」「设备名称」列），返回行索引与 列头→列号 映射。
func locateHeader(rows [][]string) (int, map[string]int) {
	for i := 0; i < len(rows) && i < 5; i++ {
		colOf := map[string]int{}
		for j, cell := range rows[i] {
			if h := strings.TrimSpace(cell); h != "" {
				colOf[h] = j
			}
		}
		if _, ok := colOf["设备编号"]; ok {
			if _, ok2 := colOf["设备名称"]; ok2 {
				return i, colOf
			}
		}
	}
	return -1, nil
}

// resolveType 设备分类解析：字典 label/value 精确匹配 → 别名表 → 自动创建字典项（value=custom_N 递增，attrs 空=零值规则）。
func (s *EquipmentService) resolveType(text string, typeByText map[string]string) (string, *errs.Error) {
	if v, ok := typeByText[text]; ok {
		return v, nil
	}
	if v, ok := typeAlias[text]; ok {
		return v, nil
	}
	// 自动扩充字典：value=custom_N（取现存最大值 +1）
	maxN := 0
	for v := range typeByText {
		if strings.HasPrefix(v, "custom_") {
			if n, err := strconv.Atoi(strings.TrimPrefix(v, "custom_")); err == nil && n > maxN {
				maxN = n
			}
		}
	}
	value := fmt.Sprintf("custom_%d", maxN+1)
	dd := sysmodel.SysDictData{
		TypeCode: "equipment_type", Label: text, Value: value,
		Sort: 900 + maxN + 1, Status: sysmodel.StatusEnabled,
		Remark: "台账导入自动扩充（无规则参数=不自动判到期，可在字典管理补规则）",
	}
	if err := s.db.Create(&dd).Error; err != nil {
		return "", errs.ErrInternal
	}
	typeByText[text] = value
	typeByText[value] = value
	return value, nil
}

// joinRemark 安装位置/管控区域/备注拼接（各段去空，「·」连接位置信息，「；」接备注）。
func joinRemark(location, area, remark string) string {
	parts := []string{}
	if location != "" {
		parts = append(parts, location)
	}
	if area != "" && area != location {
		parts = append(parts, area)
	}
	out := strings.Join(parts, " · ")
	if remark != "" {
		if out != "" {
			out += "；"
		}
		out += remark
	}
	if len([]rune(out)) > 255 {
		out = string([]rune(out)[:255])
	}
	return out
}

// isEmptyRow 判断整行为空。
func isEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// Export 当前筛选结果全量导出（上限 5000 条，xlsx）。
func (s *EquipmentService) Export(c *gin.Context, q *dto.ListQuery) (*excelize.File, *errs.Error) {
	var rows []model.Equipment
	if err := s.filtered(c, q).Order("code ASC").Limit(equipmentExportMaxRows).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := s.toItems(rows)
	f := excelize.NewFile()
	sheet := "Sheet1"
	headers := []string{"序号", "设备编号", "设备名称", "设备类型", "小区", "楼栋", "点位",
		"出厂日期", "最近维保日期", "下次到期日", "报废日期", "到期状态", "状态", "备注"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, errs.ErrInternal
		}
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, errs.ErrInternal
	}
	if err := f.SetCellStyle(sheet, "A1", "N1", style); err != nil {
		return nil, errs.ErrInternal
	}
	dueLabels := map[string]string{DueNone: "无到期日", DueNormal: "正常", DueWarning: "临期", DueOverdue: "已逾期"}
	get := func(item gin.H, key string) string {
		if v, ok := item[key].(string); ok {
			return v
		}
		return ""
	}
	for r, item := range items {
		vals := []string{
			strconv.Itoa(r + 1), get(item, "code"), get(item, "name"), get(item, "type_label"),
			get(item, "community_name"), get(item, "building_name"), get(item, "point_name"),
			get(item, "manufacture_date"), get(item, "last_maintenance_date"), get(item, "next_due_date"),
			get(item, "scrap_date"), dueLabels[get(item, "due_state")], get(item, "status_label"), get(item, "remark"),
		}
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, errs.ErrInternal
			}
		}
	}
	f.SetColWidth(sheet, "A", "N", 16)
	return f, nil
}
