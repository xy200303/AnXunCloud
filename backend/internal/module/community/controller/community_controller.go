// Package controller 小区与楼栋接口 HTTP 层。
package controller

import (

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"property-inspection/internal/module/community/dto"
	"property-inspection/internal/module/community/service"
	"property-inspection/internal/pkg/bind"
	"property-inspection/internal/pkg/errs"
	"property-inspection/internal/pkg/response"
)

// CommunityController 小区/楼栋接口。
type CommunityController struct {
	svc *service.CommunityService
}

func NewCommunityController(svc *service.CommunityService) *CommunityController {
	return &CommunityController{svc: svc}
}

func pathID(c *gin.Context) (string, *errs.Error) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return "", errs.ErrParam.WithMsg("id 须为 UUID")
	}
	return id, nil
}

func (ctl *CommunityController) List(c *gin.Context) {
	var q dto.CommunityListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.ListCommunities(c, &q)
	write(c, page, be)
}

func (ctl *CommunityController) Create(c *gin.Context) {
	var req dto.CommunitySaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.CreateCommunity(&req)
	write(c, gin.H{"id": id}, be)
}

func (ctl *CommunityController) Detail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.CommunityDetail(c, id)
	write(c, data, be)
}

func (ctl *CommunityController) Update(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.CommunitySaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.UpdateCommunity(c, id, &req))
}

func (ctl *CommunityController) Delete(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.DeleteCommunity(c, id))
}

func (ctl *CommunityController) ListBuildings(c *gin.Context) {
	var q dto.BuildingListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.svc.ListBuildings(c, &q)
	write(c, page, be)
}

func (ctl *CommunityController) CreateBuilding(c *gin.Context) {
	var req dto.BuildingSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	id, be := ctl.svc.CreateBuilding(c, &req)
	write(c, gin.H{"id": id}, be)
}

func (ctl *CommunityController) BuildingDetail(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.BuildingDetail(c, id)
	write(c, data, be)
}

func (ctl *CommunityController) UpdateBuilding(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.BuildingSaveReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.UpdateBuilding(c, id, &req))
}

func (ctl *CommunityController) DeleteBuilding(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.svc.DeleteBuilding(c, id))
}

func write(c *gin.Context, data any, be *errs.Error) {
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}
