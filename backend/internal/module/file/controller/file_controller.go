// Package controller 统一文件层 HTTP 接口（/api/files）。
package controller

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/file/service"
	mpdto "anxuncloud/internal/module/mp/dto"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// FileController 统一文件接口。
type FileController struct {
	svc *service.FileService
}

func NewFileController(svc *service.FileService) *FileController {
	return &FileController{svc: svc}
}

// Upload POST /api/files（multipart：file 文件 + scene 场景）
func (ctl *FileController) Upload(c *gin.Context) {
	scene := c.PostForm("scene")
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("缺少上传文件 file"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	data, be := ctl.svc.Upload(c, scene, fh.Filename, fh.Size, f)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// STS POST /api/files/sts（直传凭证签发；云模式返回凭证，local 返回本地上传入口）
func (ctl *FileController) STS(c *gin.Context) {
	var req mpdto.STSReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.STS(middleware.CurrentUserID(c), &req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// Download GET /api/files/*key（inline 预览；?download=1 强制附件下载，文件名为原始名称）
func (ctl *FileController) Download(c *gin.Context) {
	data, redirect, name, mime, be := ctl.svc.Download(c, c.Param("key"))
	if be != nil {
		response.Fail(c, be)
		return
	}
	if redirect != "" {
		c.Redirect(http.StatusFound, redirect)
		return
	}
	// 图片/PDF 默认内联预览，?download=1 强制附件；其余类型一律按附件给原始文件名
	disposition := "attachment"
	if c.Query("download") != "1" && (mime == "application/pdf" || len(mime) >= 6 && mime[:6] == "image/") {
		disposition = "inline"
	}
	if name == "" {
		name = "file"
	}
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(name)))
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, mime, data)
}
