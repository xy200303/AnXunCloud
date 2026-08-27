// Package controller 小程序端接口 HTTP 层。
package controller

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/mp/dto"
	"anxuncloud/internal/module/mp/service"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// MPController 小程序端接口。
type MPController struct {
	mp      *service.MPService
	checkin *service.CheckinService
	upload  *service.UploadService
	notices *systemsvc.NoticeService
}

func NewMPController(mp *service.MPService, checkin *service.CheckinService, upload *service.UploadService, notices *systemsvc.NoticeService) *MPController {
	return &MPController{mp: mp, checkin: checkin, upload: upload, notices: notices}
}

func uid(c *gin.Context) string { return middleware.CurrentUserID(c) }

// pathID 解析路径参数 {id}（UUID 字符串；"0" 保留用于消息全部已读约定）。
func pathID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("id")
	if id == "" {
		return "", errs.ErrParam.WithMsg("id 不能为空")
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

// ========== 登录 ==========

// Login POST /login
func (ctl *MPController) Login(c *gin.Context) {
	var req dto.MPLoginReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.mp.Login(c.Request.Context(), &req, c.ClientIP(), c.GetHeader("User-Agent"))
	write(c, data, be)
}

// Refresh POST /refresh
func (ctl *MPController) Refresh(c *gin.Context) {
	var req dto.MPRefreshReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.mp.Refresh(c.Request.Context(), req.RefreshToken)
	write(c, data, be)
}

// ========== 任务 ==========

// TodayTasks GET /tasks/today
func (ctl *MPController) TodayTasks(c *gin.Context) {
	data, be := ctl.mp.TodayTasks(uid(c))
	write(c, data, be)
}

// PointByCode GET /points/by-code/:code（扫码/NFC 定位任务）
func (ctl *MPController) PointByCode(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, errs.ErrParam.WithMsg("code 不能为空"))
		return
	}
	data, be := ctl.mp.PointByCode(uid(c), code)
	write(c, data, be)
}

// Points GET /points?community_id=（问题上报关联点位的候选列表）
func (ctl *MPController) Points(c *gin.Context) {
	data, be := ctl.mp.Points(c, uid(c), c.Query("community_id"))
	write(c, data, be)
}

// PublicPoint GET /api/public/point/:code（短链接 H5 点位信息页，免登录脱敏摘要）
func (ctl *MPController) PublicPoint(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.Fail(c, errs.ErrParam.WithMsg("code 不能为空"))
		return
	}
	data, be := ctl.mp.PublicPoint(code)
	write(c, data, be)
}

// TaskDetail GET /tasks/:id
func (ctl *MPController) TaskDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.mp.TaskDetail(uid(c), id)
	if be == nil {
		// AI 能力透出（配置归 checkin 服务管）：逐项识别是否可用、已提交点位是否允许覆盖修改
		data["ai_enabled"] = ctl.checkin.AIEnabled()
		data["ai_result_editable"] = ctl.checkin.AIResultEditable()
	}
	write(c, data, be)
}

// ========== 打卡 ==========

// Checkin POST /checkin
func (ctl *MPController) Checkin(c *gin.Context) {
	var req dto.CheckinReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.checkin.Submit(c, uid(c), &req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OKMsg(c, "打卡成功", data)
}

// OfflineSync POST /checkin/offline-sync
func (ctl *MPController) OfflineSync(c *gin.Context) {
	var req dto.OfflineSyncReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.checkin.OfflineSync(c, uid(c), &req)
	write(c, data, be)
}

// CheckinItems GET /checkins/:id/items（本人打卡记录的逐项 AI 结论，提交后回显用）
func (ctl *MPController) CheckinItems(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.mp.CheckinItems(uid(c), id)
	write(c, data, be)
}

// SubmitAIItemJob POST /checkin/ai-item-jobs（逐项 AI 识别：提交单个检查项照片，异步识别）
func (ctl *MPController) SubmitAIItemJob(c *gin.Context) {
	var req dto.AIItemJobReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.checkin.SubmitAIItemJob(c.Request.Context(), uid(c), &req)
	write(c, data, be)
}

// AIItemJobs GET /checkin/ai-item-jobs?ids=a,b,c（批量轮询逐项识别结果，≤20 个）
func (ctl *MPController) AIItemJobs(c *gin.Context) {
	data, be := ctl.checkin.AIItemJobs(c.Request.Context(), uid(c), c.Query("ids"))
	write(c, data, be)
}

// ItemDrafts GET /checkin/item-drafts?task_id[&point_id]（逐项识别/手动项过程草稿，断点恢复用）
func (ctl *MPController) ItemDrafts(c *gin.Context) {
	data, be := ctl.checkin.ItemDrafts(c.Request.Context(), uid(c), c.Query("task_id"), c.Query("point_id"))
	write(c, data, be)
}

// SaveManualDraft POST /checkin/item-drafts/manual（手动确认项选择落云端草稿）
func (ctl *MPController) SaveManualDraft(c *gin.Context) {
	var req dto.ManualItemDraftReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.checkin.SaveManualDraft(c.Request.Context(), uid(c), &req)
	write(c, data, be)
}

// ========== 上传 ==========

// STS POST /upload/sts
func (ctl *MPController) STS(c *gin.Context) {
	var req dto.STSReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.upload.STS(uid(c), &req)
	write(c, data, be)
}

// Local POST /upload/local（local 模式上传）
func (ctl *MPController) Local(c *gin.Context) {
	scene := c.PostForm("scene")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.ErrParam.WithMsg("缺少上传文件 file"))
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	defer f.Close()
	data, be := ctl.upload.SaveLocal(uid(c), scene, fileHeader.Filename, fileHeader.Size, f)
	write(c, data, be)
}

// Callback POST /upload/callback（OSS 服务端间回调，无 JWT）
func (ctl *MPController) Callback(c *gin.Context) {
	body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	status, resp := ctl.upload.Callback(c, body)
	c.JSON(status, resp)
}

// ========== 消息与公告 ==========

// Messages GET /messages
func (ctl *MPController) Messages(c *gin.Context) {
	var q dto.MessageQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.mp.Messages(uid(c), &q)
	write(c, data, be)
}

// MarkRead PUT /messages/:id/read（id=0 全部已读）
func (ctl *MPController) MarkRead(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.mp.MarkRead(uid(c), id))
}

// Announcements GET /announcements
func (ctl *MPController) Announcements(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	data, be := ctl.notices.Published(page, pageSize, middleware.CurrentIdentity(c).TenantID)
	write(c, data, be)
}

// AnnouncementDetail GET /announcements/:id（仅已发布可见）
func (ctl *MPController) AnnouncementDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.notices.PublishedDetail(id, middleware.CurrentIdentity(c).TenantID)
	write(c, data, be)
}
