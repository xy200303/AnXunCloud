package controller

import (
	"github.com/gin-gonic/gin"

	"property-inspection/internal/module/system/dto"
	"property-inspection/internal/module/system/service"
	"property-inspection/internal/pkg/bind"
	"property-inspection/internal/pkg/response"
)

// ConfigController 参数配置接口。
type ConfigController struct {
	svc *service.ConfigService
}

func NewConfigController(svc *service.ConfigService) *ConfigController {
	return &ConfigController{svc: svc}
}

// List GET /system/configs
func (ctl *ConfigController) List(c *gin.Context) {
	var q dto.ConfigQuery
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

// Create POST /system/configs
func (ctl *ConfigController) Create(c *gin.Context) {
	var req dto.ConfigSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.Create(&req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// Update PUT /system/configs/:id
func (ctl *ConfigController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.ConfigSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Update(id, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// Delete DELETE /system/configs/:id
func (ctl *ConfigController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Delete(id); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
