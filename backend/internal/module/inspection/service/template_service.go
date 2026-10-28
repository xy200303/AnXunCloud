package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// TemplateService 检查项模板服务。
// v22 起模板按租户隔离（tenant_id 直挂，不走 community 链）：列表/详情/编辑/删除均以租户上下文收口
// （非超管=本人租户；超管=所选租户，缺省默认租户，见 middleware.TenantScopeOrDefault）。
// tenant_id 为 NULL 的历史行视为平台全局行：列表仅超管在默认租户上下文不可见——实际仅超管可改，各租户列表不出现。
// v18 起检查项存 check_template_item 独立表；对外 items JSON 结构不变（新增 requirement 字段）。
type TemplateService struct {
	db *gorm.DB
}

func NewTemplateService(db *gorm.DB) *TemplateService { return &TemplateService{db: db} }

// List 模板分页列表（按租户上下文隔离；tenant_id 为 NULL 的历史全局行不出现在租户列表）。
func (s *TemplateService) List(c *gin.Context, q *dto.TemplateListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.CheckTemplate{})
	tid, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return nil, be
	}
	if tid != "" {
		db = db.Where("tenant_id = ?", tid)
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.PointType != "" {
		db = db.Where("point_type = ?", q.PointType)
	}
	if q.Purpose != "" {
		db = db.Where("purpose = ?", q.Purpose)
	}
	if q.Status == model.TemplateStatusDraft || q.Status == model.TemplateStatusEnabled {
		db = db.Where("status = ?", q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.CheckTemplate
	offset, limit := q.Normalize()
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	itemsByTpl := s.loadItems(templateIDs(rows))
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, templateItem(&rows[i], itemsByTpl[rows[i].ID]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func templateIDs(rows []model.CheckTemplate) []string {
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	return ids
}

// loadItems 批量加载模板检查项（按 sort 升序），key 为 template_id。
func (s *TemplateService) loadItems(tplIDs []string) map[string][]model.CheckTemplateItem {
	out := map[string][]model.CheckTemplateItem{}
	if len(tplIDs) == 0 {
		return out
	}
	var items []model.CheckTemplateItem
	s.db.Where("template_id IN ? AND retired_at IS NULL", tplIDs).Order("template_id ASC, sort ASC").Find(&items)
	for _, it := range items {
		out[it.TemplateID] = append(out[it.TemplateID], it)
	}
	return out
}

func templateItem(t *model.CheckTemplate, items []model.CheckTemplateItem) gin.H {
	return gin.H{
		"id": t.ID, "code": t.Code, "name": t.Name, "point_type": t.PointType,
		"purpose": t.Purpose, "items": templateItemViews(items), "sort": t.Sort,
		"status": t.Status, "published_at": t.PublishedAt, "remark": t.Remark,
		"created_at": timefmt.T(t.CreatedAt), "updated_at": timefmt.T(t.UpdatedAt),
	}
}

func validateTemplateHeader(req *dto.TemplateSaveReq) *errs.Error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.PointType) == "" {
		return errs.ErrParam.WithMsg("模板名称与点位类型必填")
	}
	switch req.Purpose {
	case model.TemplatePurposePatrol, model.TemplatePurposeSpot, model.TemplatePurposeVerify:
	default:
		return errs.ErrParam.WithMsg("模板用途只能是 patrol、spot 或 verify")
	}
	return nil
}

func templateItemViews(items []model.CheckTemplateItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for i := range items {
		out = append(out, itemRowView(&items[i]))
	}
	return out
}

// Create 新增模板（模板 + 项行同事务写入）。
func (s *TemplateService) Create(c *gin.Context, req *dto.TemplateSaveReq) (string, *errs.Error) {
	tid, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return "", be
	}
	if be := validateTemplateHeader(req); be != nil {
		return "", be
	}
	t := model.CheckTemplate{
		Code: types.NewID(), Name: strings.TrimSpace(req.Name), PointType: strings.TrimSpace(req.PointType),
		Purpose: req.Purpose, Sort: req.Sort, Status: model.TemplateStatusDraft, Remark: strings.TrimSpace(req.Remark),
	}
	if tid != "" {
		t.TenantID = &tid
	}
	if err := s.db.Create(&t).Error; err != nil {
		return "", errs.ErrInternal
	}
	return t.ID, nil
}

// loadOwned 加载模板并校验租户归属（tenant_id 直挂行，无 community 链；越权按不存在口径）。
func (s *TemplateService) loadOwned(c *gin.Context, id string) (*model.CheckTemplate, *errs.Error) {
	var t model.CheckTemplate
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckTenantRow(c, s.db, t.TenantID); be != nil {
		return nil, errs.ErrNotFound
	}
	return &t, nil
}

// Detail 模板详情。
func (s *TemplateService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	t, be := s.loadOwned(c, id)
	if be != nil {
		return nil, be
	}
	return templateItem(t, s.loadItems([]string{id})[id]), nil
}

// Update 修改模板自身字段（检查项由项级接口单独维护）。
func (s *TemplateService) Update(c *gin.Context, id string, req *dto.TemplateSaveReq) *errs.Error {
	t, be := s.loadOwned(c, id)
	if be != nil {
		return be
	}
	if be := validateTemplateHeader(req); be != nil {
		return be
	}
	if req.Status != model.TemplateStatusDraft && req.Status != model.TemplateStatusEnabled {
		return errs.ErrParam.WithMsg("模板状态只能是 draft 或 enabled")
	}
	updates := map[string]any{
		"name": strings.TrimSpace(req.Name), "point_type": strings.TrimSpace(req.PointType),
		"purpose": req.Purpose, "sort": req.Sort, "remark": strings.TrimSpace(req.Remark), "status": req.Status,
	}
	if req.Status == model.TemplateStatusEnabled {
		var items []model.CheckTemplateItem
		if err := s.db.Where("template_id = ? AND retired_at IS NULL", id).Find(&items).Error; err != nil {
			return errs.ErrInternal
		}
		if len(items) == 0 {
			return errs.ErrConflict.WithMsg("至少配置一个检查项后才能发布模板")
		}
		for i := range items {
			if err := items[i].ValidateDefinition(); err != nil {
				return errs.ErrConflict.WithMsg("检查项「" + items[i].Name + "」：" + err.Error())
			}
		}
		if t.Status != model.TemplateStatusEnabled {
			updates["published_at"] = time.Now()
		}
	}
	if err := s.db.Model(t).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 删除模板；被点位引用时拒绝并提示引用数量（引用口径 = point_template 关联行，软删点位不计）。
func (s *TemplateService) Delete(c *gin.Context, id string) *errs.Error {
	t, be := s.loadOwned(c, id)
	if be != nil {
		return be
	}
	var count int64
	s.db.Model(&model.PointTemplate{}).
		Joins("JOIN inspection_point ON inspection_point.id = point_template.point_id AND inspection_point.deleted_at IS NULL").
		Where("point_template.template_id = ?", id).Count(&count)
	if count > 0 {
		return errs.ErrParam.WithMsg(fmt.Sprintf("模板已被 %d 个点位引用，不可删除", count))
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(t).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CheckTemplateItem{}).Where("template_id = ? AND retired_at IS NULL", id).Update("retired_at", time.Now()).Error; err != nil {
			return err
		}
		return tx.Where("template_id = ?", id).Delete(&model.PointTemplate{}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

func normalizeTags(tags []string) types.StringArray {
	out := make(types.StringArray, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || len([]rune(t)) > 30 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func templateItemFromRequest(req *dto.TemplateItemSaveReq) (*model.CheckTemplateItem, *errs.Error) {
	row := &model.CheckTemplateItem{
		Code: strings.ToLower(strings.TrimSpace(req.Code)), Name: strings.TrimSpace(req.Name),
		Standard: strings.TrimSpace(req.Standard), StandardNo: strings.TrimSpace(req.StandardNo),
		ValueType: req.ValueType, Unit: strings.TrimSpace(req.Unit), ValueMin: req.ValueMin, ValueMax: req.ValueMax,
		AIPromptKind: req.AIPromptKind, AIEnabled: req.AIEnabled, PhotoRequired: req.PhotoRequired,
		NoteRequired: req.NoteRequired, Tags: normalizeTags(req.Tags),
	}
	if row.AIPromptKind == "" {
		row.AIPromptKind = "general"
	}
	if row.PhotoRequired == "" {
		row.PhotoRequired = types.PhotoReqNone
	}
	for _, option := range req.EnumOptions {
		row.EnumOptions = append(row.EnumOptions, model.EnumOption{
			Value: strings.TrimSpace(option.Value), Label: strings.TrimSpace(option.Label), Result: option.Result,
		})
	}
	if h := strings.TrimSpace(req.AIHint); h != "" {
		row.AIHint = &h
	}
	if g := strutil.Truncate(strings.TrimSpace(req.Guide), 200); g != "" {
		row.Guide = &g
	}
	switch row.PhotoRequired {
	case types.PhotoReqNone, types.PhotoReqOptional, types.PhotoReqRequired:
	default:
		return nil, errs.ErrParam.WithMsg("拍照要求只能是 none、optional 或 required")
	}
	if err := row.ValidateDefinition(); err != nil {
		return nil, errs.ErrParam.WithMsg(err.Error())
	}
	return row, nil
}

func itemRowView(it *model.CheckTemplateItem) gin.H {
	return gin.H{
		"id": it.ID, "code": it.Code, "name": it.Name, "standard": it.Standard, "standard_no": it.StandardNo,
		"value_type": it.ValueType, "unit": it.Unit, "value_min": it.ValueMin, "value_max": it.ValueMax,
		"enum_options": it.EnumOptions, "ai_hint": it.AIHint, "ai_prompt_kind": it.AIPromptKind,
		"ai_enabled": it.AIEnabled, "note_required": it.NoteRequired, "tags": it.Tags, "guide": it.Guide,
		"photo_required": it.PhotoRequired, "sort": it.Sort, "created_at": timefmt.T(it.CreatedAt),
	}
}

func (s *TemplateService) Items(c *gin.Context, templateID string) ([]gin.H, *errs.Error) {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return nil, be
	}
	var rows []model.CheckTemplateItem
	if err := s.db.Where("template_id = ? AND retired_at IS NULL", templateID).Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := make([]gin.H, 0, len(rows))
	for i := range rows {
		out = append(out, itemRowView(&rows[i]))
	}
	return out, nil
}

func (s *TemplateService) AddItem(c *gin.Context, templateID string, req *dto.TemplateItemSaveReq) (string, *errs.Error) {
	tpl, be := s.loadOwned(c, templateID)
	if be != nil {
		return "", be
	}
	row, be := templateItemFromRequest(req)
	if be != nil {
		return "", be
	}
	row.TemplateID, row.TenantID = templateID, tpl.TenantID
	if req.Sort != nil {
		row.Sort = *req.Sort
	} else {
		s.db.Model(&model.CheckTemplateItem{}).Where("template_id = ?", templateID).
			Select("COALESCE(MAX(sort), -1)").Scan(&row.Sort)
		row.Sort++
	}
	if err := s.db.Create(row).Error; err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return "", errs.ErrConflict.WithMsg("检查项编码已存在（含已退役检查项）")
		}
		return "", errs.ErrInternal
	}
	return row.ID, nil
}

func (s *TemplateService) findItem(templateID, itemID string) (*model.CheckTemplateItem, *errs.Error) {
	var row model.CheckTemplateItem
	if err := s.db.First(&row, "id = ? AND template_id = ? AND retired_at IS NULL", itemID, templateID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return &row, nil
}

func (s *TemplateService) UpdateItem(c *gin.Context, templateID, itemID string, req *dto.TemplateItemSaveReq) *errs.Error {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return be
	}
	current, be := s.findItem(templateID, itemID)
	if be != nil {
		return be
	}
	if !strings.EqualFold(strings.TrimSpace(req.Code), current.Code) {
		return errs.ErrConflict.WithMsg("检查项编码创建后不可修改")
	}
	row, be := templateItemFromRequest(req)
	if be != nil {
		return be
	}
	row.ID, row.TemplateID, row.TenantID, row.CreatedAt = current.ID, templateID, current.TenantID, current.CreatedAt
	row.Code = current.Code
	if req.Sort == nil {
		row.Sort = current.Sort
	} else {
		row.Sort = *req.Sort
	}
	if err := s.db.Save(row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *TemplateService) DeleteItem(c *gin.Context, templateID, itemID string) *errs.Error {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return be
	}
	row, be := s.findItem(templateID, itemID)
	if be != nil {
		return be
	}
	if err := s.db.Model(row).Update("retired_at", time.Now()).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
