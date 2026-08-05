// Package controller 工单接口 HTTP 层（管理后台）。
package controller

import (

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"property-inspection/internal/middleware"
	"property-inspection/internal/module/workorder/dto"
	"property-inspection/internal/module/workorder/service"
	"property-inspection/internal/pkg/bind"
	"property-inspection/internal/pkg/errs"
	"property-inspection/internal/pkg/response"
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

// Create POST /workorders
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

// Assign POST /workorders/:id/assign
func (ctl *OrderController) Assign(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.AssignReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Assign(c, id, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"status": "assigned"})
}

// Finish POST /workorders/:id/finish（后台代录）
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
	response.OK(c, gin.H{"status": "review"})
}

// Review POST /workorders/:id/review
func (ctl *OrderController) Review(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.ReviewReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	status, be := ctl.svc.Review(c, id, &req)
	write(c, gin.H{"status": status}, be)
}
