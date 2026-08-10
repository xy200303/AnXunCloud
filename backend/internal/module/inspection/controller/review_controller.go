package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/response"
)

// ReviewController 打卡记录审核与抽查接口。
type ReviewController struct {
	review *service.ReviewService
}

func NewReviewController(review *service.ReviewService) *ReviewController {
	return &ReviewController{review: review}
}

func (ctl *ReviewController) Records(c *gin.Context) {
	var q dto.ReviewListQuery
	if be := bind.Query(c, &q); be != nil {
		response.Fail(c, be)
		return
	}
	page, be := ctl.review.List(c, &q)
	write(c, page, be)
}

func (ctl *ReviewController) Pass(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.review.Pass(c, id))
}

func (ctl *ReviewController) Reject(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req dto.ReviewRejectReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.review.Reject(c, id, req.Reason))
}

func (ctl *ReviewController) BatchPass(c *gin.Context) {
	var req dto.BatchPassReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	result, be := ctl.review.BatchPass(c, req.IDs)
	write(c, result, be)
}

func (ctl *ReviewController) Reopen(c *gin.Context) {
	id, be := pathID(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	write(c, nil, ctl.review.Reopen(c, id))
}

func (ctl *ReviewController) Spotcheck(c *gin.Context) {
	var req dto.SpotcheckReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	result, be := ctl.review.Spotcheck(c, &req)
	write(c, result, be)
}
