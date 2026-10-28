package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

// PostService 保留既有装配名称，只负责集中审批流程配置。
type PostService struct {
	db *gorm.DB
}

func NewPostService(db *gorm.DB) *PostService { return &PostService{db: db} }

func CopyFlowTemplatesToTenant(tx *gorm.DB, tenantID string) error {
	var flows []model.ApprovalFlow
	if err := tx.Where("scope = ? AND scope_id = ?", model.ScopePlatform, model.PlatformScopeID).Find(&flows).Error; err != nil {
		return err
	}
	for _, source := range flows {
		var count int64
		if err := tx.Model(&model.ApprovalFlow{}).Where("scope = ? AND scope_id = ? AND flow_code = ?", model.ScopeTenant, tenantID, source.FlowCode).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		flow := model.ApprovalFlow{Scope: model.ScopeTenant, ScopeID: tenantID, FlowCode: source.FlowCode, Steps: source.Steps}
		if err := tx.Create(&flow).Error; err != nil {
			return err
		}
	}
	return nil
}

func flowScope(tenantID *string) (string, string) {
	if tenantID == nil {
		return model.ScopePlatform, model.PlatformScopeID
	}
	return model.ScopeTenant, *tenantID
}

func (s *PostService) GetFlow(tenantID *string, code string) (gin.H, *errs.Error) {
	if !model.ValidApprovalFlowCode(code) {
		return nil, errs.ErrParam.WithMsg("未知审批流程")
	}
	scope, scopeID := flowScope(tenantID)
	var flow model.ApprovalFlow
	source := scope
	if err := s.db.Where("scope = ? AND scope_id = ? AND flow_code = ?", scope, scopeID, code).First(&flow).Error; err != nil {
		flow.Steps = communitysvc.DefaultFlow(s.db, code)
		source = "default"
	}
	return gin.H{"flow_code": code, "steps": flow.Steps, "source": source}, nil
}

func (s *PostService) SaveFlow(tenantID *string, code string, steps types.FlowStepArray) *errs.Error {
	if !model.ValidApprovalFlowCode(code) {
		return errs.ErrParam.WithMsg("未知审批流程")
	}
	var be *errs.Error
	if code == model.FlowCheckinReview || code == model.FlowMaintReview {
		be = communitysvc.ValidateFlowSteps(s.db, steps)
	} else {
		be = communitysvc.ValidateHumanFlowSteps(s.db, steps)
	}
	if be != nil {
		return be
	}
	scope, scopeID := flowScope(tenantID)
	var flow model.ApprovalFlow
	if err := s.db.Where("scope = ? AND scope_id = ? AND flow_code = ?", scope, scopeID, code).First(&flow).Error; err != nil {
		flow = model.ApprovalFlow{Scope: scope, ScopeID: scopeID, FlowCode: code, Steps: steps}
		if err := s.db.Create(&flow).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := s.db.Model(&flow).Update("steps", steps).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
