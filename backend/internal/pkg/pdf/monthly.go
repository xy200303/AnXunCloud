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
	// SignatureFileID 签字时的手写签名图 file_id 快照（空回退打印姓名）
	SignatureFileID string
}

type ReviewSignGroup struct {
	Name  string
	Signs []SignInfo
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

// LedgerRow 问题清单及整改台账行（新版模板：一条异常打卡记录一行）。
type LedgerRow struct {
	Date            string   // 日期（异常打卡日）
	Category        string   // 类别（点位类型中文名）
	Location        string   // 区域位置（点位名/楼栋位置）
	Problem         string   // 问题说明（异常备注）
	ProblemPhotoIDs []string // 故障问题照片 file_id（渲染取首张）
	FixText         string   // 整改情况（处置方式/复核结论）
	FixPhotoIDs     []string // 处理完结照片 file_id（渲染取首张）
	Inspector       string   // 检查人（打卡巡检员）
}

// PhotoCell 现场照片单元（标注 + 图片 file_id）。
type PhotoCell struct {
	Label  string // 小标注：检查项名（逐项照片）或"全景"（记录级照片）
	FileID string // 图片 file_id（经 ImageLoader 加载）
}

// PhotoGroup 分项检查照片分组（v2：一个设施类别一组）。
type PhotoGroup struct {
	Title string // 设施类别中文名（渲染为「N.{类别}巡检照片」）
	Cells []PhotoCell
}

// ContactInfo 封面页脚联系方式（全部为空则不画页脚）。
type ContactInfo struct {
	Tel     string // 电话
	Email   string // 邮箱
	Website string // 网址
	Address string // 地址
}

// MonthlyReportData 月度巡检报告 PDF 数据。
type MonthlyReportData struct {
	ReportNo         string   // 封面报告编号（空则留白线）
	CommunityName    string   // 项目名称（小区名）
	CommunityAddress string   // 项目地址（小区地址，空则留白线）
	Period           string   // YYYY-MM
	TitleLine        string   // 封面大标题首行（空回落「物业设施月度」；由巡查类型推导）
	CompanyName      string   // 落款单位（空则封面留白 / 台账页尾「物业服务中心」）
	CompanyNameEn    string   // 落款单位英文名（空则封面不显示该行）
	Contact          ContactInfo // 封面页脚联系方式
	Approved         bool     // 已终审（公章仅终审后加盖）
	ApproveDate      string   // 终审日期 YYYY-MM-DD
	SealFileID       string   // 公章图 file_id（仅 Approved 时嵌入）
	TypeNames        []string // 设施类别（该小区有点位的类型中文名）
	Summary          []SummaryRow
	Details          []DetailTable
	PhotoGroups      []PhotoGroup // 附件：分项检查照片（按设施类别分组）
	Ledger           []LedgerRow
	ReviewSigns      []ReviewSignGroup
	// ImageLoader 按 file_id 加载图片字节与类型（JPG/PNG）；nil 则跳过签名图/公章。
	// 单张加载失败返回 error，PDF 侧跳过该张，不影响整体生成。
	ImageLoader func(fileID string) (data []byte, imgType string, err error)
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

	// 居中大标题（首行可配，次行固定）
	titleLine := d.TitleLine
	if titleLine == "" {
		titleLine = "物业设施月度"
	}
	p.SetY(66)
	p.SetFont("noto", "B", 25)
	p.CellFormat(contentW, 15, titleLine, "", 1, "C", false, 0, "")
	p.CellFormat(contentW, 15, "巡 检 报 告", "", 1, "C", false, 0, "")

	// 报告编号 / 项目名称 / 项目地址 / 器材类别 / 报告日期（下划线填空式）
	infoY := 118.0
	lineH := 13.0
	labelW, valueW := 30.0, 96.0
	blockX := (pageW - labelW - valueW) / 2
	typeLine := strings.Join(d.TypeNames, "/")
	// 报告日期按报告月份展示（YYYY-MM → 2026 年 8 月）
	dateLine := periodCN(d.Period)
	labels := []string{"报告编号：", "项目名称：", "项目地址：", "器材类别：", "报告日期："}
	infos := []string{d.ReportNo, d.CommunityName, d.CommunityAddress, typeLine, dateLine}
	for i := range labels {
		y := infoY + float64(i)*lineH
		p.SetXY(blockX, y)
		p.SetFont("noto", "", 13)
		p.CellFormat(labelW, lineH-4, labels[i], "", 0, "L", false, 0, "")
		p.CellFormat(valueW, lineH-4, infos[i], "B", 1, "C", false, 0, "")
	}

	// 落款公司块：公司名 + 英文名（可配）+（加盖公章）；仅终审通过后加盖公章
	companyY := infoY + float64(len(labels))*lineH + 10
	// 公章先绘于文字下层：PNG 压平白底后不透明，后置会遮挡落款文字
	if d.Approved && d.SealFileID != "" {
		if name, w, h, ok := registerImage(p, d.ImageLoader, d.SealFileID, "seal", 35, 35); ok {
			p.ImageOptions(name, blockX+(labelW+valueW)/2+8, companyY-h/2, w, h, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
		}
	}
	p.SetFont("noto", "B", 14)
	p.SetXY(blockX, companyY)
	p.CellFormat(labelW+valueW, 9, d.CompanyName, "", 1, "C", false, 0, "")
	if d.CompanyNameEn != "" {
		p.SetFont("noto", "", 9)
		p.CellFormat(labelW+valueW, 5.5, d.CompanyNameEn, "", 1, "C", false, 0, "")
	}
	p.SetFont("noto", "", 10)
	p.CellFormat(labelW+valueW, 6, "（加盖公章）", "", 1, "C", false, 0, "")

	// 页脚：联系方式（任一配置非空才画；逐项拼接，缺项不显示）
	var contactParts []string
	if d.Contact.Tel != "" {
		contactParts = append(contactParts, "电话："+d.Contact.Tel)
	}
	if d.Contact.Email != "" {
		contactParts = append(contactParts, "邮箱："+d.Contact.Email)
	}
	if d.Contact.Website != "" {
		contactParts = append(contactParts, "网址："+d.Contact.Website)
	}
	if len(contactParts) > 0 || d.Contact.Address != "" {
		p.SetFont("noto", "", 9)
		y := 273.0
		if d.Contact.Address != "" {
			p.SetXY(margin, y)
			p.CellFormat(contentW, 5, "地址："+d.Contact.Address, "", 0, "C", false, 0, "")
			y -= 6
		}
		if len(contactParts) > 0 {
			p.SetXY(margin, y)
			p.CellFormat(contentW, 5, strings.Join(contactParts, "     "), "", 0, "C", false, 0, "")
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
				n, w, h, ok := registerImage(p, d.ImageLoader, cell.FileID, fmt.Sprintf("site-%d-%d-%d", gi, row, i), photoImgMaxW, photoImgMaxH)
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
		if name, iw, ih, ok := registerImage(p, d.ImageLoader, s.SignatureFileID, uniq, 34, 13); ok {
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

// renderLedgerIntroAndSign 报告编制说明（新版模板 5 节原文，项目名/覆盖范围动态代入）+ 签字审批栏。
func renderLedgerIntroAndSign(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	p.SetY(18)
	p.SetFont("noto", "B", 15)
	p.CellFormat(contentW, 8, "1.报告编制说明", "", 1, "L", false, 0, "")
	p.Ln(2)
	project := d.CommunityName
	if project == "" {
		project = "园区"
	}
	coverage := strings.Join(d.TypeNames, "、")
	coverageText := "园区室内外消火栓、公共区域灭火器、楼道应急照明灯、疏散指示标志、防火门、防火卷帘、消防疏散通道，实现园区全域、无死角覆盖。"
	if coverage != "" {
		coverageText = "园区" + coverage + "、消防疏散通道等，实现园区全域、无死角覆盖。"
	}
	sections := []struct {
		title string
		body  string
	}{
		{"1.编制依据", "本报告依据《消防安全管理规定》《物业服务合同》编制，是物业服务人员每月对" + project + "消防器材巡检（维护）形成的正式归档资料，完整记录器材状况、安全隐患及整改闭环全过程，可用于消防、住建、街道社区各级主管部门核查。"},
		{"2.执行主体与职责", "（1）巡检执行人：现场实地逐项检查，使用NFC标签点位打卡，现场拍摄设备照片，同步上传系统，如实记录设备状态、现场隐患；对轻微隐患当场处置，无法现场整改的拍照登记上报。\n（2）安全负责人：统筹月度巡检计划，核对全部巡检数据、影像资料，跟踪隐患整改进度，复查整改完成点位，审核巡检台账真实性。\n（3）项目负责人：最终审批月度巡检报告，统筹整改资源，对园区消防安全管理负总责，定期抽查巡检完成质量。"},
		{"3.巡检方式与核验标准", "采用NFC点位打卡+人工现场核查+AI图像识别三重核验；巡检人员现场实拍设备，系统自动留存影像、设备编号、压力、有效期等数据，资料加密留痕、不可篡改，全程可追溯。"},
		{"4.巡检覆盖范围", coverageText},
		{"5.归档与附件管理", "所有巡检实拍图、AI核验截图、隐患整改前后对比照片，按器材唯一编号分类存入电子档案；纸质报告签字盖章后与电子资料同步留存，长期保管备查。"},
	}
	for _, sec := range sections {
		p.SetFont("noto", "B", 11)
		p.CellFormat(contentW, 6.5, sec.title, "", 1, "L", false, 0, "")
		p.SetFont("noto", "", 9.5)
		for _, line := range wrapText(p, sec.body, contentW-6) {
			p.SetX(margin + 3)
			p.CellFormat(contentW-6, 5.4, line, "", 1, "L", false, 0, "")
		}
		p.Ln(1)
	}
	// 签字审批栏整体约 92mm，放不下则整栏移到下页
	ensureSpace(p, 92)
	renderLedgerSignTable(p, d)
}

func renderLedgerSignTable(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.SetFont("noto", "B", 14)
	p.CellFormat(contentW, 8, "2.签字审批栏", "", 1, "L", false, 0, "")
	p.Ln(2)
	count := len(d.ReviewSigns)
	if count == 0 { p.SetY(p.GetY()+8); return }
	headerH, bodyH := 12.0, 62.0
	cellW := contentW / float64(count)
	x0, y0 := margin, p.GetY()
	p.SetFillColor(oliveHeader[0], oliveHeader[1], oliveHeader[2])
	for idx, group := range d.ReviewSigns {
		role, stage := signRole(idx, group.Name)
		x := x0 + float64(idx)*cellW
		p.Rect(x, y0, cellW, headerH, "DF")
		p.SetFont("noto", "B", 10)
		p.SetXY(x, y0+1.5)
		p.CellFormat(cellW, 4.5, role+"签字", "", 0, "C", false, 0, "")
		p.SetFont("noto", "", 8)
		p.SetXY(x, y0+6.5)
		p.CellFormat(cellW, 4, stage, "", 0, "C", false, 0, "")
		p.Rect(x, y0+headerH, cellW, bodyH, "D")
		renderSignCell(p, d, idx, x, y0+headerH, cellW, bodyH-11, group.Signs)
		date := signBarDate(group.Signs)
		if date == "" { date = "      年    月    日" }
		p.SetXY(x, y0+headerH+bodyH-8)
		p.SetFont("noto", "", 9)
		p.CellFormat(cellW, 5, date, "", 0, "C", false, 0, "")
	}
	p.SetY(y0 + headerH + bodyH)
}

// signRole 签字审批栏角色文案：第 1 环=巡检执行人（检查）、第 2 环=安全负责人（审核）、
// 第 3 环=项目负责人（签批）；多于 3 环顺延按审核环节处理（角色名取环节名）。
func signRole(idx int, stepName string) (role, stage string) {
	switch idx {
	case 0:
		return "巡检执行人", "（检查）"
	case 1:
		return "安全负责人", "（审核）"
	case 2:
		return "项目负责人", "（签批）"
	}
	if stepName == "" {
		stepName = "审核人"
	}
	return stepName, "（审核）"
}

// renderLedgerSummary 本月检查汇总表（行=点位类型分项，按模板顺序排序；表下附模板备注两条）。
func renderLedgerSummary(p *gofpdf.Fpdf, d MonthlyReportData) {
	p.AddPage()
	p.SetY(22)
	p.SetFont("noto", "B", 15)
	p.CellFormat(contentW, 8, "3.本月检查汇总表", "", 1, "L", false, 0, "")
	p.Ln(2)
	widths := []float64{29, 14, 18, 20, 17, 17, 20, 45}
	headerRow(p, widths, []string{"设施类别", "总数", "正常完好", "巡检完成率", "存在问题", "整改完毕", "整改完成率", "备注"}, 12)
	for _, row := range d.Summary {
		rectifyRate := pctText(row.RectifyRate)
		if row.Problems == 0 {
			rectifyRate = "—" // 无问题点位时整改完成率无意义
		}
		dataRow(p, widths, []string{row.TypeName, itoa(row.Total), itoa(row.Normal), pctText(row.InspectRate), itoa(row.Problems), itoa(row.Rectified), rectifyRate, row.Remark}, 12, 9)
	}
	p.Ln(2)
	p.SetFont("noto", "", 8.5)
	for _, line := range []string{
		"备注：1.公区硬件点位，仅查实物完好、在位、无遮挡等，不测试系统联动。",
		"2.消火栓、灭火器每月实行集中全履盖检查1次，应急灯、指示牌等按要求抽查。",
	} {
		p.CellFormat(contentW, 5, line, "", 1, "L", false, 0, "")
	}
}

// ledgerDetailTitle 分项明细表标题（4.N {点位类型}巡检明细表）。
func ledgerDetailTitle(idx int, typeName string) string {
	return fmt.Sprintf("4.%d %s巡检明细表", idx+1, typeName)
}

// drawLedgerDetailHeader 明细表头：序号/位置编号 + 检查项竖排窄列 + 巡检人/巡检时间。
// 新版模板明细表无「问题说明」列（问题统一进第 5 节台账），让渡宽度给检查项列与巡检时间。
// 检查项列宽随项数均分，表头文字逐字竖排（对齐甲方样稿）。
// 行高按最长检查项动态撑开（竖排总高+留白）；超长兜底压缩字高，封顶 42mm 防溢出。
func drawLedgerDetailHeader(p *gofpdf.Fpdf, items []string) []float64 {
	itemW := 0.0
	if len(items) > 0 {
		itemW = (contentW - 9 - 32 - 20 - 24) / float64(len(items))
	}
	widths := []float64{9, 32}
	for range items {
		widths = append(widths, itemW)
	}
	widths = append(widths, 20, 24)
	labels := []string{"序号", "位置/编号"}
	labels = append(labels, items...)
	labels = append(labels, "巡检人", "巡检时间")
	maxChars := 0
	for _, it := range items {
		if n := len([]rune(it)); n > maxChars {
			maxChars = n
		}
	}
	h, charH := 17.0, 3.9
	if maxChars > 0 {
		if need := float64(maxChars)*charH + 6; need > h {
			h = need
		}
		if h > 42 {
			charH = (42 - 6) / float64(maxChars)
			h = 42
		}
	}
	x0, y0 := margin, p.GetY()
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
			cy := y0 + (h-float64(len(chars))*charH)/2
			for _, char := range chars {
				p.SetXY(x, cy)
				p.CellFormat(widths[idx], charH, string(char), "", 0, "C", false, 0, "")
				cy += charH
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
			cells = append(cells, row.Inspector, row.Time)
			dataRow(p, widths, cells, 14.0, 8.5)
		}
		for empty := end - start; empty < 9; empty++ {
			dataRow(p, widths, make([]string, len(widths)), 14.0, 8.5)
		}
		p.Ln(2)
		p.SetFont("noto", "", 8.5)
		for _, line := range wrapText(p, detailNote(table), contentW) {
			p.CellFormat(contentW, 5, line, "", 1, "L", false, 0, "")
		}
	}
}

// detailNote 明细表下「注：检查标准……」：灭火器/消火栓照新版模板原文，
// 其余点位类型用数据组装的标准注，兜底通用句。
func detailNote(table DetailTable) string {
	name := table.TypeName
	if strings.Contains(name, "灭火器") {
		return "注：检查标准，压力指针在绿色区域为正常；瓶体无破损、喷管完好、摆放便于取用、无遮挡。"
	}
	if strings.Contains(name, "消火栓") || strings.Contains(name, "消防栓") {
		return "注：检查标准：箱门完好、水带水枪齐全无破损、接口完好、水压正常、周围无遮挡。"
	}
	if table.Note != "" {
		return table.Note
	}
	return "注：检查标准：设施完好、在位、无遮挡。"
}

// ========== 5.问题清单及整改台账 ==========

// 新版模板列：序号/类别/区域位置/问题说明（附故障问题照片）/整改情况（附处理完结照片）。
var issueLedgerWidths = []float64{9, 22, 32, 56, 61}

// issueLedgerRowH 台账行高：文字至多两行 + 照片 + 照片标注。
const issueLedgerRowH = 30.0

func drawIssueLedgerHeader(p *gofpdf.Fpdf) {
	headerRow(p, issueLedgerWidths, []string{"序号", "类别", "区域位置", "问题说明", "整改情况"}, 11)
}

// drawIssueLedgerCell 问题/整改单元格：上文字（至多两行），下照片（取首张，带小标注）。
func drawIssueLedgerCell(p *gofpdf.Fpdf, d MonthlyReportData, text string, photos []string, uniq, caption string, x, y, w float64) {
	p.SetFont("noto", "", 8)
	lines := wrapText(p, text, w-3)
	if len(lines) > 2 {
		lines = lines[:2]
	}
	for i, line := range lines {
		p.SetXY(x+1.5, y+1.5+float64(i)*4.5)
		p.CellFormat(w-3, 4.5, line, "", 0, "L", false, 0, "")
	}
	if len(photos) == 0 {
		return
	}
	if name, iw, ih, ok := registerImage(p, d.ImageLoader, photos[0], uniq, w-8, 13); ok {
		p.ImageOptions(name, x+(w-iw)/2, y+11.5+(13-ih)/2, iw, ih, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
		p.SetXY(x, y+issueLedgerRowH-5)
		p.SetFont("noto", "", 6.5)
		p.CellFormat(w, 4, caption, "", 0, "C", false, 0, "")
	}
}

// renderIssueLedger 问题清单及整改台账（一条异常打卡记录一行；末尾落款=落款单位+报告期次年月，日留白手填）。
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
			row, y0 := d.Ledger[idx], p.GetY()
			x := margin
			for _, w := range issueLedgerWidths {
				p.Rect(x, y0, w, issueLedgerRowH, "D")
				x += w
			}
			center := []struct {
				index int
				text  string
			}{{0, itoa(idx + 1)}, {1, row.Category}, {2, row.Location}}
			for _, cell := range center {
				x := margin
				for before := 0; before < cell.index; before++ {
					x += issueLedgerWidths[before]
				}
				p.SetXY(x, y0+(issueLedgerRowH-5)/2)
				p.SetFont("noto", "", 8)
				p.CellFormat(issueLedgerWidths[cell.index], 5, trunc(p, cell.text, issueLedgerWidths[cell.index]-1), "", 0, "C", false, 0, "")
			}
			xProb := margin + issueLedgerWidths[0] + issueLedgerWidths[1] + issueLedgerWidths[2]
			drawIssueLedgerCell(p, d, row.Problem, row.ProblemPhotoIDs, fmt.Sprintf("issue-ledger-p-%d", idx), "故障问题照片", xProb, y0, issueLedgerWidths[3])
			drawIssueLedgerCell(p, d, row.FixText, row.FixPhotoIDs, fmt.Sprintf("issue-ledger-f-%d", idx), "处理完结照片", xProb+issueLedgerWidths[3], y0, issueLedgerWidths[4])
			p.SetY(y0 + issueLedgerRowH)
		}
		for empty := end - start; empty < 6; empty++ {
			y0, x := p.GetY(), margin
			for _, w := range issueLedgerWidths {
				p.Rect(x, y0, w, issueLedgerRowH, "D")
				x += w
			}
			p.SetY(y0 + issueLedgerRowH)
		}
	}
	// 末尾落款（仅最后一页）：落款单位 + 报告期次年月（日留白手填，对齐模板「2026年 月 日」）
	ensureSpace(p, 18)
	company, date := d.CompanyName, periodCN(d.Period)
	if company == "" {
		company = "物业服务中心"
	}
	if date == "" {
		date = "        年    月"
	}
	p.Ln(3)
	p.SetFont("noto", "", 10)
	p.CellFormat(contentW, 6, company, "", 1, "R", false, 0, "")
	p.CellFormat(contentW, 6, date+"    日", "", 1, "R", false, 0, "")
}
