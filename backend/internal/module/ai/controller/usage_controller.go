package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/ai/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

type UsageController struct {
	svc *service.UsageService
	db  *gorm.DB
}

func NewUsageController(svc *service.UsageService, db *gorm.DB) *UsageController {
	return &UsageController{svc: svc, db: db}
}

func (ctl *UsageController) actor(c *gin.Context) (service.Actor, *errs.Error) {
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.Actor{}, be
	}
	return service.Actor{TenantID: tenantID, UserID: middleware.CurrentUserID(c)}, nil
}

func (ctl *UsageController) Usage(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Usage(actor)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

func (ctl *UsageController) SaveQuota(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req service.QuotaReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.SaveQuota(actor, c.Param("taskType"), req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

func (ctl *UsageController) ResetQuota(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.ResetQuota(actor, c.Param("taskType")); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
