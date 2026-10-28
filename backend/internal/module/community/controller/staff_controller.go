// 项目任职与审批流程覆盖接口 HTTP 层。
package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/module/community/dto"
	"anxuncloud/internal/module/community/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/types"
)

// StaffController 项目岗位编制/职责绑定接口。
type StaffController struct {
	svc *service.StaffService
}

func NewStaffController(svc *service.StaffService) *StaffController {
	return &StaffController{svc: svc}
}

// pathStaffID 解析路径中的编制记录 ID。
func pathStaffID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("staffId")
	if _, err := uuid.Parse(id); err != nil {
		return "", errs.ErrParam.WithMsg("staffId 须为 UUID")
	}
	return id, nil
}

// List 编制名单（GET /communities/:id/staff）。
func (ctl *StaffController) List(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	items, be := ctl.svc.ListStaff(c, id)
	write(c, items, be)
}

// Create 新增编制成员（POST /communities/:id/staff）。
func (ctl *StaffController) Create(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.StaffSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	staffID, be := ctl.svc.CreateStaff(c, id, &req)
	write(c, gin.H{"id": staffID}, be)
}

// Update 修改编制成员（PUT /communities/:id/staff/:staffId）。
func (ctl *StaffController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	staffID, be := pathStaffID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.StaffSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.UpdateStaff(c, id, staffID, &req))
}

// Delete 移除编制成员（DELETE /communities/:id/staff/:staffId）。
func (ctl *StaffController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	staffID, be := pathStaffID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.DeleteStaff(c, id, staffID))
}

// RoleOptions 返回当前租户可用于任职与审批节点的角色。
func (ctl *StaffController) RoleOptions(c *gin.Context) {
	items, be := ctl.svc.ListRoleOptions(c, c.Query("community_id"))
	write(c, items, be)
}

// FlowPreview 审批链名单预览（GET /communities/:id/flow-preview?flow_code=）。
func (ctl *StaffController) FlowPreview(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.FlowPreview(c, id, c.Query("flow_code"))
	write(c, data, be)
}

func (ctl *StaffController) GetFlow(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.GetFlow(c, id, c.Param("flowCode"))
	write(c, data, be)
}

func (ctl *StaffController) SaveFlow(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req struct {
		Steps types.FlowStepArray `json:"steps"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.SaveFlow(c, id, c.Param("flowCode"), req.Steps))
}
