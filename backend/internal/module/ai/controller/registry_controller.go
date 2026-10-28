package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/ai/model"
	"anxuncloud/internal/module/ai/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

type RegistryController struct {
	svc *service.RegistryService
	db  *gorm.DB
}

func NewRegistryController(svc *service.RegistryService, db *gorm.DB) *RegistryController {
	return &RegistryController{svc: svc, db: db}
}

func (ctl *RegistryController) scope(c *gin.Context) (service.Scope, *errs.Error) {
	if c.Query("scope") == model.ScopePlatform {
		identity := middleware.CurrentIdentity(c)
		if identity == nil || !identity.SuperAdmin {
			return service.Scope{}, errs.ErrNoPerm
		}
		return service.Scope{Name: model.ScopePlatform, ID: model.PlatformScopeID}, nil
	}
	if c.Query("scope") != model.ScopeTenant {
		return service.Scope{}, errs.ErrParam.WithMsg("scope 必须是 platform 或 tenant")
	}
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.Scope{}, be
	}
	return service.Scope{Name: model.ScopeTenant, ID: tenantID}, nil
}

func writeRegistry(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

func (ctl *RegistryController) Providers(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		writeRegistry(c, nil, be)
		return
	}
	data, be := ctl.svc.Providers(scope)
	writeRegistry(c, data, be)
}

func (ctl *RegistryController) SaveProvider(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		writeRegistry(c, nil, be)
		return
	}
	var req service.ProviderReq
	if be := bind.JSON(c, &req); be != nil {
		writeRegistry(c, nil, be)
		return
	}
	writeRegistry(c, nil, ctl.svc.SaveProvider(scope, c.Param("code"), req))
}

func (ctl *RegistryController) Models(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		writeRegistry(c, nil, be)
		return
	}
	data, be := ctl.svc.Models(scope)
	writeRegistry(c, data, be)
}

func (ctl *RegistryController) SaveModel(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		writeRegistry(c, nil, be)
		return
	}
	var req service.ModelReq
	if be := bind.JSON(c, &req); be != nil {
		writeRegistry(c, nil, be)
		return
	}
	writeRegistry(c, nil, ctl.svc.SaveModel(scope, c.Param("providerCode"), c.Param("modelId"), req))
}

func (ctl *RegistryController) Bindings(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		writeRegistry(c, nil, be)
		return
	}
	data, be := ctl.svc.Bindings(scope)
	writeRegistry(c, data, be)
}

func (ctl *RegistryController) SaveBinding(c *gin.Context) {
	scope, be := ctl.scope(c)
	if be != nil {
		writeRegistry(c, nil, be)
		return
	}
	var req service.BindingReq
	if be := bind.JSON(c, &req); be != nil {
		writeRegistry(c, nil, be)
		return
	}
	writeRegistry(c, nil, ctl.svc.SaveBinding(scope, c.Param("taskType"), req))
}

func (ctl *RegistryController) TaskCatalog(c *gin.Context) {
	writeRegistry(c, service.TaskCatalog(), nil)
}
