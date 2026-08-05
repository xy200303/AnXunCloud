// Package controller 统计报表接口 HTTP 层。
package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"property-inspection/internal/module/stats/dto"
	"property-inspection/internal/module/stats/service"
	"property-inspection/internal/pkg/bind"
	"property-inspection/internal/pkg/errs"
	"property-inspection/internal/pkg/response"
)

// StatsController 统计报表 + 工作台接口。
type StatsController struct {
	svc *service.StatsService
}

func NewStatsController(svc *service.StatsService) *StatsController {
	return &StatsController{svc: svc}
}

func write(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// Coverage GET /stats/coverage
func (ctl *StatsController) Coverage(c *gin.Context) {
	var q dto.ReportQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Coverage(c, &q)
	write(c, data, be)
}

// Timeliness GET /stats/timeliness
func (ctl *StatsController) Timeliness(c *gin.Context) {
	var q dto.ReportQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Timeliness(c, &q)
	write(c, data, be)
}

// Performance GET /stats/performance
func (ctl *StatsController) Performance(c *gin.Context) {
	var q dto.PerformanceQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.Performance(c, &q)
	write(c, page, be)
}

// Export POST /stats/export
func (ctl *StatsController) Export(c *gin.Context) {
	var req dto.ExportReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Export(c, &req)
	write(c, data, be)
}

// Dashboard GET /dashboard
func (ctl *StatsController) Dashboard(c *gin.Context) {
	communityID, _ := strconv.ParseInt(c.Query("community_id"), 10, 64)
	data, be := ctl.svc.Dashboard(c, communityID)
	write(c, data, be)
}
