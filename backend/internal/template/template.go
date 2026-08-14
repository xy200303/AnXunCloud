// Package template 集中管理随二进制分发的页面模板与静态资源（go:embed）：
// 品牌官网（website/）、pdf.js 内嵌查看器（pdfjs/）、点位短链接 H5 页（point_page.html）。
// 路由层只做数据组装与渲染调用，不直接持有资源文件。
package template

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"html/template"
)

// websiteFS 品牌官网模板与静态资源。
//
//go:embed website
var websiteFS embed.FS

// PdfjsFS pdf.js 精简查看器静态资源（App 端 web-view 内渲染报告 PDF）。
//
//go:embed pdfjs
var PdfjsFS embed.FS

// PointPageHTML NFC/二维码短链接 H5 点位信息页（免登录，单文件内联资源）。
//
//go:embed point_page.html
var PointPageHTML string

// Site 官网页面模板（首页/下载页），启动时解析一次。
var Site = template.Must(template.New("site").ParseFS(websiteFS, "website/index.html", "website/download.html"))

// WebsiteFile 读取官网目录内文件（静态资源分发用）。
func WebsiteFile(path string) ([]byte, error) {
	return websiteFS.ReadFile("website/" + path)
}

// SiteAssetsVer 静态资源版本号：取 site.css 内容 MD5 前 8 位，CSS 变更后链接自动变化，避免浏览器旧缓存。
var SiteAssetsVer = func() string {
	data, err := websiteFS.ReadFile("website/assets/site.css")
	if err != nil {
		return "1"
	}
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])[:8]
}()
