package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// MessageController 管理端站内消息接口（顶栏铃铛，登录即可）。
type MessageController struct {
	svc *service.MessageService
}

func NewMessageController(svc *service.MessageService) *MessageController {
	return &MessageController{svc: svc}
}

// List GET /system/messages
func (ctl *MessageController) List(c *gin.Context) {
	var q service.MessageListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.List(middleware.CurrentUserID(c), &q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// MarkRead PUT /system/messages/:id/read（id=0 全部已读；仅本人消息）
func (ctl *MessageController) MarkRead(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Fail(c, errs.ErrParam.WithMsg("id 不能为空"))
		return
	}
	if be := ctl.svc.MarkRead(middleware.CurrentUserID(c), id); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
