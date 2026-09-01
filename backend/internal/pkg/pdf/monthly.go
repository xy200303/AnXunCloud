// Package pdf 月度巡检报告 PDF 生成（gofpdf + 内嵌 Noto Sans SC 中文字体）。
// 版式 1:1 复刻《物业消防设施（器材类）月度巡检报告》v2 模板：
// 封面 / 目录 / 报告编制说明 / 签字审批栏 / 本月检查汇总表 / 分项巡检明细 / 问题清单及整改台账 / 附件：分项检查照片。
package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	// 解码 JPEG 图片尺寸（DecodeConfig）
	_ "image/jpeg"
	"image/png"
	"strings"
	"time"

	"github.com/phpdave11/gofpdf"
)

// 表头橄榄绿底（取自模板）
var oliveHeader = [3]int{190, 185, 165}

const (
	pageW    = 210.0
	margin   = 15.0
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
	Rectified   int     // 整改完毕（已复核异常打卡数）
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
	TypeCode string   // 点位类型编码（用于固定消防表格分类，避免仅靠中文名判断）
	Items    []string // 检查项列（该类别检查项模板）
	Note     string   // 表格下方"注：检查标准……"
	Rows     []DetailRow
}

// LedgerRow 问题清单及整改台账行（v2 模板：一条异常打卡记录一行）。
type LedgerRow struct {
	Date          string   // 日期（异常打卡日）
	Problem       string   // 故障/问题描述（异常备注）
	ProblemPhotos []string // 问题照片 file_key（渲染取首张）
	FixText       string   // 处理情况（异常打卡的复核结论）
	FixPhotos     []string // 整改后照片 file_key（渲染取首张）
	Inspector     string   // 检查人（打卡巡检员）
}

// PhotoCell 现场照片单元（标注 + 图片 file_key）。
type PhotoCell struct {
	Label string // 小标注：检查项名（逐项照片）或"全景"（记录级照片）
	Key   string // 图片 file_key（经 ImageLoader 加载）
}

// PhotoGroup 分项检查照片分组（v2：一个设施类别一组）。
type PhotoGroup struct {
	Title string // 设施类别中文名（渲染为「N.{类别}巡检照片」）
	Cells []PhotoCell
}

// MonthlyReportData 月度巡检报告 PDF 数据。
type MonthlyReportData struct {
	CommunityName string   // 项目名称（小区名）
	Period        string   // YYYY-MM
	TitleLine     string   // 封面大标题首行（空回落「物业设施月度」；由巡查类型推导）
	CompanyName   string   // 管理单位落款（空则封面留白 / 页尾 XX物业服务中心）
	Approved      bool     // 已终审（公章与报告日期仅终审后展示）
	ApproveDate   string   // 终审日期 YYYY-MM-DD
	SealKey       string   // 公章图 file_key（仅 Approved 时嵌入）
	TypeNames     []string // 设施类别（该小区有点位的类型中文名）
	Summary       []SummaryRow
	Details       []DetailTable
	PhotoGroups   []PhotoGroup // 附件：分项检查照片（按设施类别分组）
	Ledger        []LedgerRow
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

	renderLedgerMonthly(p, data)

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

// wrapTextNoStart 行首禁止出现的标点（避头尾：闭门点/连字符类不顶格）
const wrapTextNoStart = "，。；、！？：）】》”’…—·"

// wrapTextNoEnd 行尾禁止出现的标点（开门点留在行尾则移到下行）
const wrapTextNoEnd = "（【《“‘"

// wrapText 按列宽把文本折行（按 rune 测宽；\n 强制换行；中文标点避头尾）。
func wrapText(p *gofpdf.Fpdf, text string, w float64) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		var cur []rune
		for _, r := range para {
			if p.GetStringWidth(string(cur)+string(r)) > w && len(cur) > 0 {
				// 闭门点不允许出现在下行行首：允许其悬挂在当前行尾（略出界可接受）
				if strings.ContainsRune(wrapTextNoStart, r) {
					cur = append(cur, r)
					continue
				}
				// 开门点不允许留在当前行行尾：移到下一行
				if strings.ContainsRune(wrapTextNoEnd, cur[len(cur)-1]) {
					lines = append(lines, string(cur[:len(cur)-1]))
					cur = []rune{cur[len(cur)-1], r}
					continue
				}
				lines = append(lines, string(cur))
				cur = nil
			}
			cur = append(cur, r)
		}
		lines = append(lines, string(cur))
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

// trunc 超出列宽的文本截断（rune 安全，截断后补省略号；避免 Cell 溢出与生硬截断）。
func trunc(p *gofpdf.Fpdf, s string, w float64) string {
	if p.GetStringWidth(s) <= w-2 {
		return s
	}
	rs := []rune(s)
	for len(rs) > 1 && p.GetStringWidth(string(rs)+"…") > w-2 {
		rs = rs[:len(rs)-1]
	}
	return string(rs) + "…"
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
// PNG 统一重新编码后再嵌入：带 alpha 的（手写签名/公章）先压平白底；即使像素全不透明，
// RGBA 色型的 PNG 也会让 gofpdf 生成带 SMask 的图像，而 pdf.js 在部分 WebView 中无法完成
// SMask 解码（控制台刷 "Dependent image isn't ready yet"），表现为图片不渲染。
// Go png 编码器对不透明图像自动输出真彩色（无 alpha 通道），重编码后不再产生 SMask。
func registerImage(p *gofpdf.Fpdf, loader func(string) ([]byte, string, error), key, uniq string, maxW, maxH float64) (name string, w, h float64, ok bool) {
	if loader == nil || key == "" {
		return "", 0, 0, false
	}
	data, imgType, err := loader(key)
	if err != nil {
		return "", 0, 0, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 8 || cfg.Height < 8 {
		// 解码失败或异常小图（如 1x1 占位图）按缺失处理，避免拉伸成色块
		return "", 0, 0, false
	}
	if imgType == "PNG" {
		if img, _, derr := image.Decode(bytes.NewReader(data)); derr == nil {
			out := img
			if o, is := img.(interface{ Opaque() bool }); is && !o.Opaque() {
				flat := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
				draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
				draw.Draw(flat, flat.Bounds(), img, image.Point{}, draw.Over)
				out = flat
			}
			var buf bytes.Buffer
			if png.Encode(&buf, out) == nil {
				data = buf.Bytes()
			}
		}
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

	// 居中大标题（首行可配，次行固定）
	titleLine := d.TitleLine
	if titleLine == "" {
		titleLine = "物业设施月度"
	}
	p.SetY(112)
	p.SetFont("noto", "B", 25)
	p.CellFormat(contentW, 15, titleLine, "", 1, "C", false, 0, "")
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

// ========== 附件：分项检查照片（电子档） ==========

const (
	photoCols    = 3                    // 每行照片数
	photoCellW   = contentW / photoCols // 60mm
	photoImgMaxW = 54.0
	photoImgMaxH = 40.0
	photoCapH    = 5.0 // 标注行高
)

// renderPhotoAppendix 分项检查照片附件：按设施类别分组横排（每行 3 张），逐项照片带项名小标注。
func renderPhotoAppendix(p *gofpdf.Fpdf, d MonthlyReportData) {
	ensureSpace(p, 40)
	sectionTitle(p, "附件：分项检查照片（电子档）")
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
		p.CellFormat(contentW, 7, fmt.Sprintf("%d.%s巡检照片", gi+1, g.Title), "", 1, "L", false, 0, "")
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
					// 图片在单元格行高内垂直居中，保证行内多图底部/标注对齐（防横竖图混排错位）
					p.ImageOptions(rg.name, x+(photoCellW-rg.w)/2, y0+(rowH-rg.h)/2, rg.w, rg.h, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
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

// ========== 5.问题清单及整改台账 ==========

// v2 模板列：序号 / 日期 / 故障·问题（文字+照片）/ 处理情况（文字+照片）/ 检查人。
const (
	ledgerColIdx  = 10.0
	ledgerColDate = 20.0
	ledgerColProb = 66.0
	ledgerColFix  = 64.0
	ledgerColUser = 20.0
	ledgerImgMaxH = 20.0 // 单元格照片最大高度
	ledgerLineH   = 4.6  // 单元格文字行高
)

var ledgerWidths = []float64{ledgerColIdx, ledgerColDate, ledgerColProb, ledgerColFix, ledgerColUser}
var ledgerHeader = []string{"序号", "日期", "故障/问题", "处理情况", "检查人"}

// ledgerPhotoCell 台账文字+照片单元格内容高度测算与绘制。
// 返回内容高度（文字行 + 照片），照片取首张。
func ledgerCellContent(p *gofpdf.Fpdf, d MonthlyReportData, text string, photos []string, w float64, uniq string) (lines []string, imgName string, imgW, imgH float64) {
	lines = wrapText(p, trunc(p, text, w*3), w-4)
	if len(lines) > 3 {
		lines = lines[:3]
	}
	if len(photos) > 0 {
		if name, iw, ih, ok := registerImage(p, d.ImageLoader, photos[0], uniq, w-10, ledgerImgMaxH); ok {
			imgName, imgW, imgH = name, iw, ih
		}
	}
	return lines, imgName, imgW, imgH
}

// signBarDate 签字栏日期：取首个已签时间的日期部分。
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

// ========== 台账版月度报告（甲方样稿骨架，分项按点位类型数据驱动） ==========
//
// 八页顺序：封面 → 目录 → 说明+签字审批栏 → 本月检查汇总表 → 分项巡检明细（每点位类型一张，
// 检查列=该类型点位所绑检查项模板）→ 问题清单及整改台账 → 附件：分项检查照片。
// 动态数据只填入固定行格，超出固定行数时新增同款续页，绝不改变既有行高或挤压其他单元格。

// renderLedgerMonthly 台账版入口。
func renderLedgerMonthly(p *gofpdf.Fpdf, d MonthlyReportData) {
	renderCover(p, d)
	renderLedgerTOC(p, d)
	renderLedgerIntroAndSign(p, d)
	renderLedgerSummary(p, d)
	for idx, table := range d.Details {
		renderLedgerDetail(p, d, idx, table)
	}
	renderIssueLedger(p, d)
	// 模板仅将现场照片列为电子附件；没有照片时不额外生成空白附件页。
	if len(d.PhotoGroups) > 0 {
		renderPhotoAppendix(p, d)
	}
}

func renderLedgerTOC(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	p.SetY(38)
	p.SetFont("noto", "B", 18)
	p.CellFormat(contentW, 10, "目      录", "", 1, "C", false, 0, "")
	p.Ln(12)
	p.SetFont("noto", "", 12)
	entries := []string{"1. 报告编制说明", "2. 签字审批栏", "3. 本月检查汇总表", "4. 分项检查明细"}
	for idx, table := range d.Details {
		entries = append(entries, "    "+ledgerDetailTitle(idx, table.TypeName))
	}
	entries = append(entries, "5. 问题清单及整改台账", "附件：分项检查照片（电子档）")
	for _, entry := range entries {
		p.SetX(margin + 28)
		p.CellFormat(contentW-28, 11, entry, "", 1, "L", false, 0, "")
	}
}

func renderLedgerIntroAndSign(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	p.SetY(18)
	p.SetFont("noto", "B", 15)
	p.CellFormat(contentW, 8, "1.报告编制说明", "", 1, "L", false, 0, "")
	p.Ln(2)
	coverage := strings.Join(d.TypeNames, "、")
	if coverage == "" {
		coverage = "各巡检点位"
	}
	intro := []string{
		"本报告依据相关规定与物业服务合同编制，记录物业服务人对园区设施开展月度巡检、隐患登记和整改跟踪的情况。",
		"巡检人员使用系统完成点位打卡和现场拍照；安全负责人核对巡检数据、影像资料和整改进度；物业经理对报告进行审批。所有资料留痕归档，可追溯核查。",
		"检查覆盖" + coverage + "等设施与区域。",
	}
	boxY := p.GetY()
	for _, text := range intro {
		p.SetFont("noto", "", 9.5)
		for _, line := range wrapText(p, text, contentW-12) {
			p.SetX(margin + 6)
			p.CellFormat(contentW-12, 5.6, line, "", 1, "L", false, 0, "")
		}
		p.Ln(1)
	}
	p.Rect(margin, boxY-2, contentW, p.GetY()-boxY+3, "D")
	p.SetY(113)
	renderLedgerSignTable(p, d)
}

func renderLedgerSignTable(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.SetFont("noto", "B", 14)
	p.CellFormat(contentW, 8, "2.签字审批栏", "", 1, "L", false, 0, "")
	p.Ln(2)
	const cellW, headerH, bodyH = 60.0, 12.0, 62.0
	x0, y0 := margin, p.GetY()
	headers := []string{"巡检人签字\n（现场执行）", "安全负责人签字\n（审核复查）", "物业经理签字\n（审批）"}
	p.SetFillColor(oliveHeader[0], oliveHeader[1], oliveHeader[2])
	for idx, header := range headers {
		x := x0 + float64(idx)*cellW
		p.Rect(x, y0, cellW, headerH, "DF")
		lines := strings.Split(header, "\n")
		p.SetFont("noto", "B", 10)
		p.SetXY(x, y0+1.5)
		p.CellFormat(cellW, 4.5, lines[0], "", 0, "C", false, 0, "")
		p.SetFont("noto", "", 8)
		p.SetXY(x, y0+6.5)
		p.CellFormat(cellW, 4, lines[1], "", 0, "C", false, 0, "")
		p.Rect(x, y0+headerH, cellW, bodyH, "D")
	}
	renderSignCell(p, d, 0, x0, y0+headerH, cellW, bodyH-11, d.InspectorSigns)
	renderSignCell(p, d, 1, x0+cellW, y0+headerH, cellW, bodyH-11, []SignInfo{d.Supervisor})
	renderSignCell(p, d, 2, x0+cellW*2, y0+headerH, cellW, bodyH-11, []SignInfo{d.Manager})
	for idx, signs := range [][]SignInfo{d.InspectorSigns, {d.Supervisor}, {d.Manager}} {
		date := signBarDate(signs)
		if idx > 0 && len(signs) > 0 {
			date = dateCN(signs[0].Time)
		}
		if date == "" {
			date = "      年    月    日"
		}
		p.SetXY(x0+float64(idx)*cellW, y0+headerH+bodyH-8)
		p.SetFont("noto", "", 9)
		p.CellFormat(cellW, 5, date, "", 0, "C", false, 0, "")
	}
	p.SetY(y0 + headerH + bodyH)
}

// renderLedgerSummary 本月检查汇总表（行=点位类型分项；整改两列按甲方口径留空——系统暂无整改闭环数据）。
func renderLedgerSummary(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	p.SetY(22)
	p.SetFont("noto", "B", 15)
	p.CellFormat(contentW, 8, "3.本月检查汇总表", "", 1, "L", false, 0, "")
	p.Ln(2)
	widths := []float64{29, 14, 18, 20, 17, 17, 20, 45}
	headerRow(p, widths, []string{"设施类别", "总数", "正常完好", "巡检完成率", "存在问题", "整改完毕", "整改完成率", "备注"}, 12)
	for _, row := range d.Summary {
		dataRow(p, widths, []string{row.TypeName, itoa(row.Total), itoa(row.Normal), pctText(row.InspectRate), itoa(row.Problems), "", "", row.Remark}, 12, 9)
	}
}

// ledgerDetailTitle 分项明细表标题（4.N {点位类型}巡检明细表）。
func ledgerDetailTitle(idx int, typeName string) string {
	return fmt.Sprintf("4.%d %s巡检明细表", idx+1, typeName)
}

// drawLedgerDetailHeader 明细表头：序号/位置编号 + 检查项竖排窄列 + 问题说明/巡检人/巡检时间。
// 检查项列宽随项数均分，表头文字逐字竖排（对齐甲方样稿）。
func drawLedgerDetailHeader(p *gofpdf.Fpdf, items []string) []float64 {
	itemW := (contentW - 9 - 32 - 37 - 20 - 17) / float64(len(items))
	widths := []float64{9, 32}
	for range items {
		widths = append(widths, itemW)
	}
	widths = append(widths, 37, 20, 17)
	labels := []string{"序号", "位置/编号"}
	labels = append(labels, items...)
	labels = append(labels, "问题说明", "巡检人", "巡检时间")
	x0, y0, h := margin, p.GetY(), 17.0
	p.SetFillColor(oliveHeader[0], oliveHeader[1], oliveHeader[2])
	for idx, label := range labels {
		x := x0
		for before := 0; before < idx; before++ {
			x += widths[before]
		}
		p.Rect(x, y0, widths[idx], h, "DF")
		p.SetFont("noto", "B", 8)
		if idx >= 2 && idx < 2+len(items) {
			chars := []rune(label)
			cy := y0 + (h-float64(len(chars))*3.9)/2
			for _, char := range chars {
				p.SetXY(x, cy)
				p.CellFormat(widths[idx], 3.9, string(char), "", 0, "C", false, 0, "")
				cy += 3.9
			}
		} else {
			p.SetXY(x, y0+(h-4.5)/2)
			p.CellFormat(widths[idx], 4.5, label, "", 0, "C", false, 0, "")
		}
	}
	p.SetXY(x0, y0+h)
	return widths
}

// renderLedgerDetail 一张分项巡检明细表：检查列=该类型点位所绑检查项模板（table.Items），
// 行=点位当期最新打卡（table.Rows，Marks 与 Items 对齐：√正常/×异常/空未巡），表下检查标准注。
func renderLedgerDetail(p *gofpdf.Fpdf, d MonthlyReportData, idx int, table DetailTable) {
	rows := table.Rows
	if rows == nil {
		rows = []DetailRow{}
	}
	pageCount := (len(rows) + 8) / 9
	if pageCount == 0 {
		pageCount = 1
	}
	for page := 0; page < pageCount; page++ {
		p.AddPage()
		p.SetY(22)
		title := ledgerDetailTitle(idx, table.TypeName)
		if page > 0 {
			title += "（续）"
		}
		p.SetFont("noto", "B", 15)
		p.CellFormat(contentW, 8, title, "", 1, "L", false, 0, "")
		p.Ln(2)
		widths := drawLedgerDetailHeader(p, table.Items)
		start, end := page*9, min((page+1)*9, len(rows))
		for r := start; r < end; r++ {
			row := rows[r]
			cells := []string{itoa(r + 1), row.Location}
			cells = append(cells, row.Marks...)
			cells = append(cells, row.Problem, row.Inspector, row.Time)
			dataRow(p, widths, cells, 14.0, 8.5)
		}
		for empty := end - start; empty < 9; empty++ {
			dataRow(p, widths, make([]string, len(widths)), 14.0, 8.5)
		}
		p.Ln(2)
		p.SetFont("noto", "", 8.5)
		for _, line := range wrapText(p, table.Note, contentW) {
			p.CellFormat(contentW, 5, line, "", 1, "L", false, 0, "")
		}
	}
}

var issueLedgerWidths = []float64{9, 17, 41, 35, 41, 20, 17}

func drawIssueLedgerHeader(p *gofpdf.Fpdf) {
	headerRow(p, issueLedgerWidths, []string{"序号", "日期", "故障/问题", "照片", "处理情况", "照片", "检查人"}, 11)
}

func drawIssueLedgerPhoto(p *gofpdf.Fpdf, d MonthlyReportData, key, uniq string, x, y, w, h float64) {
	if name, iw, ih, ok := registerImage(p, d.ImageLoader, key, uniq, w-3, h-3); ok {
		p.ImageOptions(name, x+(w-iw)/2, y+(h-ih)/2, iw, ih, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
	}
}

// renderIssueLedger 问题清单及整改台账（一条异常打卡记录一行；处理情况=复核结论）。
func renderIssueLedger(p *gofpdf.Fpdf, d MonthlyReportData) {
	pageCount := (len(d.Ledger) + 5) / 6
	if pageCount == 0 {
		pageCount = 1
	}
	for page := 0; page < pageCount; page++ {
		p.AddPage()
		p.SetY(19)
		title := "5.问题清单及整改台账"
		if page > 0 {
			title += "（续）"
		}
		p.SetFont("noto", "B", 15)
		p.CellFormat(contentW, 8, title, "", 1, "L", false, 0, "")
		p.Ln(2)
		drawIssueLedgerHeader(p)
		start, end := page*6, min((page+1)*6, len(d.Ledger))
		for idx := start; idx < end; idx++ {
			row, y0, h := d.Ledger[idx], p.GetY(), 25.0
			x := margin
			for _, w := range issueLedgerWidths {
				p.Rect(x, y0, w, h, "D")
				x += w
			}
			center := []struct {
				index int
				text  string
			}{{0, itoa(idx + 1)}, {1, row.Date}, {6, row.Inspector}}
			for _, cell := range center {
				x := margin
				for before := 0; before < cell.index; before++ {
					x += issueLedgerWidths[before]
				}
				p.SetXY(x, y0+(h-5)/2)
				p.SetFont("noto", "", 8)
				p.CellFormat(issueLedgerWidths[cell.index], 5, trunc(p, cell.text, issueLedgerWidths[cell.index]-1), "", 0, "C", false, 0, "")
			}
			drawText := func(index int, text string) {
				x := margin
				for before := 0; before < index; before++ {
					x += issueLedgerWidths[before]
				}
				p.SetFont("noto", "", 8)
				lines := wrapText(p, text, issueLedgerWidths[index]-3)
				if len(lines) > 4 {
					lines = lines[:4]
				}
				for lineIndex, line := range lines {
					p.SetXY(x+1.5, y0+1.5+float64(lineIndex)*4.5)
					p.CellFormat(issueLedgerWidths[index]-3, 4.5, line, "", 0, "L", false, 0, "")
				}
			}
			drawText(2, row.Problem)
			drawText(4, row.FixText)
			xProblemPhoto := margin + issueLedgerWidths[0] + issueLedgerWidths[1] + issueLedgerWidths[2]
			xFixPhoto := xProblemPhoto + issueLedgerWidths[3] + issueLedgerWidths[4]
			if len(row.ProblemPhotos) > 0 {
				drawIssueLedgerPhoto(p, d, row.ProblemPhotos[0], fmt.Sprintf("issue-ledger-p-%d", idx), xProblemPhoto, y0, issueLedgerWidths[3], h)
			}
			if len(row.FixPhotos) > 0 {
				drawIssueLedgerPhoto(p, d, row.FixPhotos[0], fmt.Sprintf("issue-ledger-f-%d", idx), xFixPhoto, y0, issueLedgerWidths[5], h)
			}
			p.SetY(y0 + h)
		}
		for empty := end - start; empty < 6; empty++ {
			y0, h, x := p.GetY(), 25.0, margin
			for _, w := range issueLedgerWidths {
				p.Rect(x, y0, w, h, "D")
				x += w
			}
			p.SetY(y0 + h)
		}
		p.Ln(3)
		company, date := d.CompanyName, dateCN(d.ApproveDate)
		if company == "" {
			company = "物业服务中心"
		}
		if date == "" {
			date = "        年    月    日"
		}
		p.SetFont("noto", "", 10)
		p.CellFormat(contentW, 6, company, "", 1, "R", false, 0, "")
		p.CellFormat(contentW, 6, date, "", 1, "R", false, 0, "")
	}
}
