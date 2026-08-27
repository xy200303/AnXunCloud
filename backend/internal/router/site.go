// 品牌官网：服务端渲染（模板/静态资源集中在 internal/template，配置直接注入 HTML，利于 SEO/GEO 收录）+ App 下载公开接口。
package router

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"

	"anxuncloud/internal/config"
	"anxuncloud/internal/module/system/model"
	systemctl "anxuncloud/internal/module/system/controller"
	systemsvc "anxuncloud/internal/module/system/service"
	sitetpl "anxuncloud/internal/template"
)

// themeColorReSite 主题色二次校验（保存时已校验，这里防库内手改脏数据注入样式）
var themeColorReSite = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// sitePageData 官网页面渲染数据。
type sitePageData struct {
	BaseURL    string
	Year       int
	AssetsVer  string            // 静态资源版本号（内容哈希，防旧缓存）
	Cfg        map[string]string // 去掉 site. 前缀的白名单配置（空值缺省）
	ThemeStyle template.CSS      // 主题色覆盖样式（Go 侧校验为 #RRGGBB 后拼接，安全）
	JSONLD     template.JS       // 结构化数据（Go 侧 json.Marshal，安全）
	Channels   []siteChannel     // 下载页渠道卡片
}

// siteChannel 下载页单个渠道的渲染视图。
type siteChannel struct {
	Key       string // android / harmony / ios / wechat_mp
	Badge     string
	Name      string
	OS        string
	Available bool
	PageQR    bool // Android：展示本页二维码（手机扫码后到本页点下载，微信等环境兼容最好）
	MpQR      bool // 小程序：展示后台上传的小程序码
	Meta      string
	URL       string
}

// registerSiteRoutes 注册品牌官网（SSR 页面 + 静态资源 + SEO 端点 + App 下载公开接口）。
func registerSiteRoutes(r *gin.Engine, cfg *config.Config, siteSvc *systemsvc.SiteService, siteCtl *systemctl.SiteController) {
	baseURL := strings.TrimRight(cfg.App.BaseURL, "/")

	pageData := func() sitePageData {
		c := siteSvc.BrandConfigMap()
		var style template.CSS
		if themeColorReSite.MatchString(c["theme_color"]) {
			style = template.CSS(":root{--blue:" + c["theme_color"] + ";--blue-dark:" + darkenHex(c["theme_color"], 0.8) + ";}")
		}
		return sitePageData{
			BaseURL:    baseURL,
			Year:       time.Now().Year(),
			AssetsVer:  sitetpl.SiteAssetsVer,
			Cfg:        c,
			ThemeStyle: style,
			JSONLD:     siteJSONLD(baseURL, c),
		}
	}
	render := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Header("Cache-Control", "no-cache")
			c.Header("Content-Type", "text/html; charset=utf-8")
			if err := sitetpl.Site.ExecuteTemplate(c.Writer, name, pageData()); err != nil {
				c.JSON(500, gin.H{"code": 50000, "message": "页面渲染失败", "data": nil})
			}
		}
	}

	r.GET("/", render("index.html"))
	r.GET("/download", func(c *gin.Context) {
		d := pageData()
		d.Channels = buildChannels(siteSvc.LatestReleases())
		c.Header("Cache-Control", "no-cache")
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := sitetpl.Site.ExecuteTemplate(c.Writer, "download.html", d); err != nil {
			c.JSON(500, gin.H{"code": 50000, "message": "页面渲染失败", "data": nil})
		}
	})

	// SEO：robots 与站点地图
	r.GET("/robots.txt", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(200, "text/plain; charset=utf-8", []byte("User-agent: *\nAllow: /\n\nSitemap: "+baseURL+"/sitemap.xml\n"))
	})
	r.GET("/sitemap.xml", func(c *gin.Context) {
		today := time.Now().Format("2006-01-02")
		xml := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
			`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n" +
			`  <url><loc>` + baseURL + `/</loc><lastmod>` + today + `</lastmod><changefreq>weekly</changefreq><priority>1.0</priority></url>` + "\n" +
			`  <url><loc>` + baseURL + `/download</loc><lastmod>` + today + `</lastmod><changefreq>weekly</changefreq><priority>0.8</priority></url>` + "\n" +
			`</urlset>` + "\n"
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(200, "application/xml; charset=utf-8", []byte(xml))
	})

	// 官网静态资源（css/svg）。文件不带内容哈希，给短缓存兼顾更新及时性
	r.GET("/site/*filepath", func(c *gin.Context) {
		p := strings.TrimPrefix(c.Param("filepath"), "/")
		if p == "" || strings.Contains(p, "..") || strings.HasSuffix(p, ".html") {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在", "data": nil})
			return
		}
		data, err := sitetpl.WebsiteFile(p)
		if err != nil {
			c.JSON(404, gin.H{"code": 40400, "message": "资源不存在", "data": nil})
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(200, siteContentType(p), data)
	})

	// 发布物文件下载（安装包附件 / 小程序码图片）
	r.GET("/api/public/download/app/:id", siteCtl.Download)

	// 下载页二维码（内容为官网下载页地址，手机扫码打开 /download 再点下载，兼容性最好）
	r.GET("/api/public/download/qr", func(c *gin.Context) {
		png, err := qrcode.Encode(baseURL+"/download", qrcode.Medium, 256)
		if err != nil {
			c.JSON(500, gin.H{"code": 50000, "message": "二维码生成失败", "data": nil})
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(200, "image/png", png)
	})
}

// buildChannels 按固定顺序组装下载页渠道卡片（未上传发布物的渠道显示「暂未开放」）。
func buildChannels(latest map[string]model.AppRelease) []siteChannel {
	defs := []siteChannel{
		{Key: "android", Badge: "推荐", Name: "Android 版", OS: "适用于安卓手机 / 平板", PageQR: true},
		{Key: "wechat_mp", Name: "微信小程序", OS: "微信扫码，免安装使用", MpQR: true},
		{Key: "harmony", Name: "HarmonyOS 版", OS: "适用于鸿蒙手机"},
		{Key: "ios", Name: "iOS 版", OS: "适用于 iPhone"},
	}
	for i := range defs {
		ch := &defs[i]
		rel, ok := latest[ch.Key]
		if !ok {
			continue
		}
		ch.Available = true
		ch.URL = "/api/public/download/app/" + rel.ID
		parts := []string{}
		if rel.Version != "" {
			parts = append(parts, "v"+rel.Version)
		}
		if rel.Size > 0 {
			parts = append(parts, formatBytes(rel.Size))
		}
		parts = append(parts, "更新于 "+rel.CreatedAt.Format("2006-01-02"))
		ch.Meta = strings.Join(parts, " · ")
	}
	return defs
}

// formatBytes 人类可读文件大小。
func formatBytes(n int64) string {
	if n >= 1<<20 {
		return strconv.FormatFloat(float64(n)/1048576, 'f', 1, 64) + " MB"
	}
	return strconv.FormatInt(n/1024, 10) + " KB"
}

// darkenHex 主题色按比例压深（按钮悬停态）；非法输入回退品牌蓝。
func darkenHex(hex string, factor float64) string {
	if hex == "" {
		hex = "#2b5aed"
	}
	var r, g, b int64
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return "#1f47c8"
	}
	return fmt.Sprintf("#%02x%02x%02x", int64(float64(r)*factor), int64(float64(g)*factor), int64(float64(b)*factor))
}

// siteJSONLD 结构化数据（GEO/SEO：Organization + WebSite），联系方式按配置填充。
func siteJSONLD(baseURL string, c map[string]string) template.JS {
	org := map[string]any{
		"@type": "Organization",
		"name":  "安巡云 AnXunCloud",
		"url":   baseURL + "/",
		"logo":  baseURL + "/site/assets/anxuncloud-mark.svg",
	}
	if c["company_name"] != "" {
		org["legalName"] = c["company_name"]
	}
	if c["contact_phone"] != "" || c["contact_email"] != "" {
		cp := map[string]any{"@type": "ContactPoint", "contactType": "sales"}
		if c["contact_phone"] != "" {
			cp["telephone"] = c["contact_phone"]
		}
		if c["contact_email"] != "" {
			cp["email"] = c["contact_email"]
		}
		org["contactPoint"] = cp
	}
	if c["address"] != "" {
		org["address"] = map[string]any{"@type": "PostalAddress", "streetAddress": c["address"], "addressCountry": "CN"}
	}
	graph := map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			org,
			map[string]any{
				"@type":       "WebSite",
				"name":        "安巡云 AnXunCloud · 物业巡检管理平台",
				"url":         baseURL + "/",
				"description": "面向物业公司的巡检管理平台：二维码 / NFC / GPS 围栏三重到点校验，拍照留证、AI 识别异常、月度报告电子签名。",
				"inLanguage":  "zh-CN",
			},
		},
	}
	b, err := json.Marshal(graph)
	if err != nil {
		return ""
	}
	return template.JS(b)
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
