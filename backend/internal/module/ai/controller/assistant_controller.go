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

type AssistantController struct {
	svc *service.AssistantService
	db  *gorm.DB
}

func NewAssistantController(svc *service.AssistantService, db *gorm.DB) *AssistantController {
	return &AssistantController{svc: svc, db: db}
}

func (ctl *AssistantController) actor(c *gin.Context) (service.Actor, *errs.Error) {
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.Actor{}, be
	}
	return service.Actor{TenantID: tenantID, UserID: middleware.CurrentUserID(c)}, nil
}

func (ctl *AssistantController) Status(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Status(actor)
	responseData(c, data, be)
}

func (ctl *AssistantController) Conversations(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Conversations(actor)
	responseData(c, data, be)
}

func (ctl *AssistantController) Messages(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Messages(actor, c.Param("id"))
	responseData(c, data, be)
}

func (ctl *AssistantController) Chat(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req service.ChatRequest
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Chat(c.Request.Context(), actor, req)
	responseData(c, data, be)
}

func responseData(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}
