package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/bind"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
)

// RoleService 角色管理服务。
type RoleService struct {
	db *gorm.DB
}

func NewRoleService(db *gorm.DB) *RoleService { return &RoleService{db: db} }

// List 角色分页列表（user_count 统计引用该角色的用户数）。
func (s *RoleService) List(q *dto.RoleListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysRole{})
	if q.Name != "" {
		db = db.Where("name LIKE ? OR code LIKE ?", "%"+q.Name+"%", "%"+q.Name+"%")
	}
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var roles []model.SysRole
	offset, limit := q.Normalize()
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&roles).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]dto.RoleListItem, 0, len(roles))
	for _, r := range roles {
		var userCount int64
		s.db.Model(&model.SysUser{}).Where("role_ids @> ?::jsonb", fmt.Sprintf(`["%s"]`, r.ID)).Count(&userCount)
		items = append(items, dto.RoleListItem{
			ID:        r.ID,
			Code:      r.Code,
			Name:      r.Name,
			DataScope: r.DataScope,
			Remark:    r.Remark,
			Status:    model.StatusInt(r.Status),
			UserCount: userCount,
			CreatedAt: timefmt.T(r.CreatedAt),
		})
	}
	return &response.Page{List: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Create 新增角色（可同时分配菜单）。
func (s *RoleService) Create(req *dto.RoleSaveReq) (string, *errs.Error) {
	if req.Code == "" {
		return "", errs.ErrParam.WithMsg("code 为必填项")
	}
	var count int64
	s.db.Model(&model.SysRole{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return "", errs.ErrRoleCodeExists
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = model.StatusStr(*req.Status)
	}
	role := model.SysRole{
		Code:      req.Code,
		Name:      req.Name,
		DataScope: req.DataScope,
		Remark:    req.Remark,
		Status:    status,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, role.ID, req.MenuIDs)
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return role.ID, nil
}

// Detail 角色详情（含已分配菜单 id）。
func (s *RoleService) Detail(id string) (*dto.RoleDetail, *errs.Error) {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var menuIDs []string
	s.db.Model(&model.SysRoleMenu{}).Where("role_id = ?", id).Order("menu_id ASC").Pluck("menu_id", &menuIDs)
	if menuIDs == nil {
		menuIDs = []string{}
	}
	return &dto.RoleDetail{
		ID:        role.ID,
		Code:      role.Code,
		Name:      role.Name,
		DataScope: role.DataScope,
		Remark:    role.Remark,
		Status:    model.StatusInt(role.Status),
		MenuIDs:   menuIDs,
		CreatedAt: timefmt.T(role.CreatedAt),
	}, nil
}

// Update 修改角色（code 不可改；超管的 data_scope 固定为 all）。
func (s *RoleService) Update(id string, req *dto.RoleSaveReq) *errs.Error {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	updates := map[string]any{"name": req.Name, "remark": req.Remark}
	if role.Code == model.SuperAdminCode {
		updates["data_scope"] = model.ScopeAll
	} else {
		updates["data_scope"] = req.DataScope
	}
	if req.Status != nil {
		updates["status"] = model.StatusStr(*req.Status)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&role).Updates(updates).Error; err != nil {
			return err
		}
		if req.MenuIDs != nil && role.Code != model.SuperAdminCode {
			return replaceRoleMenus(tx, role.ID, req.MenuIDs)
		}
		return nil
	})
	if err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return nil
}

// Delete 删除角色（内置角色不可删；存在用户引用不可删）。
func (s *RoleService) Delete(id string) *errs.Error {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if role.IsBuiltin {
		return errs.ErrBuiltin
	}
	var userCount int64
	s.db.Model(&model.SysUser{}).Where("role_ids @> ?::jsonb", fmt.Sprintf(`["%s"]`, id)).Count(&userCount)
	if userCount > 0 {
		return errs.ErrRoleHasUsers
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&model.SysRoleMenu{}).Error; err != nil {
			return err
		}
		return tx.Delete(&role).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return nil
}

// AssignMenus 分配菜单权限与数据范围（整体替换）。
func (s *RoleService) AssignMenus(id string, req *dto.RoleAssignReq) *errs.Error {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if role.Code == model.SuperAdminCode {
		return errs.ErrBuiltin.WithMsg("内置超级管理员角色不可修改")
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&role).Update("data_scope", req.DataScope).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, id, req.MenuIDs)
	})
	if err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return nil
}

// replaceRoleMenus 事务内整体替换角色菜单关联。
func replaceRoleMenus(tx *gorm.DB, roleID string, menuIDs []string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.SysRoleMenu{}).Error; err != nil {
		return err
	}
	for _, mid := range uniqueStrings(menuIDs) {
		if err := tx.Create(&model.SysRoleMenu{RoleID: roleID, MenuID: mid}).Error; err != nil {
			return err
		}
	}
	return nil
}
