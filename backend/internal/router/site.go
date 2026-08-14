// 品牌官网与 App 下载：内嵌静态站点（随二进制分发）+ 下载渠道/页面配置公开接口。
package router

import (
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"

	"anxuncloud/internal/config"
	systemctl "anxuncloud/internal/module/system/controller"
)

// websiteAssets 内嵌官网静态资源（index.html / download.html / assets/*），随二进制分发，无需额外挂载。
//
//go:embed website
var websiteAssets embed.FS

// registerSiteRoutes 注册品牌官网（/、/download、/site/*）与 App 下载公开接口。
func registerSiteRoutes(r *gin.Engine, cfg *config.Config, siteCtl *systemctl.SiteController) {
	servePage := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			data, err := websiteAssets.ReadFile("website/" + name)
			if err != nil {
				c.JSON(404, gin.H{"code": 40400, "message": "页面不存在", "data": nil})
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Data(200, "text/html; charset=utf-8", data)
		}
	}
	r.GET("/", servePage("index.html"))
	r.GET("/download", servePage("download.html"))
	// 官网静态资源（css/svg）。文件不带内容哈希，给短缓存兼顾更新及时性
	r.GET("/site/*filepath", func(c *gin.Context) {
		p := strings.TrimPrefix(c.Param("filepath"), "/")
		if p == "" || strings.Contains(p, "..") || strings.HasSuffix(p, ".html") {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在", "data": nil})
			return
		}
		data, err := websiteAssets.ReadFile("website/" + p)
		if err != nil {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在", "data": nil})
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(200, siteContentType(p), data)
	})

	// App 下载公开数据：页面配置、各平台最新发布物、发布物文件下载
	r.GET("/api/public/site-config", siteCtl.PublicConfig)
	r.GET("/api/public/download/releases", siteCtl.PublicReleases)
	r.GET("/api/public/download/app/:id", siteCtl.Download)

	// 下载页二维码（内容为官网下载页地址，手机扫码打开 /download 再点下载，兼容性最好）
	r.GET("/api/public/download/qr", func(c *gin.Context) {
		png, err := qrcode.Encode(strings.TrimRight(cfg.App.BaseURL, "/")+"/download", qrcode.Medium, 256)
		if err != nil {
			c.JSON(500, gin.H{"code": 50000, "message": "二维码生成失败", "data": nil})
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(200, "image/png", png)
	})
}

// siteContentType 官网静态资源的常见 MIME 类型。
func siteContentType(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".woff2":
		return "font/woff2"
	default:
		return http.DetectContentType([]byte(p))
	}
}
