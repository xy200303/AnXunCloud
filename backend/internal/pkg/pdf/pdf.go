// Package pdf 月度报告 PDF 生成（gofpdf + 内嵌 Noto Sans SC 中文字体）。
package pdf

import (
	"embed"
)

// Fonts 内嵌中文字体（Noto Sans SC，OFL 开源许可；构建进二进制，无需运行时字体文件）。
//
//go:embed fonts/NotoSansSC-Regular.ttf fonts/NotoSansSC-Bold.ttf
var Fonts embed.FS
