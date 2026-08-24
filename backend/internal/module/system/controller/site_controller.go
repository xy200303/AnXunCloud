// 品牌官网管理 + 官网公开数据接口。
package controller

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
)

// SiteController 品牌官网。
type SiteController struct {
	svc   *systemsvc.SiteService
	store *storage.Storage
}

func NewSiteController(svc *systemsvc.SiteService, store *storage.Storage) *SiteController {
	return &SiteController{svc: svc, store: store}
}

// ========== 管理端 ==========

// Config GET /api/admin/system/site/config
func (ctl *SiteController) Config(c *gin.Context) {
	data, be := ctl.svc.BrandConfig()
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// SaveConfig PUT /api/admin/system/site/config  body: {"values": {"site.slogan": "..."}}
func (ctl *SiteController) SaveConfig(c *gin.Context) {
	var req struct {
		Values map[string]string `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Values) == 0 {
		response.Fail(c, errs.ErrParam)
		return
	}
	if be := ctl.svc.SaveBrandConfig(req.Values); be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, "已保存", nil)
}

// Releases GET /api/admin/system/site/releases
func (ctl *SiteController) Releases(c *gin.Context) {
	data, be := ctl.svc.ListReleases()
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// Upload POST /api/admin/system/site/releases（multipart：platform/version/note/file）
func (ctl *SiteController) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, systemsvc.ReleaseSizeLimit+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("请选择文件（≤512MB）"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errs.ErrParam)
		return
	}
	defer f.Close()
	rel, be := ctl.svc.UploadRelease(
		middleware.CurrentUserID(c),
		c.PostForm("platform"), c.PostForm("version"), c.PostForm("note"),
		filepath.Base(fh.Filename), fh.Size, f,
	)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, "上传成功", rel)
}

// Delete DELETE /api/admin/system/site/releases/:id
func (ctl *SiteController) Delete(c *gin.Context) {
	if be := ctl.svc.DeleteRelease(c.Param("id")); be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, "已删除", nil)
}

// ========== 官网公开接口 ==========

// Download GET /api/public/download/app/:id（安装包附件下载 / 小程序码图片内联预览）
func (ctl *SiteController) Download(c *gin.Context) {
	rel, be := ctl.svc.ReleaseFile(c.Param("id"))
	if be != nil {
		response.Fail(c, be)
		return
	}
	isImage := rel.Platform == "wechat_mp"
	// 本地驱动直接发文件（支持大文件断点续传友好）；云驱动读回转发（当前实现上限 64MB）
	if ctl.store.IsLocal() {
		if !isImage {
			c.Header("Content-Type", "application/octet-stream")
			c.Header("Content-Disposition", contentDisposition(rel.Name))
		}
		c.File(ctl.store.LocalPath(rel.FileKey))
		return
	}
	data, err := ctl.store.ReadFile(rel.FileKey)
	if err != nil {
		response.Fail(c, errs.ErrNotFound.WithMsg("文件不存在或已删除"))
		return
	}
	if isImage {
		c.Data(200, "image/"+strings.TrimPrefix(filepath.Ext(rel.Name), "."), data)
		return
	}
	c.Header("Content-Disposition", contentDisposition(rel.Name))
	c.Data(200, "application/octet-stream", data)
}

// contentDisposition 附件下载头（ASCII 兜底文件名 + RFC 5987 UTF-8 filename*）。
func contentDisposition(name string) string {
	if name == "" {
		name = "download"
	}
	var b strings.Builder
	for _, r := range name {
		if r < 128 && r != '"' && r != '\\' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return "attachment; filename=\"" + b.String() + "\"; filename*=UTF-8''" + url.PathEscape(name)
}
