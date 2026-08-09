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
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, templateItem(&rows[i]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func templateItem(t *model.CheckTemplate) gin.H {
	return gin.H{
		"id": t.ID, "name": t.Name, "point_type": t.PointType, "items": t.Items,
		"sort": t.Sort, "status": sysmodel.StatusInt(t.Status), "remark": t.Remark,
		"created_at": timefmt.T(t.CreatedAt), "updated_at": timefmt.T(t.UpdatedAt),
	}
}

// Create 新增模板。
func (s *TemplateService) Create(req *dto.TemplateSaveReq) (string, *errs.Error) {
	if be := validateTemplate(req); be != nil {
		return "", be
	}
	t := model.CheckTemplate{
		Name:      strings.TrimSpace(req.Name),
		PointType: req.PointType,
		Items:     toTemplateItems(req.Items),
		Sort:      req.Sort,
		Status:    sysmodel.StatusEnabled,
		Remark:    req.Remark,
	}
	if req.Status != nil {
		t.Status = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Create(&t).Error; err != nil {
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
	return templateItem(&t), nil
}

// Update 修改模板。
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
		"items": toTemplateItems(req.Items), "sort": req.Sort, "remark": req.Remark,
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(&t).Updates(updates).Error; err != nil {
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
	if err := s.db.Delete(&t).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// validateTemplate 模板项校验：至少 1 项且每项名称非空。
func validateTemplate(req *dto.TemplateSaveReq) *errs.Error {
	for _, it := range req.Items {
		if strings.TrimSpace(it.Name) == "" {
			return errs.ErrParam.WithMsg("检查项名称不能为空")
		}
	}
	return nil
}

func toTemplateItems(items []dto.TemplateItemReq) types.TemplateItemArray {
	out := make(types.TemplateItemArray, 0, len(items))
	for _, it := range items {
		out = append(out, types.TemplateItem{Name: strings.TrimSpace(it.Name), Required: it.Required})
	}
	return out
}
