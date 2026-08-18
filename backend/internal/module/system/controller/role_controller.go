package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// RoleController 角色管理接口。db 用于解析租户上下文（middleware.EffectiveTenantID）。
type RoleController struct {
	svc *service.RoleService
	db  *gorm.DB
}

func NewRoleController(svc *service.RoleService, db *gorm.DB) *RoleController {
	return &RoleController{svc: svc, db: db}
}

// List GET /system/roles（内置角色 + 上下文租户自建角色）
func (ctl *RoleController) List(c *gin.Context) {
	var q dto.RoleListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.List(&q, middleware.CurrentIdentity(c), tid)
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
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.Create(&req, middleware.CurrentIdentity(c), tid)
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
	detail, be := ctl.svc.Detail(id, middleware.CurrentIdentity(c))
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
	if be := ctl.svc.Update(id, &req, middleware.CurrentIdentity(c)); be != nil {
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
	if be := ctl.svc.Delete(id, middleware.CurrentIdentity(c)); be != nil {
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
	if be := ctl.svc.AssignMenus(id, &req, middleware.CurrentIdentity(c)); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
