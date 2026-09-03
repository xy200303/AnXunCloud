// 项目岗位编制与职责槽位绑定接口 HTTP 层。
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

// PostDict 编制表单岗位下拉（GET /post-dict?community_id=）。
func (ctl *StaffController) PostDict(c *gin.Context) {
	items, be := ctl.svc.ListPostDict(c, c.Query("community_id"))
	write(c, items, be)
}

// DutyBindings 项目职责槽位绑定视图（GET /communities/:id/duty-bindings）。
func (ctl *StaffController) DutyBindings(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	items, be := ctl.svc.ListDutyBindings(c, id)
	write(c, items, be)
}

// SaveDutyBindings 保存项目级槽位绑定（PUT /communities/:id/duty-bindings）。
func (ctl *StaffController) SaveDutyBindings(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.DutyBindingsSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.SaveDutyBindings(c, id, &req))
}

// GetReviewFlow 项目打卡审核链视图（GET /communities/:id/review-flow）。
func (ctl *StaffController) GetReviewFlow(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.GetReviewFlow(c, id)
	write(c, data, be)
}

// SaveReviewFlow 保存项目级打卡审核链覆盖（PUT /communities/:id/review-flow）。
func (ctl *StaffController) SaveReviewFlow(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req struct {
		Steps types.FlowStepArray `json:"steps" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.SaveReviewFlow(c, id, req.Steps))
}
