package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
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

// Groups GET /system/configs/groups
func (ctl *ConfigController) Groups(c *gin.Context) {
	groups, be := ctl.svc.Groups()
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, groups)
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

// MapConfig GET /map/config — 地图服务公开配置（登录即可，不绑系统配置权限）。
// key 未配置时返回空串，前端据此禁用地图选点并引导去参数配置填写。
func (ctl *ConfigController) MapConfig(c *gin.Context) {
	key, _ := ctl.svc.Get("map.tencent_key")
	response.OK(c, gin.H{"provider": "tencent", "key": key})
}

// MapSearch GET /map/search?keyword=xx[&location=lat,lng] — 地点搜索代理（登录即可）。
// 转发腾讯地点提示 API，避免前端跨域；key 不出内网。
func (ctl *ConfigController) MapSearch(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.Fail(c, errs.ErrParam.WithMsg("keyword 为必填项"))
		return
	}
	list, be := ctl.svc.SearchPlaces(c.Request.Context(), keyword, c.Query("location"))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, list)
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
