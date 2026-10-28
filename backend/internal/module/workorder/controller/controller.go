package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

type WorkOrderController struct {
	svc *service.WorkOrderService
}

func NewWorkOrderController(svc *service.WorkOrderService) *WorkOrderController {
	return &WorkOrderController{svc: svc}
}

func write(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

func pathID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", errs.ErrParam.WithMsg("工单 ID 无效")
	}
	return id, nil
}

func (ctl *WorkOrderController) List(c *gin.Context) {
	var query service.ListQuery
	if be := bind.Query(c, &query); be != nil {
		write(c, nil, be)
		return
	}
	page, be := ctl.svc.List(c, &query)
	write(c, page, be)
}

func (ctl *WorkOrderController) Candidates(c *gin.Context) {
	items, be := ctl.svc.AssignCandidates(c, c.Query("community_id"), c.Query("work_type"))
	write(c, items, be)
}

func (ctl *WorkOrderController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		write(c, nil, be)
		return
	}
	data, be := ctl.svc.Detail(c, id)
	write(c, data, be)
}

func (ctl *WorkOrderController) Claim(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		write(c, nil, be)
		return
	}
	write(c, nil, ctl.svc.Claim(c, id))
}

func (ctl *WorkOrderController) Assign(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		write(c, nil, be)
		return
	}
	var req service.AssignReq
	if be := bind.JSON(c, &req); be != nil {
		write(c, nil, be)
		return
	}
	write(c, nil, ctl.svc.Assign(c, id, req))
}

func (ctl *WorkOrderController) Finish(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		write(c, nil, be)
		return
	}
	var req service.FinishReq
	if be := bind.JSON(c, &req); be != nil {
		write(c, nil, be)
		return
	}
	write(c, nil, ctl.svc.Finish(c, id, req))
}

func (ctl *WorkOrderController) Review(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		write(c, nil, be)
		return
	}
	var req service.ReviewReq
	if be := bind.JSON(c, &req); be != nil {
		write(c, nil, be)
		return
	}
	write(c, nil, ctl.svc.Review(c, id, req))
}

func (ctl *WorkOrderController) Close(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		write(c, nil, be)
		return
	}
	var req service.CloseReq
	if be := bind.JSON(c, &req); be != nil {
		write(c, nil, be)
		return
	}
	write(c, nil, ctl.svc.Close(c, id, req))
}

func (ctl *WorkOrderController) Merge(c *gin.Context) {
	var req service.MergeReq
	if be := bind.JSON(c, &req); be != nil {
		write(c, nil, be)
		return
	}
	data, be := ctl.svc.Merge(c, req)
	write(c, data, be)
}
