package service

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/acceptance/model"
	filesvc "anxuncloud/internal/module/file/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/uploadfile"
)

type HandoverProblemReq struct {
	RiskLevel        string  `json:"risk_level" binding:"required"`
	SafetyMeasure    string  `json:"safety_measure"`
	MeasureApproved  bool    `json:"measure_approved"`
	Accepted         bool    `json:"accepted"`
	AcceptedBy       string  `json:"accepted_by"`
	AcceptanceFileID *string `json:"acceptance_file_id"`
}

func (s *BatchService) AcceptProblem(c *gin.Context, batchID, problemID string, req HandoverProblemReq) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if be := s.requireManager(c, batch); be != nil {
		return be
	}
	if batch.Status != model.StatusHandover {
		return errs.ErrConflict.WithMsg("只有待签署批次可以登记遗留接收")
	}
	if req.RiskLevel != "normal" && req.RiskLevel != "major" {
		return errs.ErrParam.WithMsg("风险等级只能是 normal 或 major")
	}
	if req.Accepted && strings.TrimSpace(req.AcceptedBy) == "" {
		return errs.ErrParam.WithMsg("遗留接收方代表必填")
	}
	if req.Accepted && req.AcceptanceFileID == nil {
		return errs.ErrParam.WithMsg("遗留接收必须附签署文件")
	}
	if req.AcceptanceFileID != nil {
		file, err := uploadfile.ByID(s.db, *req.AcceptanceFileID)
		if err != nil || file.Scene != "acceptance" || file.TenantID == nil || *file.TenantID != batch.TenantID {
			return errs.ErrParam.WithMsg("遗留接收文件不属于本租户")
		}
	}
	var problem model.Problem
	if err := s.db.First(&problem, "id = ? AND batch_id = ?", problemID, batchID).Error; err != nil {
		return errs.ErrNotFound
	}
	actorID := middleware.CurrentUserID(c)
	be = acceptanceTransaction(s.db, func(tx *gorm.DB) error {
		acceptedAt := (*time.Time)(nil)
		if req.Accepted {
			now := time.Now()
			acceptedAt = &now
		}
		if err := tx.Model(&problem).Updates(map[string]any{
			"risk_level": req.RiskLevel, "safety_measure": strings.TrimSpace(req.SafetyMeasure),
			"measure_approved": req.MeasureApproved, "accepted": req.Accepted,
			"accepted_by": strings.TrimSpace(req.AcceptedBy), "accepted_at": acceptedAt,
			"acceptance_file_id": req.AcceptanceFileID,
		}).Error; err != nil {
			return err
		}
		if req.AcceptanceFileID != nil {
			if err := filesvc.LinkObject(tx, batch.TenantID, batch.CommunityID, "acceptance_batch", batch.ID, []string{*req.AcceptanceFileID}); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Signature{}).Where("batch_id = ? AND revision = ? AND status = ?", batch.ID, batch.Revision, "valid").Updates(map[string]any{"status": "invalid", "invalid_reason": "遗留接收事实已更新"}).Error; err != nil {
			return err
		}
		result := tx.Model(&model.Batch{}).Where("id = ? AND status = ? AND revision = ?", batch.ID, model.StatusHandover, batch.Revision).Update("revision", batch.Revision+1)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errs.ErrConflict.WithMsg("批次已被其他人更新")
		}
		return s.writeLog(tx, batch, "handover_accept", actorID, batch.Status, batch.Status, problem.Fact)
	})
	if be != nil {
		return be
	}
	if err := s.enqueueRender(c.Request.Context(), batch.ID, "frozen"); err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *BatchService) validateHandoverProblems(batch *model.Batch) *errs.Error {
	var problems []model.Problem
	if err := s.db.Where("batch_id = ?", batch.ID).Find(&problems).Error; err != nil {
		return errs.ErrInternal
	}
	return validateHandoverProblemRows(problems)
}

func validateHandoverProblemRows(problems []model.Problem) *errs.Error {
	for _, problem := range problems {
		if strings.TrimSpace(problem.SafetyMeasure) == "" {
			return errs.ErrConflict.WithMsg("遗留问题「" + problem.Fact + "」尚未填写风险或安全措施")
		}
		if problem.RiskLevel == "major" && !problem.MeasureApproved {
			return errs.ErrConflict.WithMsg("重大安全风险「" + problem.Fact + "」的处置措施尚未获批")
		}
		if !problem.Accepted || problem.AcceptanceFileID == nil || strings.TrimSpace(problem.AcceptedBy) == "" {
			return errs.ErrConflict.WithMsg("遗留问题「" + problem.Fact + "」尚未取得接收方签署接受文件")
		}
	}
	return nil
}
