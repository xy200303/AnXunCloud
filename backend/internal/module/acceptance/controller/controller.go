package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/module/acceptance/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

type Controller struct {
	batches *service.BatchService
	schemas *service.SchemaService
	db      *gorm.DB
}

func NewController(batches *service.BatchService, schemas *service.SchemaService, db *gorm.DB) *Controller {
	return &Controller{batches: batches, schemas: schemas, db: db}
}

func writeAcceptance(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

func (ctl *Controller) List(c *gin.Context) {
	var query service.BatchQuery
	if be := bind.Query(c, &query); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	data, be := ctl.batches.List(c, &query)
	writeAcceptance(c, data, be)
}

func (ctl *Controller) Create(c *gin.Context) {
	var req service.BatchReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	id, be := ctl.batches.Create(c, req)
	writeAcceptance(c, gin.H{"id": id}, be)
}

func (ctl *Controller) Detail(c *gin.Context) {
	data, be := ctl.batches.Detail(c, c.Param("id"))
	writeAcceptance(c, data, be)
}

func (ctl *Controller) Start(c *gin.Context) {
	writeAcceptance(c, nil, ctl.batches.Start(c, c.Param("id")))
}

func (ctl *Controller) SaveRecord(c *gin.Context) {
	var req service.RecordReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	id, be := ctl.batches.SaveRecord(c, c.Param("id"), req)
	writeAcceptance(c, gin.H{"id": id}, be)
}

func (ctl *Controller) ConfirmCoverage(c *gin.Context) {
	writeAcceptance(c, nil, ctl.batches.ConfirmCoverage(c, c.Param("id"), c.Param("targetId")))
}

func (ctl *Controller) Submit(c *gin.Context) {
	writeAcceptance(c, nil, ctl.batches.Submit(c, c.Param("id")))
}

func (ctl *Controller) schemaScope(c *gin.Context) (service.ConfigScope, *errs.Error) {
	if c.Query("scope") == model.ScopePlatform {
		identity := middleware.CurrentIdentity(c)
		if identity == nil || !identity.SuperAdmin {
			return service.ConfigScope{}, errs.ErrNoPerm
		}
		return service.ConfigScope{Name: model.ScopePlatform, ID: model.PlatformScopeID}, nil
	}
	if c.Query("scope") != model.ScopeTenant {
		return service.ConfigScope{}, errs.ErrParam.WithMsg("scope 必须是 platform 或 tenant")
	}
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.ConfigScope{}, be
	}
	return service.ConfigScope{Name: model.ScopeTenant, ID: tenantID}, nil
}

func (ctl *Controller) Schemas(c *gin.Context) {
	scope, be := ctl.schemaScope(c)
	if be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	data, be := ctl.schemas.List(scope)
	writeAcceptance(c, data, be)
}

func (ctl *Controller) SaveSchema(c *gin.Context) {
	scope, be := ctl.schemaScope(c)
	if be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	var req service.SchemaReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.schemas.SaveSchema(scope, c.Param("code"), req))
}

func (ctl *Controller) SaveRow(c *gin.Context) {
	scope, be := ctl.schemaScope(c)
	if be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	var req service.RowReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.schemas.SaveRow(scope, c.Param("code"), c.Param("rowCode"), req))
}
