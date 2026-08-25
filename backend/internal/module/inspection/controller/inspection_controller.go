// Package controller 巡检模块接口 HTTP 层（点位/计划/任务/打卡记录）。
package controller

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/excel"
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

// pointImportMaxFileSize 导入文件大小上限 5MB。
const pointImportMaxFileSize = 5 << 20

// PointImportTemplate GET /inspection/points/import-template（直接返回 Excel 文件流）
func (ctl *InspectionController) PointImportTemplate(c *gin.Context) {
	f, err := excel.PointImportTemplate()
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	writeExcel(c, "point_import_template.xlsx", buf.Bytes())
}

// ImportPoints POST /inspection/points/import（multipart 上传 .xlsx）
func (ctl *InspectionController) ImportPoints(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("缺少上传文件 file"))
		return
	}
	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".xlsx") {
		response.Fail(c, errs.ErrImportFileType)
		return
	}
	if fileHeader.Size > pointImportMaxFileSize {
		response.Fail(c, errs.ErrParam.WithMsg("导入文件不能超过 5MB"))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	result, msg, be := ctl.points.Import(c, f)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, msg, result)
}

// writeExcel 输出 Excel 文件流（非统一 JSON 结构）。
func writeExcel(c *gin.Context, filename string, data []byte) {
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
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

// BatchCreatePoints POST /inspection/points/batch（楼栋×楼层×每层数量批量建点，幂等：同名跳过）。
func (ctl *InspectionController) BatchCreatePoints(c *gin.Context) {
	var req dto.PointBatchReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.points.BatchCreate(c, &req)
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

// PreviewPlanPoints GET /inspection/plans/preview-points?community_id=&point_types=a,b（圈选命中预览）。
func (ctl *InspectionController) PreviewPlanPoints(c *gin.Context) {
	data, be := ctl.plans.PreviewPoints(c, c.Query("community_id"), c.Query("point_types"))
	write(c, data, be)
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

// RemindTask 任务催办：给执行人发站内提醒（App 管理端一键催办）。
func (ctl *InspectionController) RemindTask(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.tasks.Remind(c, id))
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
	created, eligible, be := ctl.plans.GenerateForDate(c.Request.Context(), date)
	write(c, gin.H{"date": date.Format("2006-01-02"), "created": created, "eligible_plans": eligible}, be)
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

// ========== 问题清单（异常打卡记录只读出口） ==========

// ListIssues GET /inspection/issues（仅 result=abnormal 记录）。
func (ctl *InspectionController) ListIssues(c *gin.Context) {
	var q dto.IssueListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.tasks.IssueList(c, &q)
	write(c, page, be)
}

// ExportIssues GET /inspection/issues/export（同列表筛选，不分页，上限 5000 条，xlsx 附件）。
func (ctl *InspectionController) ExportIssues(c *gin.Context) {
	var q dto.IssueListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	rows, be := ctl.tasks.IssueExport(c, &q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	f, err := excel.ExportIssues(rows)
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	// 中文文件名：ASCII 兜底 + RFC 5987 filename*
	name := fmt.Sprintf("问题清单_%s.xlsx", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="issues_%s.xlsx"; filename*=UTF-8''%s`,
		time.Now().Format("20060102_150405"), url.PathEscape(name)))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}
