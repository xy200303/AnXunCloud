package service

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/acceptance/model"
	inspectionsvc "anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/errs"
)

type ConfigScope struct {
	Name string
	ID   string
}

type SchemaReq struct {
	Code       string  `json:"code"`
	Category   string  `json:"category" binding:"required"`
	Name       string  `json:"name" binding:"required"`
	TableKind  string  `json:"table_kind" binding:"required"`
	TemplateID *string `json:"template_id"`
	Status     string  `json:"status"`
}

type RowReq struct {
	Name          string `json:"name" binding:"required"`
	Specification string `json:"specification"`
	Unit          string `json:"unit"`
	Required      bool   `json:"required"`
	Sort          int    `json:"sort"`
	Status        string `json:"status"`
}

type SchemaService struct {
	db              *gorm.DB
	verifyTemplates func(string) ([]inspectionsvc.VerifyTemplateRef, error)
}

func NewSchemaService(db *gorm.DB, verifyTemplates func(string) ([]inspectionsvc.VerifyTemplateRef, error)) *SchemaService {
	return &SchemaService{db: db, verifyTemplates: verifyTemplates}
}

var stableCode = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func validSchemaStatus(status string) bool {
	return status == model.StatusDraft || status == model.Enabled || status == model.Disabled
}

func (s *SchemaService) List(scope ConfigScope) ([]gin.H, *errs.Error) {
	var schemas []model.ListSchema
	if err := scopedQuery(s.db, scope, &model.ListSchema{}).Order("category, code").Find(&schemas).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if scope.Name == model.ScopeTenant {
		var inherited []model.ListSchema
		if err := s.db.Where("scope = ? AND scope_id = ?", model.ScopePlatform, model.PlatformScopeID).Order("category, code").Find(&inherited).Error; err != nil {
			return nil, errs.ErrInternal
		}
		seen := map[string]bool{}
		for _, schema := range schemas {
			seen[schema.Code] = true
		}
		for _, schema := range inherited {
			if !seen[schema.Code] {
				schemas = append(schemas, schema)
			}
		}
	}
	var rows []model.ListRow
	if err := scopedQuery(s.db, scope, &model.ListRow{}).Order("table_code, sort, code").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if scope.Name == model.ScopeTenant {
		var inherited []model.ListRow
		if err := s.db.Where("scope = ? AND scope_id = ?", model.ScopePlatform, model.PlatformScopeID).Order("table_code, sort, code").Find(&inherited).Error; err != nil {
			return nil, errs.ErrInternal
		}
		seen := map[string]bool{}
		for _, row := range rows {
			seen[row.TableCode+"\x00"+row.Code] = true
		}
		for _, row := range inherited {
			if !seen[row.TableCode+"\x00"+row.Code] {
				rows = append(rows, row)
			}
		}
	}
	rowMap := map[string][]model.ListRow{}
	for _, row := range rows {
		if row.Status == model.Enabled {
			rowMap[row.TableCode] = append(rowMap[row.TableCode], row)
		}
	}
	templateNames := map[string]string{}
	if s.verifyTemplates != nil {
		templates, err := s.verifyTemplates(scope.ID)
		if err != nil {
			return nil, errs.ErrInternal
		}
		for _, template := range templates {
			templateNames[template.ID] = template.Name
		}
	}
	items := make([]gin.H, 0, len(schemas))
	for _, schema := range schemas {
		items = append(items, gin.H{"code": schema.Code, "category": schema.Category, "name": schema.Name, "table_kind": schema.TableKind, "template_id": schema.TemplateID, "template_name": templateNames[strValue(schema.TemplateID)], "status": schema.Status, "source": schema.Scope, "rows": rowMap[schema.Code]})
	}
	return items, nil
}

func scopedQuery(db *gorm.DB, scope ConfigScope, prototype any) *gorm.DB {
	if scope.Name == model.ScopeTenant {
		return db.Model(prototype).Where("scope = ? AND scope_id = ?", model.ScopeTenant, scope.ID)
	}
	return db.Model(prototype).Where("scope = ? AND scope_id = ?", model.ScopePlatform, model.PlatformScopeID)
}

func strValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *SchemaService) SaveSchema(scope ConfigScope, code string, req SchemaReq) *errs.Error {
	if code == "" {
		code = req.Code
	}
	if !stableCode.MatchString(code) || strings.TrimSpace(req.Name) == "" || !validSchemaStatus(req.Status) {
		return errs.ErrParam.WithMsg("专项编码、名称或状态无效")
	}
	categories := map[string]bool{"documents": true, "building": true, "equipment": true, "outdoor": true, "office": true, "garage": true, "unit": true}
	kinds := map[string]bool{"matrix": true, "documents": true, "keys": true, "assets": true}
	if !categories[req.Category] || !kinds[req.TableKind] {
		return errs.ErrParam.WithMsg("查验类别或表型无效")
	}
	if req.TableKind == "matrix" {
		if req.TemplateID == nil || *req.TemplateID == "" || s.verifyTemplates == nil {
			return errs.ErrParam.WithMsg("矩阵专项必须选择 verify 检查模板")
		}
		templates, err := s.verifyTemplates(scope.ID)
		if err != nil {
			return errs.ErrInternal
		}
		found := false
		for _, template := range templates {
			if template.ID == *req.TemplateID {
				found = true
				break
			}
		}
		if !found {
			return errs.ErrParam.WithMsg("verify 检查模板不存在或未启用")
		}
	} else {
		req.TemplateID = nil
	}
	row := model.ListSchema{Scope: scope.Name, ScopeID: scope.ID, Code: code, Category: req.Category, Name: strings.TrimSpace(req.Name), TableKind: req.TableKind, TemplateID: req.TemplateID, Status: req.Status}
	return saveConfigRow(s.db, &row, "scope = ? AND scope_id = ? AND code = ?", scope.Name, scope.ID, code)
}

func (s *SchemaService) SaveRow(scope ConfigScope, tableCode, code string, req RowReq) *errs.Error {
	if !stableCode.MatchString(tableCode) || !stableCode.MatchString(code) || strings.TrimSpace(req.Name) == "" || (req.Status != model.Enabled && req.Status != model.Disabled) {
		return errs.ErrParam.WithMsg("清单编码、名称或状态无效")
	}
	var count int64
	if err := scopedQuery(s.db, scope, &model.ListSchema{}).Where("code = ?", tableCode).Count(&count).Error; err != nil || count == 0 {
		return errs.ErrParam.WithMsg("清单所属专项不存在")
	}
	row := model.ListRow{Scope: scope.Name, ScopeID: scope.ID, TableCode: tableCode, Code: code, Name: strings.TrimSpace(req.Name), Specification: strings.TrimSpace(req.Specification), Unit: strings.TrimSpace(req.Unit), Required: req.Required, Sort: req.Sort, Status: req.Status}
	return saveConfigRow(s.db, &row, "scope = ? AND scope_id = ? AND table_code = ? AND code = ?", scope.Name, scope.ID, tableCode, code)
}

func saveConfigRow(db *gorm.DB, row any, query string, args ...any) *errs.Error {
	var count int64
	if err := db.Model(row).Where(query, args...).Count(&count).Error; err != nil {
		return errs.ErrInternal
	}
	if count > 0 {
		if err := db.Model(row).Where(query, args...).Select("*").Updates(row).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := db.Create(row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
