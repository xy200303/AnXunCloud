package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// SignAssetController 签章资产管理接口（手写签名/公章版本链）。
type SignAssetController struct {
	svc *service.SignAssetService
}

func NewSignAssetController(svc *service.SignAssetService) *SignAssetController {
	return &SignAssetController{svc: svc}
}

// List GET /system/sign-assets
func (ctl *SignAssetController) List(c *gin.Context) {
	var q dto.SignAssetQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.List(&q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// Create POST /system/sign-assets（创建即 active，同 type+owner 原 active 自动置 replaced）
func (ctl *SignAssetController) Create(c *gin.Context) {
	var req dto.SignAssetCreateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	item, be := ctl.svc.Create(middleware.CurrentUserID(c), &req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, item)
}

// Revoke POST /system/sign-assets/:id/revoke（reason 必填；仅 active 可作废）
func (ctl *SignAssetController) Revoke(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.SignAssetRevokeReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Revoke(id, req.Reason); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
