package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// TemplateController 检查项模板接口。
type TemplateController struct {
	templates *service.TemplateService
}

func NewTemplateController(templates *service.TemplateService) *TemplateController {
	return &TemplateController{templates: templates}
}

func (ctl *TemplateController) List(c *gin.Context) {
	var q dto.TemplateListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.templates.List(&q)
	write(c, page, be)
}

func (ctl *TemplateController) Create(c *gin.Context) {
	var req dto.TemplateSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.templates.Create(&req)
	write(c, gin.H{"id": id}, be)
}

func (ctl *TemplateController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.templates.Detail(id)
	write(c, data, be)
}

func (ctl *TemplateController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.TemplateSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.templates.Update(id, &req))
}

func (ctl *TemplateController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.templates.Delete(id))
}
