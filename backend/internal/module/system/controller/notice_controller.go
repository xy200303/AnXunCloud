package controller

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// NoticeController 通知公告接口。db 用于解析租户上下文（middleware.EffectiveTenantID）。
type NoticeController struct {
	svc *service.NoticeService
	db  *gorm.DB
}

func NewNoticeController(svc *service.NoticeService, db *gorm.DB) *NoticeController {
	return &NoticeController{svc: svc, db: db}
}

// List GET /system/notices
func (ctl *NoticeController) List(c *gin.Context) {
	var q service.NoticeListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.List(&q, tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// Create POST /system/notices
func (ctl *NoticeController) Create(c *gin.Context) {
	var req service.NoticeSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	identity := middleware.CurrentIdentity(c)
	id, be := ctl.svc.Create(&req, identity.UserID, identity.Name, tid)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// Update PUT /system/notices/:id
func (ctl *NoticeController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req service.NoticeSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Update(id, &req, tid); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// Delete DELETE /system/notices/:id
func (ctl *NoticeController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	tid, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Delete(id, tid); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
