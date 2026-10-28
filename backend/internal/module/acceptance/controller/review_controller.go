package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/module/acceptance/service"
	"anxuncloud/internal/pkg/bind"
)

func (ctl *Controller) Review(c *gin.Context) {
	var req service.ReviewReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.batches.Review(c, c.Param("id"), req))
}

func (ctl *Controller) Sign(c *gin.Context) {
	var req service.SignReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.batches.Sign(c, c.Param("id"), req))
}

func (ctl *Controller) UpdateSubjects(c *gin.Context) {
	var req struct {
		Subjects []model.SignSubject `json:"subjects" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.batches.UpdateSubjects(c, c.Param("id"), req.Subjects))
}

func (ctl *Controller) Download(c *gin.Context) {
	data, filename, be := ctl.batches.Download(c, c.Param("id"), c.Param("fileId"))
	if be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	mimeType := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	if strings.HasSuffix(filename, ".pdf") {
		mimeType = "application/pdf"
	}
	c.Data(http.StatusOK, mimeType, data)
}
