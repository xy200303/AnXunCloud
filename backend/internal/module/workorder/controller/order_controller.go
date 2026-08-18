// Package controller 工单接口 HTTP 层（管理后台）。
package controller

import (

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/workorder/dto"
	"anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// OrderController 工单接口。
type OrderController struct {
	svc *service.OrderService
}

func NewOrderController(svc *service.OrderService) *OrderController {
	return &OrderController{svc: svc}
}

func pathID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", errs.ErrParam.WithMsg("id 须为 UUID")
	}
	return id, nil
}

func write(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// List GET /workorders（data 内含 status_counts 角标）
func (ctl *OrderController) List(c *gin.Context) {
	var q dto.OrderListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, counts, be := ctl.svc.List(c, &q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{
		"status_counts": counts,
		"list":          page.List,
		"total":         page.Total,
		"page":          page.Page,
		"page_size":     page.PageSize,
	})
}

// Create POST /workorders（前台代录，source=frontdesk）
func (ctl *OrderController) Create(c *gin.Context) {
	var req dto.OrderCreateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Create(c.Request.Context(), c, &req)
	write(c, data, be)
}

// Detail GET /workorders/:id
func (ctl *OrderController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Detail(c, id)
	write(c, data, be)
}

// Update PUT /workorders/:id
func (ctl *OrderController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.OrderUpdateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.Update(c, id, &req))
}

// Delete DELETE /workorders/:id
func (ctl *OrderController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.Delete(c, id))
}

// Triage POST /workorders/:id/triage（分诊：通过=可选优先级+分类 / 驳回=原因）
func (ctl *OrderController) Triage(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.TriageReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	status, be := ctl.svc.Triage(c, id, &req)
	write(c, gin.H{"status": status}, be)
}

// Dispatch POST /workorders/:id/dispatch（派单：assignee 须为 order_accept 槽位成员）
func (ctl *OrderController) Dispatch(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.DispatchReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Dispatch(c, id, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"status": "processing"})
}

// Finish POST /workorders/:id/finish（后台代录完工）
func (ctl *OrderController) Finish(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.FinishReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	o, be2 := ctl.svc.GetChecked(c, id)
	if be2 != nil {
		response.Fail(c, be2)
		return
	}
	if be := ctl.svc.Finish(middleware.CurrentUserID(c), o, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"status": "pending_confirm"})
}

// Confirm POST /workorders/:id/confirm（验收：通过=闭环 / 不通过=退回处理中）
func (ctl *OrderController) Confirm(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.ConfirmReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	o, be := ctl.svc.GetChecked(c, id)
	if be != nil {
		response.Fail(c, be)
		return
	}
	status, be := ctl.svc.Confirm(middleware.CurrentUserID(c), o, req.Result, req.ConfirmNote)
	write(c, gin.H{"status": status}, be)
}
