package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/report/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

type OutputColumn struct {
	ItemCode string `json:"item_code"`
}

type TemplateOutputBindingReq struct {
	TemplateID string         `json:"template_id" binding:"required"`
	Sort       int            `json:"sort"`
	Columns    []OutputColumn `json:"columns"`
	RowMode    string         `json:"row_mode" binding:"required,oneof=date point grid equipment"`
}

type TemplateOutputService struct{ db *gorm.DB }

func NewTemplateOutputService(db *gorm.DB) *TemplateOutputService {
	return &TemplateOutputService{db: db}
}

func (s *TemplateOutputService) ownedDoc(scope DocTemplateScope, id string) (*model.DocTemplate, *errs.Error) {
	var doc model.DocTemplate
	if scope.Platform {
		if err := s.db.First(&doc, "id = ? AND tenant_id IS NULL", id).Error; err != nil {
			return nil, errs.ErrNotFound
		}
	} else {
		if err := s.db.First(&doc, "id = ? AND tenant_id = ?", id, scope.TenantID).Error; err != nil {
			return nil, errs.ErrNotFound
		}
	}
	return &doc, nil
}

func (s *TemplateOutputService) List(scope DocTemplateScope, docTemplateID string) ([]model.TemplateOutput, *errs.Error) {
	if _, be := s.ownedDoc(scope, docTemplateID); be != nil {
		return nil, be
	}
	var rows []model.TemplateOutput
	if err := s.db.Where("doc_template_id = ?", docTemplateID).Order("sort, id").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return rows, nil
}

func (s *TemplateOutputService) Save(scope DocTemplateScope, docTemplateID string, reqs []TemplateOutputBindingReq) *errs.Error {
	if _, be := s.ownedDoc(scope, docTemplateID); be != nil {
		return be
	}
	if scope.Platform {
		return errs.ErrParam.WithMsg("平台内置输出绑定请复制到租户后配置")
	}
	seen := map[string]bool{}
	rows := make([]model.TemplateOutput, 0, len(reqs))
	for _, req := range reqs {
		if seen[req.TemplateID] {
			return errs.ErrParam.WithMsg("同一检查模板不能重复绑定")
		}
		seen[req.TemplateID] = true
		definition, be := contract.TemplateDefinition(s.db, scope.TenantID, req.TemplateID)
		if be != nil {
			return errs.ErrParam.WithMsg("绑定模板不存在或不属于当前租户")
		}
		if definition.Status != "enabled" || definition.Purpose != "patrol" {
			return errs.ErrParam.WithMsg("只能绑定已启用的日常巡检模板")
		}
		validCodes := map[string]bool{}
		for _, item := range definition.Items {
			validCodes[item.Code] = true
		}
		seenColumns := map[string]bool{}
		for _, column := range req.Columns {
			if !validCodes[column.ItemCode] || seenColumns[column.ItemCode] {
				return errs.ErrParam.WithMsg("输出列引用不存在或重复的 item_code: " + column.ItemCode)
			}
			seenColumns[column.ItemCode] = true
		}
		columns := make([]OutputColumn, len(req.Columns))
		copy(columns, req.Columns)
		rows = append(rows, model.TemplateOutput{
			TenantID: scope.TenantID, DocTemplateID: docTemplateID, TemplateID: req.TemplateID, Sort: req.Sort,
			Columns: types.JSONMap{"items": columns}, RowMode: req.RowMode,
		})
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("doc_template_id = ?", docTemplateID).Delete(&model.TemplateOutput{}).Error; err != nil {
			return err
		}
		for i := range rows {
			if err := tx.Create(&rows[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *TemplateOutputService) StylePacks() ([]model.StylePack, *errs.Error) {
	var rows []model.StylePack
	if err := s.db.Where("status = ?", "enabled").Order("code").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return rows, nil
}

func (s *TemplateOutputService) StylePack(id string) (*model.StylePack, error) {
	var row model.StylePack
	if err := s.db.First(&row, "id = ? AND status = ?", id, "enabled").Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *TemplateOutputService) bindings(tenantID, docTemplateID string) ([]model.TemplateOutput, error) {
	var rows []model.TemplateOutput
	if err := s.db.Where("tenant_id = ? AND doc_template_id = ?", tenantID, docTemplateID).Order("sort, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DocTemplateService) StylePacks() ([]model.StylePack, *errs.Error) {
	return NewTemplateOutputService(s.db).StylePacks()
}

func (s *DocTemplateService) StylePack(id string) (*model.StylePack, error) {
	return NewTemplateOutputService(s.db).StylePack(id)
}

func (s *DocTemplateService) Bindings(scope DocTemplateScope, docTemplateID string) ([]model.TemplateOutput, *errs.Error) {
	return NewTemplateOutputService(s.db).List(scope, docTemplateID)
}

func (s *DocTemplateService) SaveBindings(scope DocTemplateScope, docTemplateID string, reqs []TemplateOutputBindingReq) *errs.Error {
	return NewTemplateOutputService(s.db).Save(scope, docTemplateID, reqs)
}

func (s *DocTemplateService) outputBindings(tenantID, docTemplateID string) ([]model.TemplateOutput, error) {
	return NewTemplateOutputService(s.db).bindings(tenantID, docTemplateID)
}
