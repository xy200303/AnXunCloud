package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	mpsvc "anxuncloud/internal/module/mp/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// UploadController 管理端图片上传（手写签名、公章、头像等）。
type UploadController struct {
	upload *mpsvc.UploadService
}

func NewUploadController(upload *mpsvc.UploadService) *UploadController {
	return &UploadController{upload: upload}
}

// Local POST /system/upload（登录即可，multipart：scene + file）。
// scene 取值：signature（手写签名）/ seal（公章）/ avatar（头像）。
func (ctl *UploadController) Local(c *gin.Context) {
	scene := c.PostForm("scene")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("缺少上传文件 file"))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	data, be := ctl.upload.SaveAdminLocal(middleware.CurrentUserID(c), scene, fileHeader.Filename, fileHeader.Size, f)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}
