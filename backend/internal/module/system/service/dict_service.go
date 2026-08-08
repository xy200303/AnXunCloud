package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
)

// DictService 字典管理服务。
type DictService struct {
	db *gorm.DB
}

func NewDictService(db *gorm.DB) *DictService { return &DictService{db: db} }

// ListTypes 字典类型分页列表（data_count 为其下字典数据条数）。
func (s *DictService) ListTypes(q *dto.DictTypeQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysDictType{})
	if q.Code != "" {
		db = db.Where("code LIKE ?", "%"+q.Code+"%")
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysDictType
	offset, limit := q.Normalize()
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		var dataCount int64
		s.db.Model(&model.SysDictData{}).Where("type_code = ?", r.Code).Count(&dataCount)
		list = append(list, map[string]any{
			"id":         r.ID,
			"code":       r.Code,
			"name":       r.Name,
			"remark":     r.Remark,
			"data_count": dataCount,
			"created_at": timefmt.T(r.CreatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// CreateType 新增字典类型（code 唯一）。
func (s *DictService) CreateType(req *dto.DictTypeSaveReq) (string, *errs.Error) {
	if req.Code == "" {
		return "", errs.ErrParam.WithMsg("code 为必填项")
	}
	var count int64
	s.db.Model(&model.SysDictType{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return "", errs.ErrDictCodeExists
	}
	row := model.SysDictType{Code: req.Code, Name: req.Name, Remark: req.Remark}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	return row.ID, nil
}

// UpdateType 修改字典类型（code 不可改）。
func (s *DictService) UpdateType(id string, req *dto.DictTypeSaveReq) *errs.Error {
	var row model.SysDictType
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := s.db.Model(&row).Updates(map[string]any{"name": req.Name, "remark": req.Remark}).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// DeleteType 删除字典类型，级联删除其下字典数据。
func (s *DictService) DeleteType(id string) *errs.Error {
	var row model.SysDictType
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("type_code = ?", row.Code).Delete(&model.SysDictData{}).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// ListData 字典数据分页列表。
func (s *DictService) ListData(q *dto.DictDataQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysDictData{}).Where("type_code = ?", q.TypeCode)
	if q.Label != "" {
		db = db.Where("label LIKE ?", "%"+q.Label+"%")
	}
	if q.Status != nil {
		db = db.Where("status = ?", model.StatusStr(*q.Status))
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.SysDictData
	offset, limit := q.Normalize()
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		list = append(list, map[string]any{
			"id":        r.ID,
			"type_code": r.TypeCode,
			"label":     r.Label,
			"value":     r.Value,
			"sort":      r.Sort,
			"status":    model.StatusInt(r.Status),
			"remark":    r.Remark,
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// CreateData 新增字典数据（类型须存在，同类型下 value 唯一）。
func (s *DictService) CreateData(req *dto.DictDataSaveReq) (string, *errs.Error) {
	if req.TypeCode == "" {
		return "", errs.ErrParam.WithMsg("type_code 为必填项")
	}
	var typeCount int64
	s.db.Model(&model.SysDictType{}).Where("code = ?", req.TypeCode).Count(&typeCount)
	if typeCount == 0 {
		return "", errs.ErrParam.WithMsg("字典类型不存在")
	}
	var count int64
	s.db.Model(&model.SysDictData{}).Where("type_code = ? AND value = ?", req.TypeCode, req.Value).Count(&count)
	if count > 0 {
		return "", errs.ErrDictCodeExists.WithMsg("同类型下字典值已存在")
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = model.StatusStr(*req.Status)
	}
	row := model.SysDictData{
		TypeCode: req.TypeCode,
		Label:    req.Label,
		Value:    req.Value,
		Sort:     req.Sort,
		Status:   status,
		Remark:   req.Remark,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	return row.ID, nil
}

// UpdateData 修改字典数据（type_code 不可改）。
func (s *DictService) UpdateData(id string, req *dto.DictDataSaveReq) *errs.Error {
	var row model.SysDictData
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	// value 变更时校验同类型唯一
	if req.Value != row.Value {
		var count int64
		s.db.Model(&model.SysDictData{}).
			Where("type_code = ? AND value = ? AND id <> ?", row.TypeCode, req.Value, id).Count(&count)
		if count > 0 {
			return errs.ErrDictCodeExists.WithMsg("同类型下字典值已存在")
		}
	}
	updates := map[string]any{"label": req.Label, "value": req.Value, "sort": req.Sort, "remark": req.Remark}
	if req.Status != nil {
		updates["status"] = model.StatusStr(*req.Status)
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// DeleteData 删除字典数据。
func (s *DictService) DeleteData(id string) *errs.Error {
	res := s.db.Delete(&model.SysDictData{}, "id = ?", id)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}
