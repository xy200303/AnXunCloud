package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/mp/dto"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
)

// PushController App 推送设备绑定（uniPush 2.0；mp 与 app 两组共用同一 handler）。
type PushController struct {
	notifier *notify.Notifier
}

func NewPushController(notifier *notify.Notifier) *PushController {
	return &PushController{notifier: notifier}
}

// BindDevice POST /push/device：绑定/改绑当前用户的个推 cid（登录即可）。
func (ctl *PushController) BindDevice(c *gin.Context) {
	var req dto.PushDeviceBindReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if err := ctl.notifier.BindDevice(uid(c), req.CID, req.Platform); err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	response.OK(c, nil)
}

// UnbindDevice DELETE /push/device：解绑当前用户的 cid（退出登录时调用；body 或 query 带 cid）。
func (ctl *PushController) UnbindDevice(c *gin.Context) {
	var req dto.PushDeviceUnbindReq
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if be := bind.JSON(c, &req); be != nil {
			response.Fail(c, be)
			return
		}
	} else if be := bind.Query(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if err := ctl.notifier.UnbindDevice(uid(c), req.CID); err != nil {
		response.Fail(c, errs.ErrInternal)
		return
	}
	response.OK(c, nil)
}
