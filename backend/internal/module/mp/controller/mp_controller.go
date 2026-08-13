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
	wodto "anxuncloud/internal/module/workorder/dto"
	wosvc "anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// MPController 小程序端接口。
type MPController struct {
	mp      *service.MPService
	checkin *service.CheckinService
	upload  *service.UploadService
	orders  *wosvc.OrderService
	notices *systemsvc.NoticeService
}

func NewMPController(mp *service.MPService, checkin *service.CheckinService, upload *service.UploadService, orders *wosvc.OrderService, notices *systemsvc.NoticeService) *MPController {
	return &MPController{mp: mp, checkin: checkin, upload: upload, orders: orders, notices: notices}
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

// Local POST /upload/local（dev 模式本地上传）
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

// ========== 我的工单 ==========

// MyOrders GET /workorders/mine
func (ctl *MPController) MyOrders(c *gin.Context) {
	var q dto.MyOrdersQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.mp.MyOrders(uid(c), &q)
	write(c, page, be)
}

// OrderCounts GET /workorders/mine/counts（按状态计数，query: type）
func (ctl *MPController) OrderCounts(c *gin.Context) {
	data, be := ctl.mp.MyOrderCounts(uid(c), c.DefaultQuery("type", ""))
	write(c, data, be)
}

// OrderDetail GET /workorders/:id
func (ctl *MPController) OrderDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.orders.GetForMP(id, uid(c))
	write(c, data, be)
}

// OrderAccept POST /workorders/:id/accept
func (ctl *MPController) OrderAccept(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.orders.Accept(id, uid(c)); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"status": "processing"})
}

// OrderFinish POST /workorders/:id/finish
func (ctl *MPController) OrderFinish(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req wodto.FinishReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if len(req.FixPhotos) == 0 {
		response.Fail(c, errs.ErrParam.WithMsg("维修照片至少 1 张"))
		return
	}
	if be := ctl.orders.FinishForMP(id, uid(c), &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"status": "review"})
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
	data, be := ctl.notices.Published(page, pageSize)
	write(c, data, be)
}

// AnnouncementDetail GET /announcements/:id（仅已发布可见）
func (ctl *MPController) AnnouncementDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.notices.PublishedDetail(id)
	write(c, data, be)
}
