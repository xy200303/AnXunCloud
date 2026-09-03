package service

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
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
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", status)
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
	s.db.Where("template_id IN ?", tplIDs).Order("template_id ASC, sort ASC").Find(&items)
	for _, it := range items {
		out[it.TemplateID] = append(out[it.TemplateID], it)
	}
	return out
}

func templateItem(t *model.CheckTemplate, items []model.CheckTemplateItem) gin.H {
	return gin.H{
		"id": t.ID, "name": t.Name, "point_type": t.PointType, "items": templateItemViews(items),
		"sort": t.Sort, "status": sysmodel.StatusInt(t.Status), "remark": t.Remark,
		"created_at": timefmt.T(t.CreatedAt), "updated_at": timefmt.T(t.UpdatedAt),
	}
}

// templateItemViews 模板项视图：name/required/photo_required 结构不变，新增 requirement/ai_hint（空=null）。
func templateItemViews(items []model.CheckTemplateItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"name": it.Name, "required": it.Required,
			"photo_required": it.PhotoRequired, "requirement": it.Requirement,
			"ai_hint": it.AIHint, "judge_type": it.JudgeType, "judge_config": it.JudgeConfig,
		})
	}
	return out
}

// Create 新增模板（模板 + 项行同事务写入）。
func (s *TemplateService) Create(c *gin.Context, req *dto.TemplateSaveReq) (string, *errs.Error) {
	tid, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return "", be
	}
	t := model.CheckTemplate{
		Name:      strings.TrimSpace(req.Name),
		PointType: req.PointType,
		Sort:      req.Sort,
		Status:    sysmodel.StatusEnabled,
		Remark:    req.Remark,
	}
	if tid != "" {
		t.TenantID = &tid // 租户归属（v22 起新建模板必有租户）
	}
	if req.Status != nil {
		t.Status = sysmodel.StatusStr(*req.Status)
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
	updates := map[string]any{
		"name": strings.TrimSpace(req.Name), "point_type": req.PointType,
		"sort": req.Sort, "remark": req.Remark,
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(t).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 删除模板；被点位引用时拒绝并提示引用数量。
func (s *TemplateService) Delete(c *gin.Context, id string) *errs.Error {
	t, be := s.loadOwned(c, id)
	if be != nil {
		return be
	}
	var count int64
	s.db.Model(&model.InspectionPoint{}).Where("template_id = ?", id).Count(&count)
	if count > 0 {
		return errs.ErrParam.WithMsg(fmt.Sprintf("模板已被 %d 个点位引用，不可删除", count))
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(t).Error; err != nil {
			return err
		}
		// 模板软删，级联删除不触发，项行显式清除
		return tx.Where("template_id = ?", id).Delete(&model.CheckTemplateItem{}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// validateItem 单个检查项校验：名称非空，photo_required 枚举合法。
func validateItem(name, photoRequired string) *errs.Error {
	if strings.TrimSpace(name) == "" {
		return errs.ErrParam.WithMsg("检查项名称不能为空")
	}
	switch photoRequired {
	case "", types.PhotoReqNone, types.PhotoReqOptional, types.PhotoReqRequired:
	default:
		return errs.ErrParam.WithMsg("检查项「" + name + "」photo_required 取值应为 none/optional/required")
	}
	return nil
}

// ========== 项级粒度接口 ==========

// itemRowView 项级接口视图：含 id/sort/created_at（区别于模板内嵌 items 快照视图）。
func itemRowView(it *model.CheckTemplateItem) gin.H {
	return gin.H{
		"id": it.ID, "name": it.Name, "requirement": it.Requirement,
		"ai_hint": it.AIHint, "judge_type": it.JudgeType, "judge_config": it.JudgeConfig,
		"required": it.Required, "photo_required": it.PhotoRequired,
		"sort": it.Sort, "created_at": timefmt.T(it.CreatedAt),
	}
}

// Items 模板检查项列表（按 sort 升序）。
func (s *TemplateService) Items(c *gin.Context, templateID string) ([]gin.H, *errs.Error) {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return nil, be
	}
	var rows []model.CheckTemplateItem
	if err := s.db.Where("template_id = ?", templateID).Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := make([]gin.H, 0, len(rows))
	for i := range rows {
		out = append(out, itemRowView(&rows[i]))
	}
	return out, nil
}

// AddItem 新增一个检查项；sort 缺省时追加到末尾。
func (s *TemplateService) AddItem(c *gin.Context, templateID string, req *dto.TemplateItemSaveReq) (string, *errs.Error) {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return "", be
	}
	if be := validateItem(req.Name, req.PhotoRequired); be != nil {
		return "", be
	}
	sort := 0
	if req.Sort != nil {
		sort = *req.Sort
	} else {
		var maxSort int
		s.db.Model(&model.CheckTemplateItem{}).Where("template_id = ?", templateID).
			Select("COALESCE(MAX(sort), -1)").Scan(&maxSort)
		sort = maxSort + 1
	}
	row := model.CheckTemplateItem{
		TemplateID: templateID, Name: strings.TrimSpace(req.Name),
		Required: req.Required, PhotoRequired: req.PhotoRequired, Sort: sort,
		// 判定类型非法值归一 general（不报错，容错旧客户端）
		JudgeType: ai.NormalizeJudgeType(req.JudgeType),
	}
	if len(req.JudgeConfig) > 0 {
		row.JudgeConfig = types.JSONMap(req.JudgeConfig)
	}
	if row.PhotoRequired == "" {
		row.PhotoRequired = types.PhotoReqNone
	}
	if r := strings.TrimSpace(req.Requirement); r != "" {
		row.Requirement = &r
	}
	if h := strings.TrimSpace(req.AIHint); h != "" {
		row.AIHint = &h
	}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	return row.ID, nil
}

// findItem 按模板 + 项 id 定位项行。
func (s *TemplateService) findItem(templateID, itemID string) (*model.CheckTemplateItem, *errs.Error) {
	var row model.CheckTemplateItem
	if err := s.db.First(&row, "id = ? AND template_id = ?", itemID, templateID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return &row, nil
}

// UpdateItem 修改一个检查项；sort 缺省保持不变。
func (s *TemplateService) UpdateItem(c *gin.Context, templateID, itemID string, req *dto.TemplateItemSaveReq) *errs.Error {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return be
	}
	row, be := s.findItem(templateID, itemID)
	if be != nil {
		return be
	}
	if be := validateItem(req.Name, req.PhotoRequired); be != nil {
		return be
	}
	pr := req.PhotoRequired
	if pr == "" {
		pr = types.PhotoReqNone
	}
	updates := map[string]any{
		"name": strings.TrimSpace(req.Name), "required": req.Required, "photo_required": pr,
		"judge_type": ai.NormalizeJudgeType(req.JudgeType),
	}
	if len(req.JudgeConfig) > 0 {
		updates["judge_config"] = types.JSONMap(req.JudgeConfig)
	} else {
		updates["judge_config"] = nil
	}
	if r := strings.TrimSpace(req.Requirement); r != "" {
		updates["requirement"] = r
	} else {
		updates["requirement"] = nil
	}
	if h := strings.TrimSpace(req.AIHint); h != "" {
		updates["ai_hint"] = h
	} else {
		updates["ai_hint"] = nil
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if err := s.db.Model(row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// DeleteItem 删除一个检查项（历史打卡记录已快照进 checkin_record_item，不受影响）。
func (s *TemplateService) DeleteItem(c *gin.Context, templateID, itemID string) *errs.Error {
	if _, be := s.loadOwned(c, templateID); be != nil {
		return be
	}
	row, be := s.findItem(templateID, itemID)
	if be != nil {
		return be
	}
	if err := s.db.Delete(row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
