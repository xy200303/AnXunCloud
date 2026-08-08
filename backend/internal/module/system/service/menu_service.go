package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
)

// MenuService 菜单管理服务。
type MenuService struct {
	db *gorm.DB
}

func NewMenuService(db *gorm.DB) *MenuService { return &MenuService{db: db} }

// parentPtr 空字符串根节点转 NULL。
func parentPtr(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}

// Tree 菜单树查询；带筛选时保留命中节点的祖先以保证树完整。
func (s *MenuService) Tree(q *dto.MenuListQuery) ([]dto.MenuNode, *errs.Error) {
	db := s.db.Model(&model.SysMenu{})
	filtered := false
	if q.Title != "" {
		db = db.Where("title LIKE ?", "%"+q.Title+"%")
		filtered = true
	}
	if q.Status != nil {
		db = db.Where("status = ?", model.StatusStr(*q.Status))
		filtered = true
	}
	var menus []model.SysMenu
	if err := db.Order("sort ASC, id ASC").Find(&menus).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if filtered {
		menus = s.withAncestors(menus)
	}
	return buildMenuTree(menus, ""), nil
}

// withAncestors 补齐命中节点的祖先。
func (s *MenuService) withAncestors(menus []model.SysMenu) []model.SysMenu {
	seen := map[string]bool{}
	for _, m := range menus {
		seen[m.ID] = true
	}
	var all []model.SysMenu
	s.db.Select("id", "parent_id").Find(&all)
	parent := map[string]string{}
	for _, m := range all {
		parent[m.ID] = m.ParentIDStr()
	}
	var missing []string
	for _, m := range menus {
		for pid := m.ParentIDStr(); pid != "" && !seen[pid]; pid = parent[pid] {
			seen[pid] = true
			missing = append(missing, pid)
		}
	}
	if len(missing) > 0 {
		var ancestors []model.SysMenu
		s.db.Where("id IN ?", missing).Find(&ancestors)
		menus = append(menus, ancestors...)
	}
	return menus
}

// buildMenuTree 递归构建菜单树（parentID 空串为根）。
func buildMenuTree(menus []model.SysMenu, parentID string) []dto.MenuNode {
	nodes := []dto.MenuNode{}
	for _, m := range menus {
		if m.ParentIDStr() != parentID {
			continue
		}
		visible := 0
		if m.Visible {
			visible = 1
		}
		nodes = append(nodes, dto.MenuNode{
			ID:       m.ID,
			ParentID: m.ParentIDStr(),
			Title:    m.Title,
			Path:     m.Path,
			Icon:     m.Icon,
			Type:     m.Type,
			Perms:    m.Perms,
			Sort:     m.Sort,
			Visible:  visible,
			Status:   model.StatusInt(m.Status),
			Children: buildMenuTree(menus, m.ID),
		})
	}
	return nodes
}

// Create 新增菜单。
func (s *MenuService) Create(req *dto.MenuSaveReq) (string, *errs.Error) {
	if be := s.validateSave(req, ""); be != nil {
		return "", be
	}
	menu := model.SysMenu{
		ParentID: parentPtr(req.ParentID),
		Title:    req.Title,
		Path:     req.Path,
		Icon:     req.Icon,
		Type:     req.Type,
		Perms:    req.Perms,
		Sort:     req.Sort,
		Visible:  true,
		Status:   model.StatusEnabled,
	}
	if req.Type == model.MenuTypeButton {
		menu.Visible = false
	}
	if req.Visible != nil {
		menu.Visible = *req.Visible == 1
	}
	if req.Status != nil {
		menu.Status = model.StatusStr(*req.Status)
	}
	if err := s.db.Create(&menu).Error; err != nil {
		return "", errs.ErrInternal
	}
	return menu.ID, nil
}

// Detail 菜单详情。
func (s *MenuService) Detail(id string) (*dto.MenuNode, *errs.Error) {
	var m model.SysMenu
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	visible := 0
	if m.Visible {
		visible = 1
	}
	return &dto.MenuNode{
		ID: m.ID, ParentID: m.ParentIDStr(), Title: m.Title, Path: m.Path, Icon: m.Icon,
		Type: m.Type, Perms: m.Perms, Sort: m.Sort, Visible: visible,
		Status: model.StatusInt(m.Status), Children: []dto.MenuNode{},
	}, nil
}

// Update 修改菜单（不允许挂到自身或子孙节点下）。
func (s *MenuService) Update(id string, req *dto.MenuSaveReq) *errs.Error {
	var m model.SysMenu
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := s.validateSave(req, id); be != nil {
		return be
	}
	if req.ParentID != "" && s.isDescendant(id, req.ParentID) {
		return errs.ErrParam.WithMsg("parent_id 不允许挂到自身或子孙节点下")
	}
	updates := map[string]any{
		"parent_id": parentPtr(req.ParentID),
		"title":     req.Title,
		"path":      req.Path,
		"icon":      req.Icon,
		"type":      req.Type,
		"perms":     req.Perms,
		"sort":      req.Sort,
	}
	if req.Visible != nil {
		updates["visible"] = *req.Visible == 1
	}
	if req.Status != nil {
		updates["status"] = model.StatusStr(*req.Status)
	}
	if err := s.db.Model(&m).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 删除菜单（存在子菜单或内置菜单不可删）。
func (s *MenuService) Delete(id string) *errs.Error {
	var m model.SysMenu
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if m.IsBuiltin {
		return errs.ErrBuiltin
	}
	var childCount int64
	s.db.Model(&model.SysMenu{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return errs.ErrParam.WithMsg("存在子菜单，不可删除")
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id = ?", id).Delete(&model.SysRoleMenu{}).Error; err != nil {
			return err
		}
		return tx.Delete(&m).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// validateSave 校验父菜单存在性及 dir/menu 的 path 必填。
func (s *MenuService) validateSave(req *dto.MenuSaveReq, selfID string) *errs.Error {
	if req.ParentID != "" {
		if req.ParentID == selfID {
			return errs.ErrParam.WithMsg("parent_id 不允许为自身")
		}
		var count int64
		s.db.Model(&model.SysMenu{}).Where("id = ?", req.ParentID).Count(&count)
		if count == 0 {
			return errs.ErrParam.WithMsg("父菜单不存在")
		}
	}
	if req.Type != model.MenuTypeButton && req.Path == "" {
		return errs.ErrParam.WithMsg("dir/menu 类型 path 必填")
	}
	return nil
}

// isDescendant 判断 candidate 是否为 ancestorID 的子孙节点。
func (s *MenuService) isDescendant(ancestorID, candidate string) bool {
	var all []model.SysMenu
	s.db.Select("id", "parent_id").Find(&all)
	parent := map[string]string{}
	for _, m := range all {
		parent[m.ID] = m.ParentIDStr()
	}
	for cur := candidate; cur != ""; cur = parent[cur] {
		if cur == ancestorID {
			return true
		}
	}
	return false
}
