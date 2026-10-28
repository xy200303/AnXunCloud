package service

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/acceptance/model"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/pkg/errs"
)

func (s *BatchService) Withdraw(c *gin.Context, batchID, reason string) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if be := s.requireManager(c, batch); be != nil {
		return be
	}
	if batch.Status != model.StatusHandover {
		return errs.ErrConflict.WithMsg("只有待签署批次可以撤回")
	}
	if strings.TrimSpace(reason) == "" {
		return errs.ErrParam.WithMsg("撤回必须填写原因")
	}
	actorID := middleware.CurrentUserID(c)
	return acceptanceTransaction(s.db, func(tx *gorm.DB) error {
		if err := tx.Model(&model.Signature{}).Where("batch_id = ? AND revision = ? AND status = ?", batch.ID, batch.Revision, "valid").Updates(map[string]any{"status": "invalid", "invalid_reason": reason}).Error; err != nil {
			return err
		}
		if err := s.transitionTx(tx, batch, model.StatusInspecting, 0); err != nil {
			return err
		}
		return s.writeLog(tx, batch, "withdraw", actorID, model.StatusHandover, model.StatusInspecting, reason)
	})
}

func (s *BatchService) RequestReopen(c *gin.Context, batchID, reason string) (string, *errs.Error) {
	batch, be := s.load(c, batchID)
	if be != nil {
		return "", be
	}
	if be := s.requireManager(c, batch); be != nil {
		return "", be
	}
	if batch.Status != model.StatusDone {
		return "", errs.ErrConflict.WithMsg("只有已归档批次可以申请重开")
	}
	if strings.TrimSpace(reason) == "" {
		return "", errs.ErrParam.WithMsg("重开必须填写原因")
	}
	request := model.ReopenRequest{TenantID: batch.TenantID, BatchID: batch.ID, RequesterID: middleware.CurrentUserID(c), Reason: strings.TrimSpace(reason), Status: "pending"}
	if err := s.db.Create(&request).Error; err != nil {
		return "", errs.ErrConflict.WithMsg("该批次已有待批准的重开申请")
	}
	return request.ID, nil
}

func (s *BatchService) DecideReopen(c *gin.Context, batchID, requestID, action, reason string) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if batch.Status != model.StatusDone {
		return errs.ErrConflict.WithMsg("批次当前不在已归档状态")
	}
	var request model.ReopenRequest
	if err := s.db.First(&request, "id = ? AND batch_id = ? AND status = ?", requestID, batchID, "pending").Error; err != nil {
		return errs.ErrNotFound
	}
	actorID := middleware.CurrentUserID(c)
	if communitysvc.SameNaturalPerson(s.db, actorID, request.RequesterID) {
		return errs.ErrNoPerm.WithMsg("重开申请必须由非原申请人批准")
	}
	if action != "approve" && action != "reject" {
		return errs.ErrParam.WithMsg("重开审批动作无效")
	}
	if action == "reject" && strings.TrimSpace(reason) == "" {
		return errs.ErrParam.WithMsg("拒绝重开必须填写原因")
	}
	status := "rejected"
	if action == "approve" {
		status = "approved"
	}
	return acceptanceTransaction(s.db, func(tx *gorm.DB) error {
		now := time.Now()
		result := tx.Model(&model.ReopenRequest{}).Where("id = ? AND status = ?", request.ID, "pending").Updates(map[string]any{"status": status, "approver_id": actorID, "decision_reason": strings.TrimSpace(reason), "decided_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errs.ErrConflict.WithMsg("重开申请已被处理")
		}
		if action == "approve" {
			if err := tx.Model(&model.Signature{}).Where("batch_id = ? AND status = ?", batch.ID, "valid").Updates(map[string]any{"status": "invalid", "invalid_reason": "归档重开：" + request.Reason}).Error; err != nil {
				return err
			}
			update := tx.Model(&model.Batch{}).Where("id = ? AND status = ? AND revision = ?", batch.ID, model.StatusDone, batch.Revision).Updates(map[string]any{"status": model.StatusInspecting, "locked_at": gorm.Expr("NULL"), "review_step": 0, "revision": batch.Revision + 1})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return errs.ErrConflict.WithMsg("批次已被其他人更新")
			}
		}
		return s.writeLog(tx, batch, "reopen_"+action, actorID, model.StatusDone, batch.Status, request.Reason)
	})
}
