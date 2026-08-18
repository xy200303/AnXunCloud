// 租户管理与租户配置覆盖接口（P3 多租户）。
package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// TenantController 租户管理与租户配置接口。db 用于解析租户上下文（middleware.EffectiveTenantID）。
type TenantController struct {
	svc *service.TenantService
	db  *gorm.DB
}

func NewTenantController(svc *service.TenantService, db *gorm.DB) *TenantController {
	return &TenantController{svc: svc, db: db}
}

// List GET /api/admin/tenants（仅超管，tenant:list）
func (ctl *TenantController) List(c *gin.Context) {
	var q dto.TenantListQuery
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

// Create POST /api/admin/tenants（仅超管，tenant:create）：开通租户 + 初始管理员账号
func (ctl *TenantController) Create(c *gin.Context) {
	var req dto.TenantCreateReq
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

// Update PUT /api/admin/tenants/:id（仅超管，tenant:update）
func (ctl *TenantController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.TenantUpdateReq
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

// SetStatus PUT /api/admin/tenants/:id/status（仅超管，tenant:update）：停用/启用
func (ctl *TenantController) SetStatus(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.StatusReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.SetStatus(c.Request.Context(), id, req.Status); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// GetConfig GET /api/admin/tenant-config?tenant_id=（tenant:config；企业品牌页，租户上下文解析）
func (ctl *TenantController) GetConfig(c *gin.Context) {
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	items, be := ctl.svc.GetConfig(tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, items)
}

// SaveConfig PUT /api/admin/tenant-config?tenant_id=  body: {"values":{"site.company_name":"..."}}
// 目标租户走租户上下文（query/header），不再从 body 取 tenant_id。
func (ctl *TenantController) SaveConfig(c *gin.Context) {
	var req struct {
		Values map[string]string `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Values) == 0 {
		response.Fail(c, errs.ErrParam)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.SaveConfig(tid, req.Values); be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, "已保存", nil)
}
