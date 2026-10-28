package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/pkg/bind"
)

type reopenDecisionReq struct {
	Action string `json:"action" binding:"required"`
	Reason string `json:"reason"`
}

func (ctl *Controller) Withdraw(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.batches.Withdraw(c, c.Param("id"), req.Reason))
}

func (ctl *Controller) RequestReopen(c *gin.Context) {
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	id, be := ctl.batches.RequestReopen(c, c.Param("id"), req.Reason)
	writeAcceptance(c, gin.H{"id": id}, be)
}

func (ctl *Controller) DecideReopen(c *gin.Context) {
	var req reopenDecisionReq
	if be := bind.JSON(c, &req); be != nil {
		writeAcceptance(c, nil, be)
		return
	}
	writeAcceptance(c, nil, ctl.batches.DecideReopen(c, c.Param("id"), c.Param("requestId"), req.Action, req.Reason))
}
