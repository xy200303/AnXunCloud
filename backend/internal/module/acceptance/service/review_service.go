package service

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/acceptance/model"
	communitysvc "anxuncloud/internal/module/community/service"
	filesvc "anxuncloud/internal/module/file/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/uploadfile"
)

type ReviewReq struct {
	Action string `json:"action" binding:"required"`
	Reason string `json:"reason"`
}

func (s *BatchService) Review(c *gin.Context, batchID string, req ReviewReq) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if batch.Status != model.StatusReviewing {
		return errs.ErrConflict.WithMsg("批次当前不在审核阶段")
	}
	if batch.ReviewStep < 0 || batch.ReviewStep >= len(batch.ReviewFlow) {
		return errs.ErrConflict.WithMsg("审核节点无效，批次已挂起")
	}
	step := batch.ReviewFlow[batch.ReviewStep]
	identity := middleware.CurrentIdentity(c)
	if !communitysvc.RoleCandidateAuthorized(s.db, []string{batch.CommunityID}, step.RoleCode, identity) {
		return errs.ErrNotInSlot
	}
	actorID := middleware.CurrentUserID(c)
	if communitysvc.SameNaturalPerson(s.db, actorID, batch.ManagerID) {
		return errs.ErrNoPerm.WithMsg("不能审核本人提交的批次")
	}
	var contributorIDs []string
	s.db.Model(&model.Record{}).Where("batch_id = ?", batch.ID).Distinct().Pluck("created_by", &contributorIDs)
	for _, contributor := range contributorIDs {
		if communitysvc.SameNaturalPerson(s.db, actorID, contributor) {
			return errs.ErrNoPerm.WithMsg("不能审核本人查验的问题事实")
		}
	}
	if req.Action != "approve" && req.Action != "reject" {
		return errs.ErrParam.WithMsg("审核动作无效")
	}
	if req.Action == "reject" && strings.TrimSpace(req.Reason) == "" {
		return errs.ErrParam.WithMsg("驳回必须填写原因")
	}
	be = acceptanceTransaction(s.db, func(tx *gorm.DB) error {
		vote := model.ReviewVote{TenantID: batch.TenantID, BatchID: batch.ID, StepIndex: batch.ReviewStep, UserID: actorID, Action: req.Action, Reason: strings.TrimSpace(req.Reason)}
		if err := tx.Create(&vote).Error; err != nil {
			return errs.ErrConflict.WithMsg("当前节点已审核或已被其他人更新")
		}
		if err := s.writeLog(tx, batch, "review_"+req.Action, actorID, batch.Status, batch.Status, req.Reason); err != nil {
			return err
		}
		if req.Action == "reject" {
			return s.transitionTx(tx, batch, model.StatusInspecting, batch.ReviewStep)
		}
		if step.Mode == "all" {
			candidates := communitysvc.RoleCandidateUserIDsForCommunity(tx, batch.CommunityID, step.RoleCode)
			var votes int64
			if err := tx.Model(&model.ReviewVote{}).Where("batch_id = ? AND step_index = ? AND action = ?", batch.ID, batch.ReviewStep, "approve").Count(&votes).Error; err != nil {
				return err
			}
			if int(votes) < len(candidates) {
				return nil
			}
		}
		next := batch.ReviewStep + 1
		if next < len(batch.ReviewFlow) {
			return s.transitionTx(tx, batch, model.StatusReviewing, next)
		}
		return s.transitionTx(tx, batch, model.StatusHandover, next)
	})
	if be != nil {
		return be
	}
	if req.Action == "approve" && batch.ReviewStep+1 >= len(batch.ReviewFlow) {
		if err := s.enqueueRender(c.Request.Context(), batch.ID, "frozen"); err != nil {
			return errs.ErrInternal
		}
	}
	return nil
}

func (s *BatchService) transitionTx(tx *gorm.DB, batch *model.Batch, status string, reviewStep int) error {
	result := tx.Model(&model.Batch{}).Where("id = ? AND status = ? AND revision = ?", batch.ID, batch.Status, batch.Revision).Updates(map[string]any{"status": status, "revision": batch.Revision + 1, "review_step": reviewStep})
	if result.Error != nil {
		return errs.ErrInternal
	}
	if result.RowsAffected != 1 {
		return errs.ErrConflict.WithMsg("批次已被其他人更新")
	}
	return nil
}

func (s *BatchService) writeLog(tx *gorm.DB, batch *model.Batch, action, actorID, from, to, reason string) error {
	return tx.Create(&model.HandoverLog{TenantID: batch.TenantID, BatchID: batch.ID, Action: action, ActorID: actorID, FromStatus: from, ToStatus: to, Reason: reason}).Error
}

func (s *BatchService) createReportFile(tx *gorm.DB, batch *model.Batch, stage string, signatures []model.Signature) (*model.ReportFile, error) {
	var targets []model.Target
	var records []model.Record
	if err := tx.Where("batch_id = ?", batch.ID).Order("created_at, id").Find(&targets).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("batch_id = ?", batch.ID).Order("updated_at, id").Find(&records).Error; err != nil {
		return nil, err
	}
	var problems []model.Problem
	if err := tx.Where("batch_id = ?", batch.ID).Find(&problems).Error; err != nil {
		return nil, err
	}
	problemByRecord := map[string]model.Problem{}
	for _, problem := range problems {
		problemByRecord[problem.RecordID] = problem
	}
	for i := range records {
		if problem, ok := problemByRecord[records[i].ID]; ok && problem.Accepted {
			records[i].Note += "；遗留接收：" + problem.AcceptedBy + "；风险措施：" + problem.SafetyMeasure
		}
	}
	return s.createMotherReportFiles(tx, batch, targets, records, signatures)
}

type SignReq struct {
	SubjectCode    string `json:"subject_code" binding:"required"`
	Representative string `json:"representative"`
	FileID         string `json:"file_id" binding:"required"`
}

func (s *BatchService) Sign(c *gin.Context, batchID string, req SignReq) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if batch.Status != model.StatusHandover {
		return errs.ErrConflict.WithMsg("批次当前不在待签署阶段")
	}
	var subject model.SignSubject
	if err := s.db.First(&subject, "batch_id = ? AND code = ?", batchID, req.SubjectCode).Error; err != nil {
		return errs.ErrNotFound
	}
	actorID := middleware.CurrentUserID(c)
	if subject.External {
		if be := s.requireManager(c, batch); be != nil {
			return be
		}
		if strings.TrimSpace(req.Representative) == "" {
			return errs.ErrParam.WithMsg("外部签署方代表人必填")
		}
	} else {
		if subject.RoleCode == "" {
			if _, be := s.memberFor(c, batch, ""); be != nil {
				return be
			}
		} else if !communitysvc.RoleCandidateAuthorized(s.db, []string{batch.CommunityID}, subject.RoleCode, middleware.CurrentIdentity(c)) {
			return errs.ErrNotInSlot
		}
		req.Representative = middleware.CurrentIdentity(c).Name
	}
	var frozenFiles []model.ReportFile
	if err := s.db.Where("batch_id = ? AND revision = ? AND stage = ? AND status = ?", batchID, batch.Revision, "frozen", "ready").Find(&frozenFiles).Error; err != nil || len(frozenFiles) == 0 {
		return errs.ErrConflict.WithMsg("待签报告尚未生成，不能签署")
	}
	hasDOCX, hasPDF := false, false
	for _, file := range frozenFiles {
		hasDOCX = hasDOCX || file.Format == "docx" && file.FileID != nil
		hasPDF = hasPDF || file.Format == "pdf" && file.FileID != nil
	}
	if !hasDOCX || !hasPDF {
		return errs.ErrConflict.WithMsg("待签 DOCX/PDF 尚未全部生成，不能签署")
	}
	frozenSHA := acceptanceFilesDigest(frozenFiles)
	file, err := uploadfile.ByID(s.db, req.FileID)
	if err != nil || file.Scene != "acceptance" || file.UserID != actorID || file.TenantID == nil || *file.TenantID != batch.TenantID {
		return errs.ErrParam.WithMsg("签署证据不属于当前用户或租户")
	}
	allSigned := false
	be = acceptanceTransaction(s.db, func(tx *gorm.DB) error {
		signature := model.Signature{TenantID: batch.TenantID, BatchID: batch.ID, Revision: batch.Revision, SubjectCode: subject.Code, Representative: strings.TrimSpace(req.Representative), UserID: actorID, FileID: req.FileID, ReportSHA256: frozenSHA, Status: "valid", SignedAt: time.Now()}
		if err := tx.Create(&signature).Error; err != nil {
			return errs.ErrConflict.WithMsg("该签署位已签署或报告已更新")
		}
		if err := filesvc.LinkObject(tx, batch.TenantID, batch.CommunityID, "acceptance_batch", batch.ID, []string{req.FileID}); err != nil {
			return err
		}
		if err := s.writeLog(tx, batch, "sign", actorID, batch.Status, batch.Status, subject.Name); err != nil {
			return err
		}
		var missing int64
		if err := tx.Model(&model.SignSubject{}).Where("batch_id = ? AND required = true AND code NOT IN (?)", batchID, tx.Model(&model.Signature{}).Select("subject_code").Where("batch_id = ? AND revision = ? AND status = ? AND report_sha256 = ?", batchID, batch.Revision, "valid", frozenSHA)).Count(&missing).Error; err != nil {
			return err
		}
		allSigned = missing == 0
		return nil
	})
	if be != nil {
		return be
	}
	if allSigned {
		if be := s.validateHandoverProblems(batch); be != nil {
			return be
		}
		if err := s.enqueueRender(c.Request.Context(), batch.ID, "archived"); err != nil {
			return errs.ErrInternal
		}
	}
	return nil
}

func (s *BatchService) Download(c *gin.Context, batchID, reportFileID string) ([]byte, string, *errs.Error) {
	if _, be := s.load(c, batchID); be != nil {
		return nil, "", be
	}
	var reportFile model.ReportFile
	if err := s.db.First(&reportFile, "id = ? AND batch_id = ? AND status = ?", reportFileID, batchID, "ready").Error; err != nil || reportFile.FileID == nil {
		return nil, "", errs.ErrNotFound
	}
	file, err := uploadfile.ByID(s.db, *reportFile.FileID)
	if err != nil || file.TenantID == nil || *file.TenantID != reportFile.TenantID || file.Scene != "acceptance" {
		return nil, "", errs.ErrDataScope
	}
	data, err := s.store.ReadFile(file.StorageKey)
	if err != nil {
		return nil, "", errs.ErrInternal
	}
	return data, reportFile.VolumeName + "-" + reportFile.Stage + "." + reportFile.Format, nil
}

func (s *BatchService) UpdateSubjects(c *gin.Context, batchID string, subjects []model.SignSubject) *errs.Error {
	batch, be := s.load(c, batchID)
	if be != nil {
		return be
	}
	if be := s.requireManager(c, batch); be != nil {
		return be
	}
	if batch.Status != model.StatusDraft {
		return errs.ErrConflict.WithMsg("只有草稿批次可以调整签署位")
	}
	if len(subjects) == 0 || len(subjects) > 10 {
		return errs.ErrParam.WithMsg("签署位须为 1-10 个")
	}
	seen := map[string]bool{}
	for i := range subjects {
		subjects[i].Code = strings.TrimSpace(subjects[i].Code)
		subjects[i].Name = strings.TrimSpace(subjects[i].Name)
		if !stableCode.MatchString(subjects[i].Code) || subjects[i].Name == "" || seen[subjects[i].Code] {
			return errs.ErrParam.WithMsg("签署位编码或名称无效")
		}
		seen[subjects[i].Code] = true
		subjects[i].TenantID, subjects[i].BatchID, subjects[i].Sort = batch.TenantID, batch.ID, i+1
		subjects[i].ID = ""
	}
	return acceptanceTransaction(s.db, func(tx *gorm.DB) error {
		if err := tx.Where("batch_id = ?", batch.ID).Delete(&model.SignSubject{}).Error; err != nil {
			return err
		}
		for _, subject := range subjects {
			if err := tx.Create(&subject).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func acceptanceTransaction(db *gorm.DB, fn func(*gorm.DB) error) *errs.Error {
	err := db.Transaction(fn)
	if err == nil {
		return nil
	}
	if be, ok := err.(*errs.Error); ok {
		return be
	}
	logger.L.Error("承接查验事务执行失败", zap.Error(err))
	return errs.ErrInternal
}
