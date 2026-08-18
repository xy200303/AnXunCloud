package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
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

// guardRole 角色归属校验（P3 多租户）：
// 内置角色（tenant_id 空）全平台共享、只读不可改（仅超管可维护）；
// 租户自建角色仅本租户管理员与超管可改，跨租户一律按不存在处理。
func guardRole(role *model.SysRole, op *middleware.Identity, write bool) *errs.Error {
	if role.TenantID == nil {
		if write && !op.SuperAdmin {
			return errs.ErrNoPerm.WithMsg("内置角色全平台共享，只读不可改")
		}
		return nil
	}
	if op.SuperAdmin || *role.TenantID == op.TenantID {
		return nil
	}
	return errs.ErrNotFound
}

// shouldFilterPlatform 平台级菜单（菜单行 is_platform=true，平台管理目录整棵子树）仅 super_admin 持有
// （《管理后台信息架构与菜单归位方案》§1：平台管理目录仅超管可见可授权）。
// super_admin 的菜单绑定由 seed 维护、接口禁止改动，因此任何经接口的分配一律剔除平台级菜单——
// 既防租户管理员越权，也防超管误授租户角色/内置共享角色导致 casbin 接口级越权。
func shouldFilterPlatform(roleCode string) bool {
	return roleCode != model.SuperAdminCode
}

// filterPlatformMenus 分配菜单时按菜单行 is_platform 列剔除平台级菜单（判定规则见 shouldFilterPlatform）。
func (s *RoleService) filterPlatformMenus(menuIDs []string, roleCode string) []string {
	if len(menuIDs) == 0 || !shouldFilterPlatform(roleCode) {
		return menuIDs
	}
	var menus []model.SysMenu
	s.db.Select("id", "is_platform").Where("id IN ?", menuIDs).Find(&menus)
	allowed := map[string]bool{}
	for _, m := range menus {
		if !m.IsPlatform {
			allowed[m.ID] = true
		}
	}
	out := make([]string, 0, len(menuIDs))
	for _, id := range menuIDs {
		if allowed[id] {
			out = append(out, id)
		}
	}
	return out
}

// List 角色分页列表（user_count 统计引用该角色的用户数）。
// 租户上下文（菜单归位方案 §2）：内置角色（tenant_id 空）+ 上下文租户自建角色；
// tenantID 由 controller 经 middleware.EffectiveTenantID 解析（非超管=本人租户，超管=上下文租户）。
func (s *RoleService) List(q *dto.RoleListQuery, op *middleware.Identity, tenantID string) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.SysRole{}).Where("tenant_id IS NULL OR tenant_id = ?", tenantID)
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
// 租户归属（菜单归位方案 §2）：新建角色一律归属上下文租户（tenantID 由 EffectiveTenantID 解析，不为空）；
// 平台级共享角色（tenant_id 空）仅由 seed/迁移维护，不再经接口创建。
// code 在目标作用域内唯一（含内置角色占用，防止与程序内引用编码冲突）。
func (s *RoleService) Create(req *dto.RoleSaveReq, op *middleware.Identity, tenantID string) (string, *errs.Error) {
	if req.Code == "" {
		return "", errs.ErrParam.WithMsg("code 为必填项")
	}
	var count int64
	// 同租户或内置角色已占用该 code 均视为冲突
	s.db.Model(&model.SysRole{}).Where("code = ?", req.Code).
		Where("tenant_id IS NULL OR tenant_id = ?", tenantID).Count(&count)
	if count > 0 {
		return "", errs.ErrRoleCodeExists
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = model.StatusStr(*req.Status)
	}
	role := model.SysRole{
		TenantID:  &tenantID,
		Code:      req.Code,
		Name:      req.Name,
		DataScope: req.DataScope,
		Remark:    req.Remark,
		Status:    status,
	}
	menuIDs := s.filterPlatformMenus(req.MenuIDs, req.Code)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, role.ID, menuIDs)
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return role.ID, nil
}

// Detail 角色详情（含已分配菜单 id；内置角色全员可读，租户角色限本租户）。
func (s *RoleService) Detail(id string, op *middleware.Identity) (*dto.RoleDetail, *errs.Error) {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := guardRole(&role, op, false); be != nil {
		return nil, be
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

// Update 修改角色（code 不可改；超管的 data_scope 固定为 all；内置角色仅超管可维护，租户角色限本租户）。
func (s *RoleService) Update(id string, req *dto.RoleSaveReq, op *middleware.Identity) *errs.Error {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := guardRole(&role, op, true); be != nil {
		return be
	}
	updates := map[string]any{"name": req.Name, "remark": req.Remark}
	if role.Code == model.SuperAdminCode {
		updates["data_scope"] = model.ScopeAll
	} else {
		updates["data_scope"] = req.DataScope
	}
	if req.Status != nil {
		// 内置角色（尤其 super_admin）禁停用：停用后所有持有账号即时失去权限，可能导致超管集体锁死
		if role.IsBuiltin && model.StatusStr(*req.Status) == model.StatusDisabled {
			return errs.ErrParam.WithMsg("内置角色不可停用")
		}
		updates["status"] = model.StatusStr(*req.Status)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&role).Updates(updates).Error; err != nil {
			return err
		}
		if req.MenuIDs != nil && role.Code != model.SuperAdminCode {
			return replaceRoleMenus(tx, role.ID, s.filterPlatformMenus(req.MenuIDs, role.Code))
		}
		return nil
	})
	if err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return nil
}

// Delete 删除角色（内置角色不可删；存在用户引用不可删；租户角色限本租户操作）。
func (s *RoleService) Delete(id string, op *middleware.Identity) *errs.Error {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := guardRole(&role, op, true); be != nil {
		return be
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

// AssignMenus 分配菜单权限与数据范围（整体替换；内置角色仅超管可维护，租户角色限本租户）。
func (s *RoleService) AssignMenus(id string, req *dto.RoleAssignReq, op *middleware.Identity) *errs.Error {
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := guardRole(&role, op, true); be != nil {
		return be
	}
	if role.Code == model.SuperAdminCode {
		return errs.ErrBuiltin.WithMsg("内置超级管理员角色不可修改")
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&role).Update("data_scope", req.DataScope).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, id, s.filterPlatformMenus(req.MenuIDs, role.Code))
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
