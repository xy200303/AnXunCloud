package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/report/dto"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// ListReportPlans GET /reports/plans?community_id=
func (ctl *ReportController) ListReportPlans(c *gin.Context) {
	data, be := ctl.svc.ListReportPlans(c, c.Query("community_id"))
	write(c, data, be)
}

// CreateReportPlan POST /reports/plans
func (ctl *ReportController) CreateReportPlan(c *gin.Context) {
	var req dto.ReportPlanReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.CreateReportPlan(c, &req)
	write(c, data, be)
}

// UpdateReportPlan PUT /reports/plans/:id
func (ctl *ReportController) UpdateReportPlan(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.ReportPlanReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be2 := ctl.svc.UpdateReportPlan(c, id, &req)
	write(c, data, be2)
}

// DeleteReportPlan DELETE /reports/plans/:id
func (ctl *ReportController) DeleteReportPlan(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.DeleteReportPlan(c, id))
}

// RunReportPlanNow POST /reports/plans/:id/run（手动触发：立即生成上一份完整周期报告）
func (ctl *ReportController) RunReportPlanNow(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be2 := ctl.svc.RunReportPlanNow(c, id)
	write(c, data, be2)
}
