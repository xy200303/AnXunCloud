package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/acceptance/service"
	"anxuncloud/internal/pkg/bind"
)

func (ctl *Controller) AcceptProblem(c *gin.Context) {
	var req service.HandoverProblemReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.batches.AcceptProblem(c, c.Param("id"), c.Param("problemId"), req))
}
