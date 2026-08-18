package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// SignAssetController 签章资产管理接口（手写签名/公章版本链）。
// db 用于解析租户上下文（middleware.EffectiveTenantID）。
type SignAssetController struct {
	svc *service.SignAssetService
	db  *gorm.DB
}

func NewSignAssetController(svc *service.SignAssetService, db *gorm.DB) *SignAssetController {
	return &SignAssetController{svc: svc, db: db}
}

// List GET /system/sign-assets（按租户上下文隔离；超管可 ?tenant_id= 切换）
func (ctl *SignAssetController) List(c *gin.Context) {
	var q dto.SignAssetQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.List(&q, tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// Create POST /system/sign-assets（创建即 active，同租户+type+owner 原 active 自动置 replaced）
func (ctl *SignAssetController) Create(c *gin.Context) {
	var req dto.SignAssetCreateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	item, be := ctl.svc.Create(middleware.CurrentUserID(c), tid, &req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, item)
}

// Revoke POST /system/sign-assets/:id/revoke（reason 必填；仅 active 可作废；跨租户 404）
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
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Revoke(id, req.Reason, tid); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
