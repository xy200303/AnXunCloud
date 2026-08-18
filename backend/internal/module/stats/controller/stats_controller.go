// Package controller 统计报表接口 HTTP 层。
package controller

import (

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/stats/dto"
	"anxuncloud/internal/module/stats/service"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
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
	// 操作/登录日志属敏感数据，路由只挂了 stats:export，此处按类型追加日志导出权限点校验
	if req.ReportType == "operation_log" || req.ReportType == "login_log" {
		ok, err := authz.EnforceAny(middleware.CurrentUserID(c), "system:log:export")
		if err != nil || !ok {
			response.Fail(c, errs.ErrNoPerm.WithMsg("导出日志需要日志导出权限"))
			return
		}
	}
	data, be := ctl.svc.Export(c, &req)
	write(c, data, be)
}

// Dashboard GET /dashboard
func (ctl *StatsController) Dashboard(c *gin.Context) {
	data, be := ctl.svc.Dashboard(c, c.Query("community_id"))
	write(c, data, be)
}
