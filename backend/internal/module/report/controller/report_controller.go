// Package controller 月度报告接口 HTTP 层（管理后台）。
package controller

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"anxuncloud/internal/module/report/dto"
	"anxuncloud/internal/module/report/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// ReportController 月度报告接口。
type ReportController struct {
	svc *service.ReportService
}

func NewReportController(svc *service.ReportService) *ReportController {
	return &ReportController{svc: svc}
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

// List GET /reports
func (ctl *ReportController) List(c *gin.Context) {
	var q dto.ReportListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.List(c, &q)
	write(c, page, be)
}

// Detail GET /reports/:id
func (ctl *ReportController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Detail(c, id)
	write(c, data, be)
}

// PagedRecords GET /reports/:id/records 打卡明细分页
func (ctl *ReportController) PagedRecords(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var q dto.ReportRecordsQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.PagedRecords(c, id, &q)
	write(c, page, be)
}

// Generate POST /reports/generate
func (ctl *ReportController) Generate(c *gin.Context) {
	var req dto.GenerateReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Generate(c, &req)
	write(c, data, be)
}

// Rebuild POST /reports/:id/rebuild（模板升级后用当前模板重渲染 PDF 并覆盖归档，状态/签字留痕不变）
func (ctl *ReportController) Rebuild(c *gin.Context) {
	if be := ctl.svc.RebuildPDF(c, c.Param("id")); be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, "已按当前模板重新生成", nil)
}

// SignCandidates GET /reports/sign-candidates?community_id=[&patrol_type=]（生成报告时的可选签字人；
// patrol_type 非空时主管级默认名单按该类型汇报线槽位取）
func (ctl *ReportController) SignCandidates(c *gin.Context) {
	communityID := c.Query("community_id")
	if _, err := uuid.Parse(communityID); err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("community_id 须为 UUID"))
		return
	}
	data, be := ctl.svc.SignCandidates(c, communityID, c.Query("patrol_type"))
	write(c, data, be)
}

// SignInspector POST /reports/:id/sign-inspector（电子确认；body 带 proxy_for+reason 为代签）
func (ctl *ReportController) SignInspector(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.InspectorSignReq
	_ = c.ShouldBindJSON(&req) // 本人确认为空 body，容错解析
	data, be := ctl.svc.SignInspector(c, id, &req)
	write(c, data, be)
}

// SignSupervisor POST /reports/:id/sign-supervisor
func (ctl *ReportController) SignSupervisor(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.SignReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.SignSupervisor(c, id, &req)
	write(c, data, be)
}

// SignManager POST /reports/:id/sign-manager
func (ctl *ReportController) SignManager(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.SignReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.SignManager(c, id, &req)
	write(c, data, be)
}

// PDF GET /reports/:id/pdf（已归档走 file_key，未终审即时生成临时版）
func (ctl *ReportController) PDF(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, filename, be := ctl.svc.PDF(c, id)
	if be != nil {
		response.Fail(c, be)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename)))
	c.Data(http.StatusOK, "application/pdf", data)
}

// PDFTicket POST /reports/:id/pdf-ticket（App web-view 预览用：签发限时 ticket，判定同 PDF 接口）
func (ctl *ReportController) PDFTicket(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	ticket, be := ctl.svc.PDFTicket(c, id)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"ticket": ticket})
}

// PDFByTicket GET /api/public/report-pdf/:id?ticket=（免登录，仅凭 ticket；inline 供 web-view 内渲染）
func (ctl *ReportController) PDFByTicket(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, filename, be := ctl.svc.PDFByTicket(c, id, c.Query("ticket"))
	if be != nil {
		response.Fail(c, be)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", url.PathEscape(filename)))
	c.Data(http.StatusOK, "application/pdf", data)
}
