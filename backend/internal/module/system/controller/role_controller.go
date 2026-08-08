package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// RoleController 角色管理接口。
type RoleController struct {
	svc *service.RoleService
}

func NewRoleController(svc *service.RoleService) *RoleController {
	return &RoleController{svc: svc}
}

// List GET /system/roles
func (ctl *RoleController) List(c *gin.Context) {
	var q dto.RoleListQuery
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

// Create POST /system/roles
func (ctl *RoleController) Create(c *gin.Context) {
	var req dto.RoleSaveReq
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

// Detail GET /system/roles/:id
func (ctl *RoleController) Detail(c *gin.Context) {
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

// Update PUT /system/roles/:id
func (ctl *RoleController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.RoleSaveReq
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

// Delete DELETE /system/roles/:id
func (ctl *RoleController) Delete(c *gin.Context) {
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

// AssignMenus PUT /system/roles/:id/menus
func (ctl *RoleController) AssignMenus(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.RoleAssignReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.AssignMenus(id, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
