package service

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// TemplateService 检查项模板服务（模板全局共享，不做小区数据权限）。
// v18 起检查项存 check_template_item 独立表；对外 items JSON 结构不变（新增 requirement 字段）。
type TemplateService struct {
	db *gorm.DB
}

func NewTemplateService(db *gorm.DB) *TemplateService { return &TemplateService{db: db} }

// List 模板分页列表。
func (s *TemplateService) List(q *dto.TemplateListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.CheckTemplate{})
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

// templateItemViews 模板项视图：name/required/photo_required 结构不变，新增 requirement（空=null）。
func templateItemViews(items []model.CheckTemplateItem) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"name": it.Name, "required": it.Required,
			"photo_required": it.PhotoRequired, "requirement": it.Requirement,
		})
	}
	return out
}

// Create 新增模板（模板 + 项行同事务写入）。
func (s *TemplateService) Create(req *dto.TemplateSaveReq) (string, *errs.Error) {
	if be := validateTemplate(req); be != nil {
		return "", be
	}
	t := model.CheckTemplate{
		Name:      strings.TrimSpace(req.Name),
		PointType: req.PointType,
		Sort:      req.Sort,
		Status:    sysmodel.StatusEnabled,
		Remark:    req.Remark,
	}
	if req.Status != nil {
		t.Status = sysmodel.StatusStr(*req.Status)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		return tx.Create(toItemRows(t.ID, req.Items)).Error
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	return t.ID, nil
}

// Detail 模板详情。
func (s *TemplateService) Detail(id string) (gin.H, *errs.Error) {
	var t model.CheckTemplate
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	return templateItem(&t, s.loadItems([]string{id})[id]), nil
}

// Update 修改模板（事务内更新模板字段 + 整表替换项行）。
func (s *TemplateService) Update(id string, req *dto.TemplateSaveReq) *errs.Error {
	var t model.CheckTemplate
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := validateTemplate(req); be != nil {
		return be
	}
	updates := map[string]any{
		"name": strings.TrimSpace(req.Name), "point_type": req.PointType,
		"sort": req.Sort, "remark": req.Remark,
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&t).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("template_id = ?", id).Delete(&model.CheckTemplateItem{}).Error; err != nil {
			return err
		}
		return tx.Create(toItemRows(id, req.Items)).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 删除模板；被点位引用时拒绝并提示引用数量。
func (s *TemplateService) Delete(id string) *errs.Error {
	var t model.CheckTemplate
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	var count int64
	s.db.Model(&model.InspectionPoint{}).Where("template_id = ?", id).Count(&count)
	if count > 0 {
		return errs.ErrParam.WithMsg(fmt.Sprintf("模板已被 %d 个点位引用，不可删除", count))
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&t).Error; err != nil {
			return err
		}
		// 模板软删，级联删除不触发，项行显式清除（checkin_record_item.template_item_id 血缘不受影响）
		return tx.Where("template_id = ?", id).Delete(&model.CheckTemplateItem{}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// validateTemplate 模板项校验：至少 1 项且每项名称非空，photo_required 枚举合法。
func validateTemplate(req *dto.TemplateSaveReq) *errs.Error {
	for _, it := range req.Items {
		if strings.TrimSpace(it.Name) == "" {
			return errs.ErrParam.WithMsg("检查项名称不能为空")
		}
		switch it.PhotoRequired {
		case "", types.PhotoReqNone, types.PhotoReqOptional, types.PhotoReqRequired:
		default:
			return errs.ErrParam.WithMsg("检查项「" + it.Name + "」photo_required 取值应为 none/optional/required")
		}
	}
	return nil
}

// toItemRows 请求项 → 模板项行（sort 按提交顺序）。
func toItemRows(templateID string, items []dto.TemplateItemReq) []model.CheckTemplateItem {
	out := make([]model.CheckTemplateItem, 0, len(items))
	for i, it := range items {
		pr := it.PhotoRequired
		if pr == "" {
			pr = types.PhotoReqNone
		}
		row := model.CheckTemplateItem{
			TemplateID: templateID, Name: strings.TrimSpace(it.Name),
			Required: it.Required, PhotoRequired: pr, Sort: i,
		}
		if r := strings.TrimSpace(it.Requirement); r != "" {
			row.Requirement = &r
		}
		out = append(out, row)
	}
	return out
}
