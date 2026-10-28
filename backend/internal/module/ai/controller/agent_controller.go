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

type AgentController struct {
	svc *service.AgentService
	db  *gorm.DB
}

func NewAgentController(svc *service.AgentService, db *gorm.DB) *AgentController {
	return &AgentController{svc: svc, db: db}
}

func (ctl *AgentController) actor(c *gin.Context) (service.Actor, *errs.Error) {
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.Actor{}, be
	}
	return service.Actor{TenantID: tenantID, UserID: middleware.CurrentUserID(c)}, nil
}

func (ctl *AgentController) Status(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Status(actor)
	responseData(c, data, be)
}

func (ctl *AgentController) Tools(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Tools(actor)
	responseData(c, data, be)
}

func (ctl *AgentController) SetTool(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req struct {
		Code    string `json:"code" binding:"required"`
		Enabled bool   `json:"enabled"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	responseData(c, nil, ctl.svc.SetTool(actor, req.Code, req.Enabled))
}

func (ctl *AgentController) Chat(c *gin.Context) {
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

func (ctl *AgentController) Calls(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Calls(actor, c.Param("id"))
	responseData(c, data, be)
}
