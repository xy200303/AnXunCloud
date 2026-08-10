// Package pdf 月度巡检报告 PDF 生成（gofpdf + 内嵌 Noto Sans SC 中文字体）。
// 版式 1:1 复刻《物业消防设施（器材类）月度巡检报告》模板：
// 封面 / 目录 / 说明 / 检查工作概述 / 本月检查汇总表 / 分项巡检明细 / 现场照片 / 隐患问题清单及整改台账 / 典型整改事项 / 附件清单 / 签字审批栏。
package pdf

import (
	"bytes"
	"fmt"
	"image"
	// 解码 JPEG/PNG 图片尺寸（DecodeConfig）
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"github.com/phpdave11/gofpdf"
)

// 表头橄榄绿底（取自模板）
var oliveHeader = [3]int{190, 185, 165}

const (
	pageW   = 210.0
	margin  = 15.0
	contentW = pageW - margin*2 // 180
)

// SignInfo 签字栏信息（Name 空 / Time 空表示待签字）。
type SignInfo struct {
	Name   string
	Time   string
	Remark string
	// SignatureKey 签字时的手写签名图快照 file_key（空回退打印姓名）
	SignatureKey string
}

// SummaryRow 本月检查汇总表行（按点位类型一行）。
type SummaryRow struct {
	TypeName    string  // 设施类别
	Total       int     // 总数（点位数）
	Normal      int     // 正常完好（正常打卡数）
	InspectRate float64 // 巡检完成率 %
	Problems    int     // 存在问题（异常打卡数）
	Rectified   int     // 整改完毕（已闭环工单数）
	RectifyRate float64 // 整改完成率 %
	Remark      string
}

// DetailRow 分项巡检明细行（一条打卡记录一行）。
type DetailRow struct {
	Location  string   // 位置/编号（点位名称+编码）
	Marks     []string // 与 DetailTable.Items 对齐：√/×/空（未检）
	Problem   string   // 问题说明（异常原因）
	Inspector string   // 巡检人
	Time      string   // 巡检时间
}

// DetailTable 分项巡检明细表（每个有数据的点位类型一张）。
type DetailTable struct {
	TypeName string   // 设施类别中文名
	Items    []string // 检查项列（该类别检查项模板）
	Note     string   // 表格下方"注：检查标准……"
	Rows     []DetailRow
}

// LedgerRow 隐患问题清单及整改台账行（异常工单）。
type LedgerRow struct {
	Location   string // 位置/编号
	TypeName   string // 设施类型
	Problem    string // 问题描述
	Assignee   string // 整改责任人
	FinishTime string // 完成时间
	Status     string // 整改状态
	Reviewer   string // 复查人
	ReviewDate string // 复查日期
	Remark     string
}

// TypicalItem 本月典型整改事项（问题描述 + 处理结果）。
type TypicalItem struct {
	Problem string
	Result  string
}

// PhotoCell 现场照片单元（标注 + 图片 file_key）。
type PhotoCell struct {
	Label string // 小标注：检查项名（逐项照片）或"全景"（记录级照片）
	Key   string // 图片 file_key（经 ImageLoader 加载）
}

// PhotoGroup 现场照片分组（一条打卡记录一组）。
type PhotoGroup struct {
	Title string // 组标题：点位名称 + 打卡时间
	Cells []PhotoCell
}

// MonthlyReportData 月度巡检报告 PDF 数据。
type MonthlyReportData struct {
	CommunityName string   // 项目名称（小区名）
	Period        string   // YYYY-MM
	CompanyName   string   // 管理单位落款（空则封面留白 / 页尾 XX物业服务中心）
	Approved      bool     // 已终审（公章与报告日期仅终审后展示）
	ApproveDate   string   // 终审日期 YYYY-MM-DD
	SealKey       string   // 公章图 file_key（仅 Approved 时嵌入）
	TypeNames     []string // 设施类别（该小区有点位的类型中文名）
	Summary       []SummaryRow
	Details       []DetailTable
	PhotoGroups   []PhotoGroup // 现场照片（按打卡记录分组）
	Ledger        []LedgerRow
	Typical       []TypicalItem
	// 三级签字栏：巡检员确认名单（含时间）、安全负责人、物业经理
	InspectorSigns []SignInfo
	Supervisor     SignInfo
	Manager        SignInfo
	// ImageLoader 按 file_key 加载图片字节与类型（JPG/PNG）；nil 则跳过签名图/公章。
	// 单张加载失败返回 error，PDF 侧跳过该张，不影响整体生成。
	ImageLoader func(fileKey string) (data []byte, imgType string, err error)
}

// GenerateMonthly 生成月度巡检报告 PDF（A4 纵向，内嵌 Noto Sans SC 中文字体）。
func GenerateMonthly(data MonthlyReportData) ([]byte, error) {
	regular, err := Fonts.ReadFile("fonts/NotoSansSC-Regular.ttf")
	if err != nil {
		return nil, fmt.Errorf("读取内嵌字体失败: %w", err)
	}
	bold, err := Fonts.ReadFile("fonts/NotoSansSC-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("读取内嵌字体失败: %w", err)
	}

	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(margin, margin, margin)
	p.SetAutoPageBreak(true, 18)
	p.AddUTF8FontFromBytes("noto", "", regular)
	p.AddUTF8FontFromBytes("noto", "B", bold)
	if p.Err() {
		return nil, fmt.Errorf("加载中文字体失败: %w", p.Error())
	}

	renderCover(p, data)
	renderTOC(p, data)
	renderIntro(p, data)
	renderOverviewAndSummary(p, data)
	renderDetails(p, data)
	renderPhotos(p, data)
	renderLedger(p, data.Ledger)
	renderTypical(p, data.Typical)
	renderAttachments(p)
	renderSignBar(p, data)

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, fmt.Errorf("生成 PDF 失败: %w", err)
	}
	return buf.Bytes(), nil
}

// ========== 基础排版辅助 ==========

// sectionTitle 章节标题（加粗）。
func sectionTitle(p *gofpdf.Fpdf, title string) {
	p.SetFont("noto", "B", 14)
	p.CellFormat(contentW, 10, title, "", 1, "L", false, 0, "")
}

// wrapText 按列宽把文本折行（按 rune 测宽；\n 强制换行）。
func wrapText(p *gofpdf.Fpdf, text string, w float64) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		var cur strings.Builder
		for _, r := range para {
			if p.GetStringWidth(cur.String()+string(r)) > w {
				lines = append(lines, cur.String())
				cur.Reset()
			}
			cur.WriteRune(r)
		}
		lines = append(lines, cur.String())
	}
	return lines
}

// paragraph 渲染正文段落（首行缩进两字符，跟随自动分页）。
func paragraph(p *gofpdf.Fpdf, text string, size, lh float64, indent bool) {
	p.SetFont("noto", "", size)
	lines := wrapText(p, text, contentW)
	for i, ln := range lines {
		if i == 0 && indent {
			p.CellFormat(11, lh, "", "", 0, "L", false, 0, "")
			p.CellFormat(contentW-11, lh, ln, "", 1, "L", false, 0, "")
		} else {
			p.CellFormat(contentW, lh, ln, "", 1, "L", false, 0, "")
		}
	}
}

// trunc 超出列宽的文本截断（避免 Cell 溢出）。
func trunc(p *gofpdf.Fpdf, s string, w float64) string {
	for len(s) > 0 && p.GetStringWidth(s) > w-2 {
		s = s[:len(s)-1]
	}
	return s
}

// ensureSpace 剩余高度不足 need 时先换页（避免章节标题孤行）。
func ensureSpace(p *gofpdf.Fpdf, need float64) {
	_, pageH := p.GetPageSize()
	if p.GetY()+need > pageH-18 {
		p.AddPage()
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func pctText(v float64) string { return fmt.Sprintf("%.1f%%", v) }

// periodCN "2026-08" → "2026年8月"。
func periodCN(period string) string {
	t, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return period
	}
	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month()))
}

// dateCN "2026-08-09..." → "2026 年 8 月 9 日"；空/解析失败返回空。
func dateCN(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		t, err = time.ParseInLocation("2006-01-02", s[:min(10, len(s))], time.Local)
		if err != nil {
			return ""
		}
	}
	return fmt.Sprintf("%d 年 %d 月 %d 日", t.Year(), int(t.Month()), t.Day())
}

// headerRow 橄榄绿底表头行（单元格按宽度自动折行，最多两行，垂直居中）。
func headerRow(p *gofpdf.Fpdf, widths []float64, cells []string, h float64) {
	p.SetFont("noto", "B", 8.5)
	x0 := margin
	y0 := p.GetY()
	p.SetFillColor(oliveHeader[0], oliveHeader[1], oliveHeader[2])
	for i, cell := range cells {
		x := x0
		for j := 0; j < i; j++ {
			x += widths[j]
		}
		p.Rect(x, y0, widths[i], h, "DF")
		lines := wrapText(p, cell, widths[i]-1.5)
		if len(lines) > 2 {
			lines = lines[:2]
		}
		lh := 4.2
		cy := y0 + (h-float64(len(lines))*lh)/2
		for _, ln := range lines {
			p.SetXY(x, cy)
			p.CellFormat(widths[i], lh, ln, "", 0, "C", false, 0, "")
			cy += lh
		}
	}
	p.SetXY(x0, y0+h)
}

// dataRow 数据行（单行、细黑线边框；文本超长截断）。
func dataRow(p *gofpdf.Fpdf, widths []float64, cells []string, h, size float64) {
	p.SetFont("noto", "", size)
	for i, cell := range cells {
		ln := 0
		if i == len(cells)-1 {
			ln = 1
		}
		p.CellFormat(widths[i], h, trunc(p, cell, widths[i]), "1", ln, "C", false, 0, "")
	}
}

// registerImage 加载并注册图片，返回等比缩放后的尺寸（受 maxW/maxH 约束）；失败 ok=false。
func registerImage(p *gofpdf.Fpdf, loader func(string) ([]byte, string, error), key, uniq string, maxW, maxH float64) (name string, w, h float64, ok bool) {
	if loader == nil || key == "" {
		return "", 0, 0, false
	}
	data, imgType, err := loader(key)
	if err != nil {
		return "", 0, 0, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width == 0 {
		return "", 0, 0, false
	}
	name = uniq
	p.RegisterImageOptionsReader(name, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: true}, bytes.NewReader(data))
	if p.Err() {
		p.ClearError() // 单张失败不影响整体生成
		return "", 0, 0, false
	}
	w = maxW
	h = w * float64(cfg.Height) / float64(cfg.Width)
	if h > maxH {
		h = maxH
		w = h * float64(cfg.Width) / float64(cfg.Height)
	}
	return name, w, h, true
}

// ========== 封面 ==========

func renderCover(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()

	// 左上：项目名称 / 检查月份 两行表格
	p.SetFont("noto", "B", 12)
	x0, y0 := 25.0, 28.0
	labelW, valueW, rowH := 26.0, 62.0, 12.0
	rows := [][2]string{{"项目名称", d.CommunityName}, {"检查月份", periodCN(d.Period)}}
	for i, r := range rows {
		y := y0 + float64(i)*rowH
		p.SetXY(x0, y)
		p.SetFont("noto", "B", 12)
		p.CellFormat(labelW, rowH, r[0], "1", 0, "L", false, 0, "")
		p.SetFont("noto", "", 12)
		p.CellFormat(valueW, rowH, r[1], "1", 1, "L", false, 0, "")
	}

	// 居中大标题
	p.SetY(112)
	p.SetFont("noto", "B", 25)
	p.CellFormat(contentW, 15, "物业消防设施（器材类）月度", "", 1, "C", false, 0, "")
	p.CellFormat(contentW, 15, "巡 检 报 告", "", 1, "C", false, 0, "")

	// 设施类别 / 管理单位 / 报告日期（下划线填空式）
	infoY := 178.0
	lineH := 13.0
	labelW2, valueW2 := 30.0, 78.0
	blockX := (pageW - labelW2 - valueW2) / 2
	typeLine := strings.Join(d.TypeNames, "/")
	dateLine := ""
	if d.Approved {
		dateLine = dateCN(d.ApproveDate)
	}
	infos := []string{typeLine, d.CompanyName, dateLine}
	labels := []string{"设施类别：", "管理单位：", "报告日期："}
	for i := range labels {
		y := infoY + float64(i)*lineH
		p.SetXY(blockX, y)
		p.SetFont("noto", "", 13)
		p.CellFormat(labelW2, lineH-4, labels[i], "", 0, "L", false, 0, "")
		p.CellFormat(valueW2, lineH-4, infos[i], "B", 1, "C", false, 0, "")
	}
	p.SetFont("noto", "", 10)
	p.SetXY(blockX, infoY+3*lineH+2)
	p.CellFormat(labelW2+valueW2, 6, "（加盖公章）", "", 1, "C", false, 0, "")

	// 公章：仅终审通过且配置了公章图时嵌入（约 35mm 宽，覆盖在日期区域上方）
	if d.Approved && d.SealKey != "" {
		if name, w, h, ok := registerImage(p, d.ImageLoader, d.SealKey, "seal", 35, 35); ok {
			p.ImageOptions(name, blockX+labelW2+valueW2-w+6, infoY+2*lineH-h/2+1, w, h, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
		}
	}
}

// ========== 目录 ==========

func renderTOC(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	p.SetY(32)
	p.SetFont("noto", "B", 16)
	p.CellFormat(contentW, 10, "目      录", "", 1, "C", false, 0, "")
	p.Ln(8)

	p.SetFont("noto", "", 13)
	entries := []struct {
		text   string
		indent float64
	}{
		{"1. 报告说明", 0},
		{"2. 本月检查工作概述", 0},
		{"3. 本月检查汇总表", 0},
		{"4. 分项检查明细", 0},
	}
	for i, dt := range d.Details {
		entries = append(entries, struct {
			text   string
			indent float64
		}{fmt.Sprintf("4.%d %s巡检明细表", i+1, dt.TypeName), 12})
	}
	entries = append(entries,
		struct {
			text   string
			indent float64
		}{"5. 现场照片", 0},
		struct {
			text   string
			indent float64
		}{"6. 隐患问题清单及整改台账", 0},
		struct {
			text   string
			indent float64
		}{"7. 本月典型整改事项", 0},
		struct {
			text   string
			indent float64
		}{"8. 附件清单", 0},
		struct {
			text   string
			indent float64
		}{"9. 签字审批栏", 0},
	)
	for _, e := range entries {
		p.SetX(margin + 25 + e.indent)
		p.CellFormat(contentW-25-e.indent, 9.5, e.text, "", 1, "L", false, 0, "")
	}
}

// ========== 说明（报告说明） ==========

func renderIntro(p *gofpdf.Fpdf, _ MonthlyReportData) {
	p.AddPage()
	p.SetY(30)
	p.SetFont("noto", "B", 15)
	p.CellFormat(contentW, 10, "说      明", "", 1, "C", false, 0, "")
	p.Ln(6)

	boxX, boxW := margin+8.0, contentW-16.0
	startY := p.GetY()
	p.SetX(boxX + 6)

	sections := []struct {
		title string
		body  []string
	}{
		{"1.编制依据", []string{
			"本报告依据《消防安全管理规定》《物业服务台账管理标准》编制，为物业服务企业每月对园区单体消防器材全覆盖巡检形成的正式归档文件，完整记录器材运行状况、安全隐患及整改闭环全过程，可用于消防、住建、街道社区各级主管部门核查。",
		}},
		{"2.执行主体与职责", []string{
			"（1）巡检人：现场实地逐项检查，使用 NFC 标签点位打卡，现场拍摄设备照片，同步上传系统，如实记录设备状态、现场隐患；对轻微隐患当场处置，无法现场整改的拍照登记上报。",
			"（2）安全负责人：统筹月度巡检计划，核对全部巡检数据、影像资料，跟踪隐患整改进度，复查整改完成点位，审核巡检台账真实性。",
			"（3）物业经理：最终审批月度巡检报告，统筹整改资源，对园区消防安全管理负总责，定期抽查巡检完成质量。",
		}},
		{"3.巡检方式与核验标准", []string{
			"采用 NFC 点位打卡+人工现场核查+AI 图像识别三重核验；巡检人员现场实拍设备，系统自动留存影像、设备编号、压力、有效期等数据，资料加密留痕、不可篡改，全程可追溯。",
		}},
		{"4.巡检覆盖范围", []string{
			"园区室内外消火栓、公共区域全部灭火器、楼道应急照明灯、疏散指示标志、防火门、防火卷帘、消防疏散通道，实现园区全域、无死角覆盖。",
		}},
		{"5.归档与附件管理", []string{
			"所有巡检实拍图、AI 核验截图、隐患整改前后对比照片，按器材唯一编号分类存入电子档案；纸质报告签字盖章后与电子资料同步留存，长期保管备查。",
		}},
	}
	textW := boxW - 12
	for _, sec := range sections {
		p.SetX(boxX + 6)
		p.SetFont("noto", "B", 11.5)
		p.CellFormat(textW, 8, sec.title, "", 1, "L", false, 0, "")
		for _, body := range sec.body {
			p.SetFont("noto", "", 11)
			lines := wrapText(p, body, textW-8)
			for i, ln := range lines {
				p.SetX(boxX + 6)
				if i == 0 {
					p.CellFormat(8, 7, "", "", 0, "L", false, 0, "")
					p.CellFormat(textW-8, 7, ln, "", 1, "L", false, 0, "")
				} else {
					p.CellFormat(textW, 7, ln, "", 1, "L", false, 0, "")
					// 换行后回到框内左边距
					p.SetX(boxX + 6)
				}
			}
		}
		p.Ln(1.5)
	}
	endY := p.GetY()
	p.Rect(boxX, startY-4, boxW, endY-startY+8, "D")
}

// ========== 2.检查工作概述 + 3.本月检查汇总表 ==========

func renderOverviewAndSummary(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	sectionTitle(p, "2.检查工作概述")
	p.Ln(1)
	paragraph(p, fmt.Sprintf("为落实消防安全常态化管控，夯实小区防火安全基础，本月由项目安保/工程巡检人员，在安全负责人统筹安排下，对%s园区全部消火栓、灭火器、应急照明、防火门等单体消防器材开展全覆盖月度巡检。", d.CommunityName), 12, 8, true)
	p.Ln(2)
	paragraph(p, "巡检全程使用 NFC 感应点位确认，现场核对器材外观、压力、配件、通道环境，同步拍摄影像上传管理系统；系统自动汇总巡检完成率、故障数量、整改数据，形成标准化台账。所有巡检记录、影像资料加密存档，满足行业及主管部门迎检要求。", 12, 8, true)
	p.Ln(6)

	sectionTitle(p, "3.本月检查汇总表")
	p.Ln(1)
	widths := []float64{30, 15, 19, 20, 17, 17, 20, 42}
	header := []string{"设施类别", "总数", "正常完好", "巡检完成率", "存在问题", "整改完毕", "整改完成率", "备注"}
	headerRow(p, widths, header, 10)
	if len(d.Summary) == 0 {
		dataRow(p, widths, []string{"本月无巡检点位", "-", "-", "-", "-", "-", "-", "-"}, 8, 9)
		return
	}
	for _, r := range d.Summary {
		dataRow(p, widths, []string{
			r.TypeName, itoa(r.Total), itoa(r.Normal), pctText(r.InspectRate),
			itoa(r.Problems), itoa(r.Rectified), pctText(r.RectifyRate), r.Remark,
		}, 8, 9)
	}
}

// ========== 4.分项巡检明细 ==========

// 明细表固定列宽
const (
	detailColIdx   = 10.0 // 序号
	detailColLoc   = 30.0 // 位置/编号
	detailColUser  = 20.0 // 巡检人
	detailColTime  = 30.0 // 巡检时间
	detailMaxItems = 7    // 检查项列数上限
	detailRowH     = 7.0
)

// detailItemWidth 检查项列宽：按列数均分（8~12mm），问题说明列占据剩余宽度。
func detailItemWidth(n int) float64 {
	w := 60.0 / float64(n)
	if w > 12 {
		w = 12
	}
	if w < 8 {
		w = 8
	}
	return w
}

// shortItemLabel 检查项列头缩写（≤3 字取全名，否则取前 2 字，竖排展示）。
func shortItemLabel(name string) string {
	rs := []rune(name)
	if len(rs) <= 3 {
		return name
	}
	return string(rs[:2])
}

// drawDetailHeader 明细表表头：检查项列逐字竖排，其余列单行居中。
func drawDetailHeader(p *gofpdf.Fpdf, items []string, itemW, problemW float64) {
	widths, labels := detailColumns(items, itemW, problemW)
	// 表头高度：竖排最多字数决定
	maxStack := 1
	for _, it := range items {
		if n := len([]rune(shortItemLabel(it))); n > maxStack {
			maxStack = n
		}
	}
	headerH := float64(maxStack)*4.6 + 4
	if headerH < 9 {
		headerH = 9
	}
	x0 := margin
	y0 := p.GetY()
	p.SetFillColor(oliveHeader[0], oliveHeader[1], oliveHeader[2])
	for i, label := range labels {
		x := x0
		for j := 0; j < i; j++ {
			x += widths[j]
		}
		p.Rect(x, y0, widths[i], headerH, "DF")
		p.SetFont("noto", "B", 9)
		if i >= 2 && i < 2+len(items) {
			// 检查项列：逐字竖排居中
			chars := []rune(label)
			totalH := float64(len(chars)) * 4.6
			cy := y0 + (headerH-totalH)/2
			for _, ch := range chars {
				p.SetXY(x, cy)
				p.CellFormat(widths[i], 4.6, string(ch), "", 0, "C", false, 0, "")
				cy += 4.6
			}
		} else {
			p.SetXY(x, y0+(headerH-6)/2)
			p.CellFormat(widths[i], 6, label, "", 0, "C", false, 0, "")
		}
	}
	p.SetXY(x0, y0+headerH)
}

// detailColumns 明细表列宽与列头。
func detailColumns(items []string, itemW, problemW float64) (widths []float64, labels []string) {
	widths = []float64{detailColIdx, detailColLoc}
	labels = []string{"序号", "位置/编号"}
	for _, it := range items {
		widths = append(widths, itemW)
		labels = append(labels, shortItemLabel(it))
	}
	widths = append(widths, problemW, detailColUser, detailColTime)
	labels = append(labels, "问题说明", "巡检人", "巡检时间")
	return widths, labels
}

// renderDetails 每个有点位的点位类型一张明细表（跨页重复表头）。
func renderDetails(p *gofpdf.Fpdf, d MonthlyReportData) {
	ensureSpace(p, 40)
	sectionTitle(p, "4. 分项巡检明细")
	p.Ln(1)
	for ti, dt := range d.Details {
		ensureSpace(p, 34)
		p.SetFont("noto", "B", 12)
		p.CellFormat(contentW, 8, fmt.Sprintf("4.%d %s巡检明细表", ti+1, dt.TypeName), "", 1, "L", false, 0, "")
		p.Ln(1)

		items := dt.Items
		if len(items) > detailMaxItems {
			items = items[:detailMaxItems]
		}
		itemW := detailItemWidth(len(items))
		problemW := contentW - detailColIdx - detailColLoc - detailColUser - detailColTime - float64(len(items))*itemW
		widths, _ := detailColumns(items, itemW, problemW)

		drawDetailHeader(p, items, itemW, problemW)
		if len(dt.Rows) == 0 {
			p.SetFont("noto", "", 9)
			p.CellFormat(contentW, detailRowH, "本月无巡检记录", "1", 1, "C", false, 0, "")
		}
		_, pageH := p.GetPageSize()
		breakY := pageH - 18
		for i, row := range dt.Rows {
			if p.GetY()+detailRowH > breakY {
				p.AddPage()
				drawDetailHeader(p, items, itemW, problemW)
			}
			cells := []string{itoa(i + 1), row.Location}
			for j := range items {
				mark := ""
				if j < len(row.Marks) {
					mark = row.Marks[j]
				}
				cells = append(cells, mark)
			}
			cells = append(cells, row.Problem, row.Inspector, row.Time)
			dataRow(p, widths, cells, detailRowH, 9)
		}
		// 表下注释
		if dt.Note != "" {
			p.SetFont("noto", "", 8.5)
			for _, ln := range wrapText(p, dt.Note, contentW) {
				p.CellFormat(contentW, 5.5, ln, "", 1, "L", false, 0, "")
			}
		}
		p.Ln(5)
	}
}

// ========== 5.现场照片 ==========

const (
	photoCols    = 3                 // 每行照片数
	photoCellW   = contentW / photoCols // 60mm
	photoImgMaxW = 54.0
	photoImgMaxH = 40.0
	photoCapH    = 5.0 // 标注行高
)

// renderPhotos 现场照片章节：按打卡记录分组横排（每行 3 张），逐项照片带项名小标注，记录级照片标"全景"。
func renderPhotos(p *gofpdf.Fpdf, d MonthlyReportData) {
	ensureSpace(p, 40)
	sectionTitle(p, "5.现场照片")
	p.Ln(1)
	if len(d.PhotoGroups) == 0 {
		paragraph(p, "本月无现场照片。", 12, 8, false)
		p.Ln(4)
		return
	}
	_, pageH := p.GetPageSize()
	breakY := pageH - 18
	for gi, g := range d.PhotoGroups {
		// 组标题与首行照片不拆开
		if p.GetY()+8+photoImgMaxH+photoCapH > breakY {
			p.AddPage()
		}
		p.SetFont("noto", "B", 10)
		p.CellFormat(contentW, 7, fmt.Sprintf("%d. %s", gi+1, trunc(p, g.Title, contentW)), "", 1, "L", false, 0, "")
		for row := 0; row*photoCols < len(g.Cells); row++ {
			end := (row + 1) * photoCols
			if end > len(g.Cells) {
				end = len(g.Cells)
			}
			cells := g.Cells[row*photoCols : end]
			// 先注册本行图片以确定行高（取本行最大图高）
			type reg struct {
				name string
				w, h float64
				ok   bool
			}
			regs := make([]reg, len(cells))
			rowH := 0.0
			for i, cell := range cells {
				n, w, h, ok := registerImage(p, d.ImageLoader, cell.Key, fmt.Sprintf("site-%d-%d-%d", gi, row, i), photoImgMaxW, photoImgMaxH)
				regs[i] = reg{n, w, h, ok}
				if ok && h > rowH {
					rowH = h
				}
			}
			if rowH == 0 {
				rowH = photoImgMaxH * 0.6 // 本行图片全部加载失败的保底行高
			}
			if p.GetY()+rowH+photoCapH > breakY {
				p.AddPage()
			}
			x0 := margin
			y0 := p.GetY()
			for i, cell := range cells {
				x := x0 + float64(i)*photoCellW
				rg := regs[i]
				if rg.ok {
					p.ImageOptions(rg.name, x+(photoCellW-rg.w)/2, y0, rg.w, rg.h, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
				} else {
					p.Rect(x+2, y0, photoCellW-4, rowH, "D")
					p.SetXY(x, y0+rowH/2-2)
					p.SetFont("noto", "", 8)
					p.CellFormat(photoCellW, 4, "（照片缺失）", "", 0, "C", false, 0, "")
				}
				p.SetXY(x, y0+rowH+0.5)
				p.SetFont("noto", "", 8)
				p.CellFormat(photoCellW, photoCapH-1, trunc(p, cell.Label, photoCellW-4), "", 0, "C", false, 0, "")
			}
			p.SetXY(x0, y0+rowH+photoCapH+1.5)
		}
		p.Ln(2)
	}
}

// ========== 6.隐患问题清单及整改台账 ==========

func renderLedger(p *gofpdf.Fpdf, rows []LedgerRow) {
	ensureSpace(p, 40)
	sectionTitle(p, "6.问题清单及整改台账")
	p.Ln(1)
	widths := []float64{8, 28, 14, 25, 16, 26, 12, 18, 19, 14}
	header := []string{"序号", "位置/编号", "设施类型", "问题描述", "整改责任人", "完成时间", "整改状态", "复查人", "复查日期", "备注"}
	headerRow(p, widths, header, 11)
	if len(rows) == 0 {
		dataRow(p, []float64{contentW}, []string{"本月无隐患问题"}, 8, 9)
		return
	}
	_, pageH := p.GetPageSize()
	breakY := pageH - 18
	for i, r := range rows {
		if p.GetY()+8 > breakY {
			p.AddPage()
			headerRow(p, widths, header, 11)
		}
		dataRow(p, widths, []string{
			itoa(i + 1), r.Location, r.TypeName, r.Problem, r.Assignee,
			r.FinishTime, r.Status, r.Reviewer, r.ReviewDate, r.Remark,
		}, 8, 8)
	}
	p.Ln(4)
}

// ========== 7.本月典型整改事项 ==========

func renderTypical(p *gofpdf.Fpdf, items []TypicalItem) {
	ensureSpace(p, 40)
	sectionTitle(p, "7.本月典型整改事项")
	p.Ln(1)
	if len(items) == 0 {
		paragraph(p, "本月无整改事项。", 12, 8, false)
	} else {
		for i, it := range items {
			text := fmt.Sprintf("%d. %s：%s；", i+1, it.Problem, it.Result)
			paragraph(p, text, 12, 8, false)
			p.Ln(1)
		}
	}
	p.Ln(1)
	paragraph(p, "所有隐患点位均拍摄整改前、整改后对比照片，按设备编号归档，经安全负责人复查确认合格后方可闭环。", 12, 8, true)
	p.Ln(4)
}

// ========== 8.附件清单 ==========

func renderAttachments(p *gofpdf.Fpdf) {
	ensureSpace(p, 46)
	sectionTitle(p, "8.附件清单：")
	p.Ln(1)
	items := []string{
		"1、消防器材点位总台账（含唯一编号、具体位置、基础参数，系统电子归档）；",
		"2、正常点位 AI 识别巡检实拍照片（按编号分类归档）；",
		"3、问题点位整改前后对比影像资料；",
		"4、月度隐患整改闭环明细台账。",
	}
	for _, it := range items {
		paragraph(p, it, 12, 8, false)
		p.Ln(1)
	}
	p.Ln(4)
}

// ========== 9.签字审批栏 ==========

func renderSignBar(p *gofpdf.Fpdf, d MonthlyReportData) {
	ensureSpace(p, 110)
	sectionTitle(p, "9.签字审批栏")
	p.Ln(1)

	const colW = 60.0
	headerH := 16.0
	// 巡检人栏多人签名竖排：按人数加高栏体
	bodyH := 58.0
	if n := len(d.InspectorSigns); n > 1 {
		if h := 10.0 + float64(n)*22.0; h > bodyH {
			bodyH = h
		}
	}
	x0 := margin
	y0 := p.GetY()

	titles := [][2]string{
		{"巡检人签字", "（现场执行）"},
		{"安全负责人签字", "（审核复查）"},
		{"物业经理签字", "（审批）"},
	}
	p.SetFillColor(oliveHeader[0], oliveHeader[1], oliveHeader[2])
	for i, t := range titles {
		x := x0 + float64(i)*colW
		p.Rect(x, y0, colW, headerH, "DF")
		p.SetXY(x, y0+2.5)
		p.SetFont("noto", "B", 12)
		p.CellFormat(colW, 6.5, t[0], "", 0, "C", false, 0, "")
		p.SetXY(x, y0+9.5)
		p.SetFont("noto", "", 10)
		p.CellFormat(colW, 5, t[1], "", 0, "C", false, 0, "")
	}

	// 栏体边框
	y1 := y0 + headerH
	for i := 0; i < 3; i++ {
		p.Rect(x0+float64(i)*colW, y1, colW, bodyH, "D")
	}

	// 栏内签名内容：签名图（无则打印姓名）+ 签字日期；底部"年 月 日"
	renderSignCell(p, d, 0, x0, y1, colW, bodyH-12, d.InspectorSigns)
	renderSignCell(p, d, 1, x0+colW, y1, colW, bodyH-12, []SignInfo{d.Supervisor})
	renderSignCell(p, d, 2, x0+2*colW, y1, colW, bodyH-12, []SignInfo{d.Manager})
	for i := 0; i < 3; i++ {
		var dateLine string
		switch i {
		case 0:
			dateLine = signBarDate(d.InspectorSigns)
		case 1:
			dateLine = dateCN(d.Supervisor.Time)
		default:
			dateLine = dateCN(d.Manager.Time)
		}
		if dateLine == "" {
			dateLine = "          年        月        日"
		}
		p.SetXY(x0+float64(i)*colW, y1+bodyH-10)
		p.SetFont("noto", "B", 10)
		p.CellFormat(colW, 6, dateLine, "", 0, "C", false, 0, "")
	}
	p.SetXY(x0, y1+bodyH)
	p.Ln(8)

	// 页尾右对齐落款
	company := d.CompanyName
	if company == "" {
		company = "XX物业服务中心"
	}
	footerDate := dateCN(d.ApproveDate)
	if footerDate == "" {
		footerDate = "        年    月    日"
	}
	p.SetFont("noto", "", 12)
	p.CellFormat(contentW, 8, company, "", 1, "R", false, 0, "")
	p.CellFormat(contentW, 8, footerDate, "", 1, "R", false, 0, "")
}

// signBarDate 巡检人栏底部日期：取首位已签巡检员的签字日期。
func signBarDate(signs []SignInfo) string {
	for _, s := range signs {
		if s.Time != "" {
			return dateCN(s.Time)
		}
	}
	return ""
}

// renderSignCell 签字栏单栏内容：多人时竖向排多个签名+各自日期。
func renderSignCell(p *gofpdf.Fpdf, d MonthlyReportData, col int, x, y, w, availH float64, signs []SignInfo) {
	curY := y + 4
	for idx, s := range signs {
		if s.Name == "" && s.Time == "" {
			continue // 未签字留白
		}
		uniq := fmt.Sprintf("sign-%d-%d", col, idx)
		if name, iw, ih, ok := registerImage(p, d.ImageLoader, s.SignatureKey, uniq, 34, 13); ok {
			p.ImageOptions(name, x+(w-iw)/2, curY, iw, ih, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
			curY += ih + 1
		} else {
			// 无签名图回退打印姓名
			p.SetXY(x, curY+2)
			p.SetFont("noto", "B", 13)
			p.CellFormat(w, 8, s.Name, "", 0, "C", false, 0, "")
			curY += 11
		}
		if s.Time != "" {
			p.SetXY(x, curY)
			p.SetFont("noto", "", 8)
			p.CellFormat(w, 4.5, shortDateTime(s.Time), "", 0, "C", false, 0, "")
			curY += 6
		}
		curY += 2
		if curY > y+availH {
			break
		}
	}
}

// shortDateTime "2026-08-09 14:30:00" → "2026-08-09"。
func shortDateTime(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
