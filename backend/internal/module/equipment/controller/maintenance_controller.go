package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/model"
	"anxuncloud/internal/module/equipment/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// MaintenanceController 维保登记与确认链接口（admin 端 + mp 端 App 巡检员）。
type MaintenanceController struct {
	maintenance *service.MaintenanceService
}

func NewMaintenanceController(maintenance *service.MaintenanceService) *MaintenanceController {
	return &MaintenanceController{maintenance: maintenance}
}

// Register POST /equipment/maintenance（admin 端登记；与 mp 登记共用同一 service）
// v2.0 同步化：AI 核验可信即自动生效（confirm_status=confirmed），否则 pending 进确认链。
func (ctl *MaintenanceController) Register(c *gin.Context) {
	var req dto.MaintenanceRegisterReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, confirmed, be := ctl.maintenance.Register(c, &req)
	status := model.ConfirmPending
	if confirmed {
		status = model.ConfirmConfirmed
	}
	write(c, gin.H{"id": id, "confirm_status": status}, be)
}

// PendingList GET /equipment/maintenance-pending（待确认列表：review 置顶）
func (ctl *MaintenanceController) PendingList(c *gin.Context) {
	var q dto.ConfirmListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.maintenance.ConfirmList(c, &q)
	write(c, page, be)
}

// Confirm POST /equipment/maintenance/confirm（批量确认，幂等）
func (ctl *MaintenanceController) Confirm(c *gin.Context) {
	var req dto.ConfirmReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.maintenance.Confirm(c, &req)
	write(c, data, be)
}

// Reject POST /equipment/maintenance/reject（驳回 + 理由，通知登记人）
func (ctl *MaintenanceController) Reject(c *gin.Context) {
	var req dto.RejectReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.maintenance.Reject(c, &req))
}

// History GET /equipment/:id/maintenances（某设备维保历史，含确认状态）
func (ctl *MaintenanceController) History(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var q response.PageQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.maintenance.History(c, id, &q)
	write(c, page, be)
}

// ========== mp 端（App 巡检员） ==========

// MpDueDevices GET /mp/equipment/due（本租户临期+逾期在用设备，逾期在前）
func (ctl *MaintenanceController) MpDueDevices(c *gin.Context) {
	data, be := ctl.maintenance.DueDevices(c)
	write(c, data, be)
}

// MpRegister POST /mp/equipment/maintenance（打卡现场一键登记；与 admin 登记同一入口逻辑）
func (ctl *MaintenanceController) MpRegister(c *gin.Context) {
	ctl.Register(c)
}
