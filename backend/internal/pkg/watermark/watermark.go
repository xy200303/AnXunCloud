// Package watermark local 模式本地图片水印（imaging + x/image/font，云存储模式走对象存储图片处理）。
package watermark

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os"
	"sync"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"anxuncloud/internal/pkg/logger"
	"go.uber.org/zap"
)

var (
	fontOnce   sync.Once
	parsedFont *opentype.Font
	fontErr    error
	cfgPath    string

	logoOnce    sync.Once
	logoImg     image.Image
	cfgLogoPath string
)

// Init 设置水印字体与二维码标牌 LOGO 路径（启动时调用一次）。
func Init(fontPath, logoPath string) { cfgPath = fontPath; cfgLogoPath = logoPath }

// loadLogo 加载标牌 LOGO（只加载一次；文件缺失或解码失败返回 nil，调用方跳过 LOGO）。
func loadLogo() image.Image {
	logoOnce.Do(func() {
		if cfgLogoPath == "" {
			return
		}
		f, err := os.Open(cfgLogoPath)
		if err != nil {
			logger.L.Warn("标牌 LOGO 不可用，跳过", zap.String("path", cfgLogoPath), zap.Error(err))
			return
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			logger.L.Warn("标牌 LOGO 解码失败，跳过", zap.String("path", cfgLogoPath), zap.Error(err))
			return
		}
		logoImg = img
	})
	return logoImg
}

// DrawLogoCentered 在指定顶部 y 处水平居中绘制 LOGO（限制最大宽高，保持比例），返回占用高度；无 LOGO 返回 0。
func DrawLogoCentered(dst *image.RGBA, topY, maxW, maxH int) int {
	logo := loadLogo()
	if logo == nil {
		return 0
	}
	scaled := imaging.Fit(logo, maxW, maxH, imaging.Lanczos)
	x := (dst.Bounds().Dx() - scaled.Bounds().Dx()) / 2
	draw.Draw(dst, image.Rect(x, topY, x+scaled.Bounds().Dx(), topY+scaled.Bounds().Dy()),
		scaled, image.Point{}, draw.Over)
	return scaled.Bounds().Dy()
}

// loadParsed 加载并解析中文字体文件（只解析一次）。
func loadParsed(fontPath string) (*opentype.Font, error) {
	if fontPath == "" {
		fontPath = cfgPath
	}
	fontOnce.Do(func() {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			fontErr = err
			return
		}
		parsedFont, fontErr = opentype.Parse(data)
	})
	return parsedFont, fontErr
}

// faceFor 按字号生成 Face（全量 hinting，标牌打印更清晰）。
func faceFor(size float64) (font.Face, error) {
	f, err := loadParsed(cfgPath)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// loadFont 默认字号（22）Face，照片水印用。
func loadFont(fontPath string) (font.Face, error) {
	if _, err := loadParsed(fontPath); err != nil {
		return nil, err
	}
	return faceFor(22)
}

// DrawToFile 在图片左下角叠加多行水印文字（半透明黑底白字），输出为新文件。
// 字体不可用或图片解码失败时返回 error，调用方降级为不加水印。
func DrawToFile(srcPath, dstPath, fontPath string, lines []string) error {
	src, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	dst, err := drawLines(src, fontPath, lines)
	if err != nil {
		return err
	}
	if err := imaging.Save(dst, dstPath, imaging.JPEGQuality(88)); err != nil {
		return err
	}
	logger.L.Debug("水印生成", zap.String("src", srcPath), zap.String("dst", dstPath))
	return nil
}

// DrawBytes 同 DrawToFile，但输入/输出均为字节（云存储场景：ReadFile 读入 → 烧录 → Put 回写）。
func DrawBytes(src []byte, fontPath string, lines []string) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(src), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	dst, err := drawLines(img, fontPath, lines)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, dst, imaging.JPEG, imaging.JPEGQuality(88)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawLines 水印绘制核心：左下角半透明黑底白字多行文字；过大的图先缩到长边 2000 控制内存。
func drawLines(src image.Image, fontPath string, lines []string) (image.Image, error) {
	face, err := loadFont(fontPath)
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	if bounds.Dx() > 2000 || bounds.Dy() > 2000 {
		src = imaging.Fit(src, 2000, 2000, imaging.Lanczos)
		bounds = src.Bounds()
	}
	dst := imaging.New(bounds.Dx(), bounds.Dy(), color.NRGBA{0, 0, 0, 0})
	dst = imaging.Paste(dst, src, image.Pt(0, 0))

	lineH := 30
	pad := 12
	boxH := lineH*len(lines) + pad*2
	boxY := bounds.Dy() - boxH
	// 底部半透明条
	bar := imaging.New(bounds.Dx(), boxH, color.NRGBA{0, 0, 0, 110})
	dst = imaging.Paste(dst, bar, image.Pt(0, boxY))

	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.NRGBA{255, 255, 255, 255}),
		Face: face,
	}
	for i, line := range lines {
		d.Dot = fixed.P(pad+4, boxY+pad+lineH*i+20)
		d.DrawString(line)
	}
	return dst, nil
}

// TextRGBA 在图像指定位置绘制一行深色文字（供二维码标题等场景使用）。
func TextRGBA(dst *image.RGBA, x, y int, text string) {
	face, err := loadFont(cfgPath)
	if err != nil {
		return
	}
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.NRGBA{30, 30, 30, 255}),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// TextCenterRGBA 在图像水平居中绘制一行深色文字（二维码标题用），size 为字号。
func TextCenterRGBA(dst *image.RGBA, y int, text string, size float64) {
	face, err := faceFor(size)
	if err != nil {
		return
	}
	w := font.MeasureString(face, text).Ceil()
	x := (dst.Bounds().Dx() - w) / 2
	if x < 4 {
		x = 4
	}
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.NRGBA{30, 30, 30, 255}),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}
