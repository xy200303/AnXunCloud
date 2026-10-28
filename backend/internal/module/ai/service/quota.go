package service

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
	"anxuncloud/internal/pkg/errs"
)

type UsageService struct{ db *gorm.DB }

func NewUsageService(db *gorm.DB) *UsageService { return &UsageService{db: db} }

type QuotaReq struct {
	MaxRequestsDaily int   `json:"max_requests_daily" binding:"required"`
	MaxTokensDaily   int64 `json:"max_tokens_daily" binding:"required"`
	MaxConcurrent    int   `json:"max_concurrent" binding:"required"`
}

func defaultQuota(tenantID, taskType string) model.Quota {
	return model.Quota{TenantID: tenantID, TaskType: taskType, MaxRequestsDaily: 1000, MaxTokensDaily: 1000000, MaxConcurrent: 2}
}

func (s *UsageService) quota(tenantID, taskType string) (model.Quota, error) {
	var quota model.Quota
	if err := s.db.Where("tenant_id = ? AND task_type = ?", tenantID, taskType).First(&quota).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultQuota(tenantID, taskType), nil
		}
		return model.Quota{}, err
	}
	return quota, nil
}

func (s *UsageService) Reserve(actor Actor, taskType string) (*model.UsageReservation, *errs.Error) {
	var reservation model.UsageReservation
	err := s.db.Transaction(func(tx *gorm.DB) error {
		quota, err := s.quota(actor.TenantID, taskType)
		if err != nil {
			return err
		}
		today := time.Now().Truncate(24 * time.Hour)
		var requests int64
		if err := tx.Model(&model.UsageReservation{}).
			Where("tenant_id = ? AND task_type = ? AND started_at >= ?", actor.TenantID, taskType, today).Count(&requests).Error; err != nil {
			return err
		}
		if requests >= int64(quota.MaxRequestsDaily) {
			return errs.ErrConflict.WithMsg("该 AI 任务的日请求配额已耗尽")
		}
		var tokens int64
		if err := tx.Model(&model.UsageReservation{}).
			Where("tenant_id = ? AND task_type = ? AND started_at >= ?", actor.TenantID, taskType, today).
			Select("COALESCE(SUM(prompt_tokens + completion_tokens), 0)").Scan(&tokens).Error; err != nil {
			return err
		}
		if tokens >= quota.MaxTokensDaily {
			return errs.ErrConflict.WithMsg("该 AI 任务的日 token 配额已耗尽")
		}
		var running int64
		if err := tx.Model(&model.UsageReservation{}).Where("tenant_id = ? AND task_type = ? AND status = ? AND started_at > ?", actor.TenantID, taskType, "running", time.Now().Add(-5*time.Minute)).Count(&running).Error; err != nil {
			return err
		}
		if running >= int64(quota.MaxConcurrent) {
			return errs.ErrConflict.WithMsg("该 AI 任务的并发配额已占满，请稍后重试")
		}
		reservation = model.UsageReservation{TenantID: actor.TenantID, TaskType: taskType, Status: "running", StartedAt: time.Now()}
		return tx.Create(&reservation).Error
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return nil, be
		}
		return nil, errs.ErrInternal
	}
	return &reservation, nil
}

func (s *UsageService) Finish(reservation *model.UsageReservation, promptTokens, completionTokens int, failed bool) {
	status := "done"
	if failed {
		status = "failed"
	}
	now := time.Now()
	s.db.Model(reservation).Updates(map[string]any{"status": status, "finished_at": &now, "prompt_tokens": promptTokens, "completion_tokens": completionTokens})
}

func (s *UsageService) Usage(actor Actor) (gin.H, *errs.Error) {
	quota, err := s.quota(actor.TenantID, "assistant")
	if err != nil {
		return nil, errs.ErrInternal
	}
	today := time.Now().Truncate(24 * time.Hour)
	var usage struct {
		Requests int64
		Tokens   int64
	}
	if err := s.db.Model(&model.UsageReservation{}).
		Where("tenant_id = ? AND task_type = ? AND started_at >= ?", actor.TenantID, "assistant", today).
		Select("COUNT(*) AS requests, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens").Scan(&usage).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var running int64
	if err := s.db.Model(&model.UsageReservation{}).Where("tenant_id = ? AND task_type = ? AND status = ? AND started_at > ?", actor.TenantID, "assistant", "running", time.Now().Add(-5*time.Minute)).Count(&running).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var configured int64
	s.db.Model(&model.Quota{}).Where("tenant_id = ? AND task_type = ?", actor.TenantID, "assistant").Count(&configured)
	source := "default"
	if configured > 0 {
		source = "tenant"
	}
	return gin.H{"task_type": "assistant", "date": today.Format("2006-01-02"), "requests": usage.Requests, "tokens": usage.Tokens, "running": running, "quota": quota, "source": source}, nil
}

func (s *UsageService) SaveQuota(actor Actor, taskType string, req QuotaReq) *errs.Error {
	if taskType != "assistant" || req.MaxRequestsDaily <= 0 || req.MaxTokensDaily <= 0 || req.MaxConcurrent <= 0 {
		return errs.ErrParam.WithMsg("当前只支持助手任务的配额，且数值必须大于 0")
	}
	var count int64
	if err := s.db.Model(&model.Quota{}).Where("tenant_id = ? AND task_type = ?", actor.TenantID, taskType).Count(&count).Error; err != nil {
		return errs.ErrInternal
	}
	quota := model.Quota{TenantID: actor.TenantID, TaskType: taskType, MaxRequestsDaily: req.MaxRequestsDaily, MaxTokensDaily: req.MaxTokensDaily, MaxConcurrent: req.MaxConcurrent}
	if count > 0 {
		if err := s.db.Model(&model.Quota{}).Where("tenant_id = ? AND task_type = ?", actor.TenantID, taskType).Select("*").Updates(&quota).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := s.db.Create(&quota).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *UsageService) ResetQuota(actor Actor, taskType string) *errs.Error {
	if strings.TrimSpace(taskType) != "assistant" {
		return errs.ErrParam.WithMsg("任务类型无效")
	}
	if err := s.db.Where("tenant_id = ? AND task_type = ?", actor.TenantID, taskType).Delete(&model.Quota{}).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
