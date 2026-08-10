// Package controller 巡检模块接口 HTTP 层（点位/计划/任务/打卡记录）。
package controller

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// InspectionController 巡检模块接口。
type InspectionController struct {
	points *service.PointService
	plans  *service.PlanService
	tasks  *service.TaskService
}

func NewInspectionController(points *service.PointService, plans *service.PlanService, tasks *service.TaskService) *InspectionController {
	return &InspectionController{points: points, plans: plans, tasks: tasks}
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

// ========== 点位 ==========

func (ctl *InspectionController) ListPoints(c *gin.Context) {
	var q dto.PointListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.points.List(c, &q)
	write(c, page, be)
}

func (ctl *InspectionController) CreatePoint(c *gin.Context) {
	var req dto.PointSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, no, be := ctl.points.Create(c, &req)
	write(c, gin.H{"id": id, "qrcode_no": no}, be)
}

func (ctl *InspectionController) PointDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.points.Detail(c, id)
	write(c, data, be)
}

func (ctl *InspectionController) UpdatePoint(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.PointSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.points.Update(c, id, &req))
}

func (ctl *InspectionController) DeletePoint(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.points.Delete(c, id))
}

func (ctl *InspectionController) QRCodes(c *gin.Context) {
	var req dto.QRCodeBatchReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.points.QRCodeBatch(c, &req)
	write(c, data, be)
}

func (ctl *InspectionController) MapPoints(c *gin.Context) {
	data, be := ctl.points.MapPoints(c, c.Query("community_id"))
	write(c, data, be)
}

// ========== 计划 ==========

func (ctl *InspectionController) ListPlans(c *gin.Context) {
	var q dto.PlanListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.plans.List(c, &q)
	write(c, page, be)
}

func (ctl *InspectionController) CreatePlan(c *gin.Context) {
	var req dto.PlanSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.plans.Create(c, &req)
	write(c, gin.H{"id": id}, be)
}

func (ctl *InspectionController) PlanDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.plans.Detail(c, id)
	write(c, data, be)
}

func (ctl *InspectionController) UpdatePlan(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.PlanSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.plans.Update(c, id, &req))
}

func (ctl *InspectionController) DeletePlan(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.plans.Delete(c, id))
}

func (ctl *InspectionController) SetPlanStatus(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req struct {
		Status int `json:"status" binding:"min=0,max=1"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.plans.SetStatus(c, id, req.Status))
}

// ========== 任务 ==========

func (ctl *InspectionController) ListTasks(c *gin.Context) {
	var q dto.TaskListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.tasks.List(c, &q)
	write(c, page, be)
}

func (ctl *InspectionController) TaskDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.tasks.Detail(c, id)
	write(c, data, be)
}

// GenerateTasks 手动触发任务生成（默认今天；供联调与补偿，幂等）。
func (ctl *InspectionController) GenerateTasks(c *gin.Context) {
	var req dto.GenerateTaskReq
	_ = c.ShouldBindJSON(&req)
	date := time.Now()
	if req.Date != "" {
		t, err := time.ParseInLocation("2006-01-02", req.Date, time.Local)
		if err != nil {
			response.Fail(c, errs.ErrParam.WithMsg("date 格式应为 YYYY-MM-DD"))
			return
		}
		date = t
	}
	created, be := ctl.plans.GenerateForDate(c.Request.Context(), date)
	write(c, gin.H{"date": date.Format("2006-01-02"), "created": created}, be)
}

// ========== 打卡记录 ==========

func (ctl *InspectionController) ListCheckins(c *gin.Context) {
	var q dto.CheckinListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.tasks.CheckinList(c, &q)
	write(c, page, be)
}

// CheckinAuditCounts 各审核状态计数（列表页 tab 徽章）。
func (ctl *InspectionController) CheckinAuditCounts(c *gin.Context) {
	var q dto.CheckinListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.tasks.CheckinAuditCounts(c, &q)
	write(c, data, be)
}

func (ctl *InspectionController) CheckinDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.tasks.CheckinDetail(c, id)
	write(c, data, be)
}
