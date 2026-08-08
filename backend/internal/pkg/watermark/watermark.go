// Package watermark dev 模式本地图片水印（imaging + x/image/font，生产走 OSS 图片处理）。
package watermark

import (
	"image"
	"image/color"
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
	fontOnce sync.Once
	fontFace font.Face
	fontErr  error
	cfgPath  string
)

// Init 设置水印字体路径（启动时调用一次）。
func Init(fontPath string) { cfgPath = fontPath }

// loadFont 加载中文字体（只加载一次）。
func loadFont(fontPath string) (font.Face, error) {
	if fontPath == "" {
		fontPath = cfgPath
	}
	fontOnce.Do(func() {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			fontErr = err
			return
		}
		f, err := opentype.Parse(data)
		if err != nil {
			fontErr = err
			return
		}
		fontFace, fontErr = opentype.NewFace(f, &opentype.FaceOptions{Size: 22, DPI: 72})
	})
	return fontFace, fontErr
}

// DrawToFile 在图片左下角叠加多行水印文字（半透明黑底白字），输出为新文件。
// 字体不可用或图片解码失败时返回 error，调用方降级为不加水印。
func DrawToFile(srcPath, dstPath, fontPath string, lines []string) error {
	face, err := loadFont(fontPath)
	if err != nil {
		return err
	}
	src, err := imaging.Open(srcPath)
	if err != nil {
		return err
	}
	// 过大的图先缩到长边 2000，控制内存
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
	if err := imaging.Save(dst, dstPath, imaging.JPEGQuality(88)); err != nil {
		return err
	}
	logger.L.Debug("水印生成", zap.String("src", srcPath), zap.String("dst", dstPath))
	return nil
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
