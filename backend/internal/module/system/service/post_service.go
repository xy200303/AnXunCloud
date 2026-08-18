// 岗位管理与岗位模板库业务逻辑（《管理后台信息架构与菜单归位方案》第三章，最终决策版）。
// 两者操作同一张 post_dict 表，仅作用域不同：
//   - 岗位管理（系统管理 /system/posts）：当前租户上下文（EffectiveTenantID）的行，tenant_id=上下文租户；
//   - 岗位模板库（平台管理 /platform/post-templates，仅超管）：tenant_id IS NULL 的平台模板行，
//     仅作开通租户时的初始拷贝源，不参与任何租户的实际业务。
package service

import (
	"regexp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// PostService 岗位管理/岗位模板库服务。
type PostService struct {
	db *gorm.DB
}

func NewPostService(db *gorm.DB) *PostService { return &PostService{db: db} }

// postCodeRe 岗位 code 格式：小写字母/数字/下划线，首字符字母或数字，2–64 位。
var postCodeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{1,63}$`)

// postScope 作用域过滤：tenantID 为 nil = 平台模板库（tenant_id IS NULL）。
func postScope(db *gorm.DB, tenantID *string) *gorm.DB {
	if tenantID == nil {
		return db.Where("tenant_id IS NULL")
	}
	return db.Where("tenant_id = ?", *tenantID)
}

// List 岗位列表（含业务线与绑定角色名称，按 line+sort 排序）。
func (s *PostService) List(tenantID *string) ([]dto.PostItem, *errs.Error) {
	var posts []model.PostDict
	if err := postScope(s.db, tenantID).Order("line ASC, sort ASC, created_at ASC, id ASC").Find(&posts).Error; err != nil {
		return nil, errs.ErrInternal
	}
	roleName := map[string]string{}
	{
		var roles []model.SysRole
		s.db.Select("id", "name").Find(&roles)
		for _, r := range roles {
			roleName[r.ID] = r.Name
		}
	}
	items := make([]dto.PostItem, 0, len(posts))
	for _, p := range posts {
		item := dto.PostItem{
			ID: p.ID, Code: p.Code, Name: p.Name,
			Line: p.Line, LineName: model.PostLineNames[p.Line],
			IsSupervisor: p.IsSupervisor, Sort: p.Sort,
			Status: model.StatusInt(p.Status), Remark: p.Remark,
			CreatedAt: timefmt.T(p.CreatedAt),
		}
		if p.RoleID != nil {
			item.RoleID = *p.RoleID
			item.RoleName = roleName[*p.RoleID]
		}
		items = append(items, item)
	}
	return items, nil
}

// validateRoleBinding 岗位绑角色校验（方案 §3）：
// 绑定的角色必须是内置共享角色（tenant_id NULL）或本租户角色；模板岗位只允许绑内置共享角色。
func (s *PostService) validateRoleBinding(tenantID *string, roleID string) *errs.Error {
	if roleID == "" {
		return nil
	}
	var role model.SysRole
	if err := s.db.First(&role, "id = ?", roleID).Error; err != nil {
		return errs.ErrParam.WithMsg("绑定角色不存在")
	}
	if tenantID == nil {
		if role.TenantID != nil {
			return errs.ErrParam.WithMsg("模板岗位只能绑定内置共享角色")
		}
		return nil
	}
	if role.TenantID != nil && *role.TenantID != *tenantID {
		return errs.ErrParam.WithMsg("绑定角色须为内置共享角色或本租户角色")
	}
	return nil
}

// Create 新增岗位（code 作用域内唯一，创建后不可改）。
func (s *PostService) Create(tenantID *string, req *dto.PostSaveReq) (string, *errs.Error) {
	if !postCodeRe.MatchString(req.Code) {
		return "", errs.ErrParam.WithMsg("code 须为 2–64 位小写字母/数字/下划线")
	}
	if !model.ValidPostLine(req.Line) {
		return "", errs.ErrParam.WithMsg("业务线取值非法：" + req.Line)
	}
	if be := s.validateRoleBinding(tenantID, req.RoleID); be != nil {
		return "", be
	}
	var count int64
	postScope(s.db.Model(&model.PostDict{}), tenantID).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return "", errs.ErrConflict.WithMsg("岗位 code「" + req.Code + "」已存在")
	}
	row := model.PostDict{
		TenantID:     tenantID,
		Code:         req.Code,
		Name:         req.Name,
		Line:         req.Line,
		IsSupervisor: req.IsSupervisor,
		Sort:         req.Sort,
		Status:       model.StatusEnabled,
		Remark:       req.Remark,
	}
	if req.RoleID != "" {
		row.RoleID = &req.RoleID
	}
	if req.Status != nil {
		row.Status = model.StatusStr(*req.Status)
	}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	// 岗位绑角色影响有效角色并集：重建 casbin g 规则（EnforceAny 走离线同步策略）
	authz.SyncAllQuiet(s.db)
	return row.ID, nil
}

// Update 修改岗位（code 不可改；role_id 变更即时反映到有效角色并集）。
func (s *PostService) Update(tenantID *string, id string, req *dto.PostSaveReq) *errs.Error {
	var row model.PostDict
	if err := postScope(s.db, tenantID).First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if req.Code != "" && req.Code != row.Code {
		return errs.ErrParam.WithMsg("岗位 code 创建后不可改")
	}
	if !model.ValidPostLine(req.Line) {
		return errs.ErrParam.WithMsg("业务线取值非法：" + req.Line)
	}
	if be := s.validateRoleBinding(tenantID, req.RoleID); be != nil {
		return be
	}
	var roleID *string
	if req.RoleID != "" {
		roleID = &req.RoleID
	}
	updates := map[string]any{
		"name": req.Name, "line": req.Line, "role_id": roleID,
		"is_supervisor": req.IsSupervisor, "sort": req.Sort, "remark": req.Remark,
	}
	if req.Status != nil {
		updates["status"] = model.StatusStr(*req.Status)
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db) // role_id/状态变更影响有效角色并集
	return nil
}

// Delete 删除岗位。引用校验（方案 §3）：无 project_staff.posts 引用、无 duty_binding.post_codes 引用，
// 被引用拒绝并提示。
func (s *PostService) Delete(tenantID *string, id string) *errs.Error {
	var row model.PostDict
	if err := postScope(s.db, tenantID).First(&row, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := s.checkPostReferenced(tenantID, row.Code); be != nil {
		return be
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db) // 岗位删除影响有效角色并集
	return nil
}

// checkPostReferenced 岗位引用校验（被编制/槽位绑定引用时返回 409 并提示引用来源）。
func (s *PostService) checkPostReferenced(tenantID *string, code string) *errs.Error {
	// 编制引用与项目级绑定只可能来自本租户的项目；模板行不参与租户业务，仅查平台默认绑定
	if tenantID != nil {
		var projectIDs []string
		s.db.Model(&model.Community{}).Where("tenant_id = ?", *tenantID).Pluck("id", &projectIDs)
		if len(projectIDs) > 0 {
			var staffs []model.ProjectStaff
			s.db.Select("posts").Where("project_id IN ?", projectIDs).Find(&staffs)
			for _, st := range staffs {
				for _, p := range st.Posts {
					if p == code {
						return errs.ErrConflict.WithMsg("岗位「" + code + "」仍被项目编制引用，请先在小区编制中移除")
					}
				}
			}
			var bindings []model.DutyBinding
			s.db.Select("post_codes").Where("project_id IN ?", projectIDs).Find(&bindings)
			for _, b := range bindings {
				for _, p := range b.PostCodes {
					if p == code {
						return errs.ErrConflict.WithMsg("岗位「" + code + "」仍被项目职责绑定引用，请先调整绑定")
					}
				}
			}
		}
		// 租户级默认绑定
		var tenantBindings []model.DutyBinding
		s.db.Select("post_codes").Where("project_id IS NULL AND tenant_id = ?", *tenantID).Find(&tenantBindings)
		for _, b := range tenantBindings {
			for _, p := range b.PostCodes {
				if p == code {
					return errs.ErrConflict.WithMsg("岗位「" + code + "」仍被租户默认职责绑定引用，请先调整绑定")
				}
			}
		}
		return nil
	}
	// 平台模板：仅平台默认槽位绑定（tenant_id 空 + project_id 空）可引用模板行
	var bindings []model.DutyBinding
	s.db.Select("post_codes").Where("project_id IS NULL AND tenant_id IS NULL").Find(&bindings)
	for _, b := range bindings {
		for _, p := range b.PostCodes {
			if p == code {
				return errs.ErrConflict.WithMsg("模板岗位「" + code + "」仍被平台默认职责绑定引用，请先调整绑定")
			}
		}
	}
	return nil
}

// scopePostByCode 作用域内岗位 code → 岗位（含停用；绑定保存只校验存在性，与项目级口径一致）。
func (s *PostService) scopePostByCode(tenantID *string) (map[string]model.PostDict, *errs.Error) {
	var posts []model.PostDict
	if err := postScope(s.db, tenantID).Find(&posts).Error; err != nil {
		return nil, errs.ErrInternal
	}
	byCode := make(map[string]model.PostDict, len(posts))
	for _, p := range posts {
		byCode[p.Code] = p
	}
	return byCode, nil
}

// ListDutyBindings 职责槽位默认绑定视图（租户级：各行解析结果与来源 tenant/platform；平台模板库：仅 platform）。
func (s *PostService) ListDutyBindings(tenantID *string) ([]gin.H, *errs.Error) {
	postByCode, be := s.scopePostByCode(tenantID)
	if be != nil {
		return nil, be
	}
	own := map[string]types.StringArray{}
	var rows []model.DutyBinding
	if err := postScope(s.db, tenantID).Where("project_id IS NULL").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	for _, b := range rows {
		own[b.Slot] = b.PostCodes
	}
	// 租户级缺省回落平台默认（来源 platform）
	platform := map[string]types.StringArray{}
	if tenantID != nil {
		var prows []model.DutyBinding
		if err := s.db.Where("project_id IS NULL AND tenant_id IS NULL").Find(&prows).Error; err != nil {
			return nil, errs.ErrInternal
		}
		for _, b := range prows {
			platform[b.Slot] = b.PostCodes
		}
	}
	items := make([]gin.H, 0, len(model.DutySlots))
	for _, ds := range model.DutySlots {
		codes, source := types.StringArray{}, "platform"
		if oc, ok := own[ds.Slot]; ok {
			codes = oc
			if tenantID != nil {
				source = "tenant"
			}
		} else if pc, ok := platform[ds.Slot]; ok {
			codes = pc
		}
		if codes == nil {
			codes = types.StringArray{}
		}
		postNames := make([]string, 0, len(codes))
		for _, code := range codes {
			if p, ok := postByCode[code]; ok {
				postNames = append(postNames, p.Name)
			}
		}
		items = append(items, gin.H{
			"slot": ds.Slot, "name": ds.Name,
			"post_codes": codes, "post_names": postNames, "source": source,
		})
	}
	return items, nil
}

// SaveDutyBindings 保存租户级/平台级槽位默认绑定（逐槽位 upsert，写 project_id NULL + tenant_id=作用域 的行；
// post_codes 空 = 该环节跳过/降级）。
func (s *PostService) SaveDutyBindings(tenantID *string, req *dto.PostDutyBindingsSaveReq) *errs.Error {
	postByCode, be := s.scopePostByCode(tenantID)
	if be != nil {
		return be
	}
	known := make(map[string]bool, len(model.DutySlots))
	for _, ds := range model.DutySlots {
		known[ds.Slot] = true
	}
	seen := map[string]bool{}
	for _, b := range req.Bindings {
		if !known[b.Slot] {
			return errs.ErrParam.WithMsg("未知职责槽位「" + b.Slot + "」")
		}
		if seen[b.Slot] {
			return errs.ErrParam.WithMsg("职责槽位「" + b.Slot + "」重复提交")
		}
		seen[b.Slot] = true
		for _, code := range uniqueStrings(b.PostCodes) {
			if _, ok := postByCode[code]; !ok {
				return errs.ErrParam.WithMsg("岗位「" + code + "」不存在")
			}
		}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, b := range req.Bindings {
			codes := types.StringArray(uniqueStrings(b.PostCodes))
			if codes == nil {
				codes = types.StringArray{}
			}
			var row model.DutyBinding
			q := postScope(tx, tenantID).Where("project_id IS NULL AND slot = ?", b.Slot)
			if err := q.First(&row).Error; err == nil {
				if err := tx.Model(&row).Update("post_codes", codes).Error; err != nil {
					return err
				}
				continue
			}
			row = model.DutyBinding{TenantID: tenantID, Slot: b.Slot, PostCodes: codes}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// CopyPostTemplatesToTenant 开通租户复制（方案 §3：模板库只读于开通那一刻，此后租户自管）：
// seed 初始化默认租户时同样调用（seed 与开通租户走同一复制逻辑）。
// 平台模板岗位整份复制为新租户行（新 id，同 code/line/is_supervisor/sort/status/remark；
// role_id 保留——模板只绑内置共享角色，可直接引用）；平台默认槽位绑定复制为租户级默认行
// （project_id NULL、tenant_id=新租户、同 slot/post_codes）。迁移 00025 对存量租户做同样回填。
func CopyPostTemplatesToTenant(tx *gorm.DB, tenantID string) error {
	var templates []model.PostDict
	if err := tx.Where("tenant_id IS NULL").Find(&templates).Error; err != nil {
		return err
	}
	for _, tpl := range templates {
		row := model.PostDict{
			TenantID: &tenantID, Code: tpl.Code, Name: tpl.Name, Line: tpl.Line,
			IsSupervisor: tpl.IsSupervisor, RoleID: tpl.RoleID, Sort: tpl.Sort,
			Status: tpl.Status, Remark: tpl.Remark,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	var bindings []model.DutyBinding
	if err := tx.Where("tenant_id IS NULL AND project_id IS NULL").Find(&bindings).Error; err != nil {
		return err
	}
	for _, b := range bindings {
		row := model.DutyBinding{TenantID: &tenantID, Slot: b.Slot, PostCodes: b.PostCodes}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
