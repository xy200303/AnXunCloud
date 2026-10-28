package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/report/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

type DocTemplateController struct {
	svc *service.DocTemplateService
	db  *gorm.DB
}

func NewDocTemplateController(svc *service.DocTemplateService, db *gorm.DB) *DocTemplateController {
	return &DocTemplateController{svc: svc, db: db}
}

func (ctl *DocTemplateController) scope(c *gin.Context) (service.DocTemplateScope, *errs.Error) {
	if c.Query("scope") == "platform" {
		identity := middleware.CurrentIdentity(c)
		if identity == nil || !identity.SuperAdmin {
			return service.DocTemplateScope{}, errs.ErrNoPerm
		}
		return service.DocTemplateScope{Platform: true}, nil
	}
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.DocTemplateScope{}, be
	}
	return service.DocTemplateScope{TenantID: tenantID}, nil
}

func (ctl *DocTemplateController) List(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	items, be := ctl.svc.List(scope)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, items)
}

func (ctl *DocTemplateController) StylePacks(c *gin.Context) {
	rows, be := ctl.svc.StylePacks()
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, rows)
}

func (ctl *DocTemplateController) Bindings(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	rows, be := ctl.svc.Bindings(scope, c.Param("id"))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, rows)
}

func (ctl *DocTemplateController) SaveBindings(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req []service.TemplateOutputBindingReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.SaveBindings(scope, c.Param("id"), req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

func (ctl *DocTemplateController) Save(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req service.DocTemplateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Save(scope, c.Param("id"), req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
