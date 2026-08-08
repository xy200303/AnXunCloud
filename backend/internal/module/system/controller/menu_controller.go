package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// MenuController 菜单管理接口。
type MenuController struct {
	svc *service.MenuService
}

func NewMenuController(svc *service.MenuService) *MenuController {
	return &MenuController{svc: svc}
}

// Tree GET /system/menus
func (ctl *MenuController) Tree(c *gin.Context) {
	var q dto.MenuListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	tree, be := ctl.svc.Tree(&q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, tree)
}

// Create POST /system/menus
func (ctl *MenuController) Create(c *gin.Context) {
	var req dto.MenuSaveReq
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

// Detail GET /system/menus/:id
func (ctl *MenuController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	detail, be := ctl.svc.Detail(id)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, detail)
}

// Update PUT /system/menus/:id
func (ctl *MenuController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.MenuSaveReq
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

// Delete DELETE /system/menus/:id
func (ctl *MenuController) Delete(c *gin.Context) {
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
