package middleware

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
)

// 有效角色实时并集（《管理后台信息架构与菜单归位方案》第三章，最终决策）：
// 用户实际持有角色 = 用户管理手动分配的角色（sys_user.role_ids）∪ 其全部在职编制岗位绑定的角色
// （post_dict.role_id，岗位按项目所属租户的行匹配）。实时计算、不落库：配岗即得权限、离岗即失权限。
// 功能权限仍只由角色定义，岗位只是角色的一个来源；数据范围上限按并集"取最宽"走现有规则。
// 注意：casbin 接口鉴权走离线同步的 g 规则，authz.SyncAll 重建时用同一套并集逻辑展开写入。

// EffectiveRoleIDs 用户有效角色 ID 集合（role_ids ∪ 在职岗位绑定角色，去重，保持 role_ids 在前）。
func EffectiveRoleIDs(db *gorm.DB, user *model.SysUser) ([]string, error) {
	seen := map[string]bool{}
	ids := make([]string, 0, len(user.RoleIDs))
	for _, rid := range user.RoleIDs {
		if !seen[rid] {
			seen[rid] = true
			ids = append(ids, rid)
		}
	}
	var staffs []model.ProjectStaff
	if err := db.Select("project_id", "posts").
		Where("user_id = ? AND status = ?", user.ID, model.StatusEnabled).
		Find(&staffs).Error; err != nil {
		return nil, err
	}
	if len(staffs) == 0 {
		return ids, nil
	}
	// 编制项目 → 所属租户（岗位属于各自行租户，按 code+租户匹配 post_dict 行）
	projectIDs := make([]string, 0, len(staffs))
	for _, st := range staffs {
		projectIDs = append(projectIDs, st.ProjectID)
	}
	var communities []model.Community
	if err := db.Select("id", "tenant_id").Where("id IN ?", projectIDs).Find(&communities).Error; err != nil {
		return nil, err
	}
	tenantByProject := make(map[string]string, len(communities))
	tenantSeen := map[string]bool{}
	tenantIDs := make([]string, 0, len(communities))
	for _, cm := range communities {
		tenantByProject[cm.ID] = cm.TenantID
		if cm.TenantID != "" && !tenantSeen[cm.TenantID] {
			tenantSeen[cm.TenantID] = true
			tenantIDs = append(tenantIDs, cm.TenantID)
		}
	}
	if len(tenantIDs) == 0 {
		return ids, nil
	}
	// 这些租户下绑定了角色的岗位（role_id 非空才参与并集）
	var posts []model.PostDict
	if err := db.Select("tenant_id", "code", "role_id").
		Where("tenant_id IN ? AND role_id IS NOT NULL", tenantIDs).
		Find(&posts).Error; err != nil {
		return nil, err
	}
	roleByTenantCode := make(map[string]map[string]string, len(tenantIDs))
	for _, p := range posts {
		if p.TenantID == nil || p.RoleID == nil {
			continue
		}
		m := roleByTenantCode[*p.TenantID]
		if m == nil {
			m = map[string]string{}
			roleByTenantCode[*p.TenantID] = m
		}
		m[p.Code] = *p.RoleID
	}
	for _, st := range staffs {
		bindings := roleByTenantCode[tenantByProject[st.ProjectID]]
		if len(bindings) == 0 {
			continue
		}
		for _, code := range st.Posts {
			if rid, ok := bindings[code]; ok && !seen[rid] {
				seen[rid] = true
				ids = append(ids, rid)
			}
		}
	}
	return ids, nil
}
