package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/types"
)

type PostController struct {
	svc *service.PostService
	db  *gorm.DB
}

func NewPostController(svc *service.PostService, db *gorm.DB) *PostController {
	return &PostController{svc: svc, db: db}
}

func writePost(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

func flowSteps(c *gin.Context) (types.FlowStepArray, *errs.Error) {
	var req struct {
		Steps types.FlowStepArray `json:"steps"`
	}
	if be := bind.JSON(c, &req); be != nil {
		return nil, be
	}
	return req.Steps, nil
}

func (ctl *PostController) GetFlow(c *gin.Context) {
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		writePost(c, nil, be)
		return
	}
	data, be := ctl.svc.GetFlow(&tid, c.Param("flowCode"))
	writePost(c, data, be)
}

func (ctl *PostController) SaveFlow(c *gin.Context) {
	steps, be := flowSteps(c)
	if be != nil {
		writePost(c, nil, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be == nil {
		be = ctl.svc.SaveFlow(&tid, c.Param("flowCode"), steps)
	}
	writePost(c, nil, be)
}

type PostTemplateController struct {
	svc *service.PostService
}

func NewPostTemplateController(svc *service.PostService) *PostTemplateController {
	return &PostTemplateController{svc: svc}
}

func (ctl *PostTemplateController) GetFlow(c *gin.Context) {
	data, be := ctl.svc.GetFlow(nil, c.Param("flowCode"))
	writePost(c, data, be)
}

func (ctl *PostTemplateController) SaveFlow(c *gin.Context) {
	steps, be := flowSteps(c)
	if be == nil {
		be = ctl.svc.SaveFlow(nil, c.Param("flowCode"), steps)
	}
	writePost(c, nil, be)
}
