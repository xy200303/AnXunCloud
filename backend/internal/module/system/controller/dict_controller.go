package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// DictController 字典管理接口。
type DictController struct {
	svc *service.DictService
}

func NewDictController(svc *service.DictService) *DictController {
	return &DictController{svc: svc}
}

// ListTypes GET /system/dict-types
func (ctl *DictController) ListTypes(c *gin.Context) {
	var q dto.DictTypeQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.ListTypes(&q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// CreateType POST /system/dict-types
func (ctl *DictController) CreateType(c *gin.Context) {
	var req dto.DictTypeSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.CreateType(&req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// UpdateType PUT /system/dict-types/:id
func (ctl *DictController) UpdateType(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.DictTypeSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.UpdateType(id, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// DeleteType DELETE /system/dict-types/:id
func (ctl *DictController) DeleteType(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.DeleteType(id); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// ListData GET /system/dict-data
func (ctl *DictController) ListData(c *gin.Context) {
	var q dto.DictDataQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.ListData(&q)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, page)
}

// CreateData POST /system/dict-data
func (ctl *DictController) CreateData(c *gin.Context) {
	var req dto.DictDataSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.CreateData(&req)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, gin.H{"id": id})
}

// UpdateData PUT /system/dict-data/:id
func (ctl *DictController) UpdateData(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.DictDataSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.UpdateData(id, &req); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// DeleteData DELETE /system/dict-data/:id
func (ctl *DictController) DeleteData(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.DeleteData(id); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}
