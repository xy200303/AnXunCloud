package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/middleware"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// LogController 日志检索接口（只读）。
type LogController struct {
	svc *service.LogService
}

func NewLogController(svc *service.LogService) *LogController {
	return &LogController{svc: svc}
}

// Operations GET /system/logs/operations
func (ctl *LogController) Operations(c *gin.Context) {
	var q dto.OperationLogQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.OperationList(&q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// MyLoginLogs GET /system/users/my-login-logs（登录即可，仅本人记录）
func (ctl *LogController) MyLoginLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	list, be := ctl.svc.MyLoginLogs(middleware.CurrentUserID(c), limit)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, list)
}

// Logins GET /system/logs/logins
func (ctl *LogController) Logins(c *gin.Context) {
	var q dto.LoginLogQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.LoginList(&q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}
