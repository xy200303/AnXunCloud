package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/system/dto"
	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/excel"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

// importMaxRows 单次导入数据行上限（接口文档 §2.3.9）。
const importMaxRows = 500

// exportMaxRows 导出行数上限。
const exportMaxRows = 10000

// UserService 用户管理服务。killSessions 用于重置密码/停用/删除后踢下线。
type UserService struct {
	db           *gorm.DB
	killSessions func(ctx context.Context, userID string)
	store        *storage.Storage  // 签名图存储路径 → URL
	signAssets   *SignAssetService // v16 起签名走签章资产表
}

func NewUserService(db *gorm.DB, killSessions func(ctx context.Context, userID string), store *storage.Storage, signAssets *SignAssetService) *UserService {
	return &UserService{db: db, killSessions: killSessions, store: store, signAssets: signAssets}
}

// listQuery 组装列表/导出共用的查询条件。
// 租户上下文（菜单归位方案 §2）：只显示上下文租户数据，不做跨租户混合列表；
// tenantID 由 controller 经 middleware.EffectiveTenantID 解析（非超管=本人租户，超管=上下文租户）。
func (s *UserService) listQuery(q *dto.UserListQuery, op *middleware.Identity, tenantID string) *gorm.DB {
	db := s.db.Model(&model.SysUser{}).Where("tenant_id = ?", tenantID)
	if q.Username != "" {
		db = db.Where("username LIKE ? OR name LIKE ?", "%"+q.Username+"%", "%"+q.Username+"%")
	}
	if q.Phone != "" {
		db = db.Where("phone LIKE ?", "%"+q.Phone+"%")
	}
	if q.RoleID != "" {
		db = db.Where("role_ids @> ?::jsonb", fmt.Sprintf(`["%s"]`, q.RoleID))
	}
	if q.CommunityID != "" {
		// 按项目编制过滤：该项目编制内的用户
		db = db.Where("id IN (?)", s.db.Model(&model.ProjectStaff{}).
			Select("user_id").Where("project_id = ?", q.CommunityID))
	}
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", status)
	}
	return db
}

// List 用户分页列表（附带角色名与小区名）。
func (s *UserService) List(q *dto.UserListQuery, op *middleware.Identity, tenantID string) (*response.Page, *errs.Error) {
	db := s.listQuery(q, op, tenantID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var users []model.SysUser
	offset, limit := q.Normalize()
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]dto.UserItem, 0, len(users))
	for i := range users {
		items = append(items, s.toItem(&users[i]))
	}
	s.attachPostRoles(users, items)
	return &response.Page{List: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// attachPostRoles 批量填充「岗位带入角色」（在职编制岗位绑定角色，剔除已手动分配的角色；
// 与 middleware.EffectiveRoleIDs 同口径）。仅列表页调用：只读展示权限来源，导出不含此字段。
func (s *UserService) attachPostRoles(users []model.SysUser, items []dto.UserItem) {
	userIDs := make([]string, 0, len(users))
	manual := make(map[string]map[string]bool, len(users)) // userID → 已手动分配 roleID 集合
	for i := range users {
		userIDs = append(userIDs, users[i].ID)
		m := map[string]bool{}
		for _, rid := range users[i].RoleIDs {
			m[rid] = true
		}
		manual[users[i].ID] = m
	}
	var staffs []model.ProjectStaff
	if err := s.db.Select("user_id", "project_id", "posts").
		Where("user_id IN ? AND status = ?", userIDs, model.StatusEnabled).
		Find(&staffs).Error; err != nil || len(staffs) == 0 {
		return
	}
	projectIDs := make([]string, 0, len(staffs))
	for _, st := range staffs {
		projectIDs = append(projectIDs, st.ProjectID)
	}
	var communities []model.Community
	if err := s.db.Select("id", "tenant_id").Where("id IN ?", projectIDs).Find(&communities).Error; err != nil {
		return
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
		return
	}
	var posts []model.PostDict
	if err := s.db.Select("tenant_id", "code", "role_id").
		Where("tenant_id IN ? AND role_id IS NOT NULL", tenantIDs).
		Find(&posts).Error; err != nil {
		return
	}
	roleByTenantCode := make(map[string]map[string]string, len(tenantIDs))
	roleIDSeen := map[string]bool{}
	roleIDs := []string{}
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
		if !roleIDSeen[*p.RoleID] {
			roleIDSeen[*p.RoleID] = true
			roleIDs = append(roleIDs, *p.RoleID)
		}
	}
	roleItemByID := map[string]dto.RoleItem{}
	if len(roleIDs) > 0 {
		var roles []model.SysRole
		if err := s.db.Select("id", "code", "name").Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
			return
		}
		for _, r := range roles {
			roleItemByID[r.ID] = dto.RoleItem{ID: r.ID, Code: r.Code, Name: r.Name}
		}
	}
	idxByUser := make(map[string]int, len(items))
	for i := range items {
		idxByUser[items[i].ID] = i
	}
	derived := make(map[string]map[string]bool, len(users)) // userID → 已加入的带入 roleID（去重）
	for _, st := range staffs {
		idx, ok := idxByUser[st.UserID]
		if !ok {
			continue
		}
		bindings := roleByTenantCode[tenantByProject[st.ProjectID]]
		if len(bindings) == 0 {
			continue
		}
		for _, code := range st.Posts {
			rid, ok := bindings[code]
			if !ok || manual[st.UserID][rid] {
				continue
			}
			if derived[st.UserID] == nil {
				derived[st.UserID] = map[string]bool{}
			}
			if derived[st.UserID][rid] {
				continue
			}
			if ri, ok := roleItemByID[rid]; ok {
				derived[st.UserID][rid] = true
				items[idx].PostRoles = append(items[idx].PostRoles, ri)
			}
		}
	}
}

// toItem 转换为列表视图（角色名反查；所属项目名取 project_staff 在职编制，导出用）。
func (s *UserService) toItem(u *model.SysUser) dto.UserItem {
	item := dto.UserItem{
		ID:             u.ID,
		Username:       u.Username,
		Name:           u.Name,
		Phone:          u.Phone,
		Avatar:         u.Avatar,
		Roles:          []dto.RoleItem{},
		CommunityNames: []string{},
		Status:         model.StatusInt(u.Status),
		IsBuiltin:      u.IsBuiltin,
		LastLoginAt:    timefmt.TP(u.LastLoginAt),
		CreatedAt:      timefmt.T(u.CreatedAt),
	}
	if u.Openid != nil {
		item.Openid = *u.Openid
	}
	if len(u.RoleIDs) > 0 {
		var roles []model.SysRole
		s.db.Select("id", "code", "name").Where("id IN ?", []string(u.RoleIDs)).Find(&roles)
		for _, r := range roles {
			item.Roles = append(item.Roles, dto.RoleItem{ID: r.ID, Code: r.Code, Name: r.Name})
		}
	}
	_, item.CommunityNames = s.userCommunities(u.ID)
	return item
}

// userCommunities 用户在职编制覆盖的项目 ID 与名称（按 id 升序）。
func (s *UserService) userCommunities(userID string) ([]string, []string) {
	var ids []string
	s.db.Model(&model.ProjectStaff{}).
		Where("user_id = ? AND status = ?", userID, model.StatusEnabled).
		Order("project_id ASC").Pluck("project_id", &ids)
	if len(ids) == 0 {
		return []string{}, []string{}
	}
	var names []string
	s.db.Model(&model.Community{}).Where("id IN ?", ids).Order("id ASC").Pluck("name", &names)
	return ids, names
}

// checkTenant 租户归属校验：非超管操作跨租户用户一律按不存在处理（不暴露跨租户数据存在性）。
func checkTenant(userTenantID string, op *middleware.Identity) *errs.Error {
	if op.SuperAdmin || userTenantID == op.TenantID {
		return nil
	}
	return errs.ErrNotFound
}

// Create 新增用户。super_admin 角色仅超管本人可分配（防权限提升）。
// 租户归属：新建用户归属上下文租户（tenantID 由 EffectiveTenantID 解析并校验存在性，不为空）。
func (s *UserService) Create(req *dto.UserCreateReq, op *middleware.Identity, tenantID string) (string, *errs.Error) {
	if !password.ValidUsername(req.Username) {
		return "", errs.ErrParam.WithMsg("username 须为 4–32 位字母数字下划线")
	}
	if !password.ValidPassword(req.Password) {
		return "", errs.ErrParam.WithMsg("password 须为 8–32 位且含字母与数字")
	}
	if !password.ValidPhone(req.Phone) {
		return "", errs.ErrParam.WithMsg("phone 手机号格式错误")
	}
	var count int64
	// 用户名租户内唯一（P3）
	s.db.Model(&model.SysUser{}).Where("tenant_id = ? AND username = ?", tenantID, req.Username).Count(&count)
	if count > 0 {
		return "", errs.ErrUsernameExists
	}
	if be := s.checkRoles(req.RoleIDs, op); be != nil {
		return "", be
	}
	if be := s.guardSuperRoleAssign(req.RoleIDs, op.SuperAdmin); be != nil {
		return "", be
	}
	hash, err := password.Hash(req.Password)
	if err != nil {
		return "", errs.ErrInternal
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = model.StatusStr(*req.Status)
	}
	user := model.SysUser{
		TenantID: tenantID,
		Username: req.Username,
		Password: hash,
		Name:     req.Name,
		Phone:    req.Phone,
		RoleIDs:  req.RoleIDs,
		Status:   status,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return "", errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return user.ID, nil
}

// Detail 用户详情（非超管仅可看本租户用户）。
func (s *UserService) Detail(id string, op *middleware.Identity) (*dto.UserDetail, *errs.Error) {
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := checkTenant(u.TenantID, op); be != nil {
		return nil, be
	}
	d := &dto.UserDetail{
		ID:          u.ID,
		Username:    u.Username,
		Name:        u.Name,
		Phone:       u.Phone,
		Avatar:      u.Avatar,
		RoleIDs:     u.RoleIDs,
		Status:      model.StatusInt(u.Status),
		IsBuiltin:   u.IsBuiltin,
		LastLoginAt: timefmt.TP(u.LastLoginAt),
		CreatedAt:   timefmt.T(u.CreatedAt),
		UpdatedAt:   timefmt.T(u.UpdatedAt),
	}
	// 签名取当前 active 签章资产（sign_asset 表）
	if s.signAssets != nil {
		fileID, _ := s.signAssets.ActiveSignature(id)
		d.SignatureFileID = fileID
		if fileID != "" && s.store != nil {
			if f, err := uploadfile.ByID(s.db, fileID); err == nil {
				d.SignatureURL = s.store.URL(f.StorageKey)
			}
		}
	}
	if u.Openid != nil {
		d.Openid = *u.Openid
	}
	return d, nil
}

// Update 修改用户（username 不可改，改密码走重置接口；内置账号不可移除 super_admin 角色）。
// super_admin 角色仅超管本人可分配（防权限提升）；非超管仅可改本租户用户。
func (s *UserService) Update(id string, req *dto.UserUpdateReq, op *middleware.Identity) *errs.Error {
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := checkTenant(u.TenantID, op); be != nil {
		return be
	}
	if u.IsBuiltin {
		var superRole model.SysRole
		if err := s.db.Select("id").Where("code = ?", model.SuperAdminCode).First(&superRole).Error; err != nil {
			return errs.ErrInternal
		}
		if !types.IDArray(req.RoleIDs).Contains(superRole.ID) {
			return errs.ErrBuiltinAccount.WithMsg("内置账号不可移除超级管理员角色")
		}
	}
	if !password.ValidPhone(req.Phone) {
		return errs.ErrParam.WithMsg("phone 手机号格式错误")
	}
	if be := s.checkRoles(req.RoleIDs, op); be != nil {
		return be
	}
	if be := s.guardSuperRoleAssign(req.RoleIDs, op.SuperAdmin); be != nil {
		return be
	}
	updates := map[string]any{
		"name":     req.Name,
		"phone":    req.Phone,
		"role_ids": types.IDArray(req.RoleIDs),
	}
	if req.Status != nil {
		updates["status"] = model.StatusStr(*req.Status)
	}
	if err := s.db.Model(&u).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	authz.SyncAllQuiet(s.db)
	return nil
}

// ResetPassword 管理员重置密码，重置后该用户全部会话失效。
// 目标为内置账号或持有 super_admin 角色时，仅超管本人可操作（防接管超管账号）；非超管仅可重置本租户用户。
func (s *UserService) ResetPassword(ctx context.Context, id string, newPassword string, op *middleware.Identity) *errs.Error {
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := checkTenant(u.TenantID, op); be != nil {
		return be
	}
	if !op.SuperAdmin {
		if u.IsBuiltin {
			return errs.ErrNoPerm.WithMsg("内置超管账号仅超级管理员本人可重置密码")
		}
		has, be := s.hasSuperRole(u.RoleIDs)
		if be != nil {
			return be
		}
		if has {
			return errs.ErrNoPerm.WithMsg("目标为超级管理员，仅超管本人可重置其密码")
		}
	}
	if !password.ValidPassword(newPassword) {
		return errs.ErrParam.WithMsg("new_password 须为 8–32 位且含字母与数字")
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return errs.ErrInternal
	}
	if err := s.db.Model(&u).Updates(map[string]any{
		"password":             hash,
		"must_change_password": false,
	}).Error; err != nil {
		return errs.ErrInternal
	}
	s.killSessions(ctx, id)
	return nil
}

// SetStatus 启用/停用（停用即会话失效；不能操作当前登录账号；内置账号不可停用；非超管仅可操作本租户用户）。
func (s *UserService) SetStatus(ctx context.Context, id string, status int, op *middleware.Identity) *errs.Error {
	if id == op.UserID {
		return errs.ErrSelfOperation
	}
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := checkTenant(u.TenantID, op); be != nil {
		return be
	}
	if u.IsBuiltin && status == 0 {
		return errs.ErrBuiltinAccount.WithMsg("内置账号不可停用")
	}
	if err := s.db.Model(&u).Update("status", model.StatusStr(status)).Error; err != nil {
		return errs.ErrInternal
	}
	if status == 0 {
		s.killSessions(ctx, id)
	}
	authz.SyncAllQuiet(s.db)
	return nil
}

// Delete 软删除（不能删除当前登录账号；内置账号不可删除；非超管仅可删本租户用户）。
func (s *UserService) Delete(ctx context.Context, id string, op *middleware.Identity) *errs.Error {
	if id == op.UserID {
		return errs.ErrSelfOperation
	}
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := checkTenant(u.TenantID, op); be != nil {
		return be
	}
	if u.IsBuiltin {
		return errs.ErrBuiltinAccount.WithMsg("内置账号不可删除")
	}
	res := s.db.Delete(&model.SysUser{}, "id = ?", id)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	s.killSessions(ctx, id)
	authz.SyncAllQuiet(s.db)
	return nil
}

// checkRoles 校验角色存在。
// 租户约束（P3）：非超管只能分配内置角色（tenant_id 空）或本租户自建角色。
func (s *UserService) checkRoles(roleIDs []string, op *middleware.Identity) *errs.Error {
	var roles []model.SysRole
	if err := s.db.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return errs.ErrInternal
	}
	if len(roles) != len(uniqueStrings(roleIDs)) {
		return errs.ErrParam.WithMsg("role_ids 中存在无效角色")
	}
	if !op.SuperAdmin {
		for _, r := range roles {
			if r.TenantID != nil && *r.TenantID != op.TenantID {
				return errs.ErrParam.WithMsg("role_ids 中存在非本租户角色")
			}
		}
	}
	return nil
}

// hasSuperRole 角色集合是否包含 super_admin。
func (s *UserService) hasSuperRole(roleIDs []string) (bool, *errs.Error) {
	if len(roleIDs) == 0 {
		return false, nil
	}
	var n int64
	if err := s.db.Model(&model.SysRole{}).
		Where("id IN ? AND code = ?", roleIDs, model.SuperAdminCode).Count(&n).Error; err != nil {
		return false, errs.ErrInternal
	}
	return n > 0, nil
}

// guardSuperRoleAssign 非超管操作者分配 super_admin 角色时拒绝（垂直越权防护）。
func (s *UserService) guardSuperRoleAssign(roleIDs []string, operatorSuper bool) *errs.Error {
	if operatorSuper {
		return nil
	}
	has, be := s.hasSuperRole(roleIDs)
	if be != nil {
		return be
	}
	if has {
		return errs.ErrNoPerm.WithMsg("仅超级管理员可分配超级管理员角色")
	}
	return nil
}

// Export 按筛选导出全部用户（上限 10000 行；租户过滤口径与 List 一致）。
func (s *UserService) Export(q *dto.UserListQuery, op *middleware.Identity, tenantID string) ([]excel.UserExportRow, *errs.Error) {
	var users []model.SysUser
	if err := s.listQuery(q, op, tenantID).Order("id ASC").Limit(exportMaxRows + 1).Find(&users).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(users) > exportMaxRows {
		return nil, errs.ErrParam.WithMsg("导出数据超过 10000 行，请缩小筛选范围")
	}
	rows := make([]excel.UserExportRow, 0, len(users))
	for i := range users {
		item := s.toItem(&users[i])
		roleNames := make([]string, 0, len(item.Roles))
		for _, r := range item.Roles {
			roleNames = append(roleNames, r.Name)
		}
		statusText := "启用"
		if item.Status == 0 {
			statusText = "停用"
		}
		rows = append(rows, excel.UserExportRow{
			Username:    item.Username,
			Name:        item.Name,
			Phone:       item.Phone,
			Roles:       strings.Join(roleNames, ","),
			Communities: strings.Join(item.CommunityNames, ","),
			Status:      statusText,
			LastLoginAt: item.LastLoginAt,
			CreatedAt:   item.CreatedAt,
		})
	}
	return rows, nil
}

// Import 逐行校验导入用户（跳过失败行，成功行落库）。super_admin 角色仅超管本人可导入。
// 租户归属（菜单归位方案 §2）：导入用户归入上下文租户（tenantID 由 EffectiveTenantID 解析）。
// ImportTemplate 生成用户导入模板（角色/小区软下拉数据源按上下文租户生成，口径与 Import 一致；
// 非超管操作者的角色列表不含超级管理员——与 Import 的导入校验口径一致）。
func (s *UserService) ImportTemplate(c *gin.Context) (*excelize.File, *errs.Error) {
	tid, be := middleware.EffectiveTenantID(c, s.db)
	if be != nil {
		return nil, be
	}
	roleNames := []string{}
	{
		var roles []model.SysRole
		q := s.db.Where("tenant_id IS NULL OR tenant_id = ?", tid).Order("created_at ASC")
		if identity := middleware.CurrentIdentity(c); identity == nil || !identity.SuperAdmin {
			q = q.Where("code <> ?", model.SuperAdminCode)
		}
		if err := q.Find(&roles).Error; err != nil {
			return nil, errs.ErrInternal
		}
		for _, r := range roles {
			roleNames = append(roleNames, r.Name)
		}
	}
	commNames := []string{}
	if err := s.db.Model(&model.Community{}).Where("tenant_id = ?", tid).
		Order("name ASC").Pluck("name", &commNames).Error; err != nil {
		return nil, errs.ErrInternal
	}
	f, err := excel.UserImportTemplate(roleNames, commNames)
	if err != nil {
		return nil, errs.ErrInternal
	}
	return f, nil
}

func (s *UserService) Import(r io.Reader, op *middleware.Identity, tenantID string) (*dto.ImportResult, string, *errs.Error) {
	rows, err := excel.ParseUserImport(r)
	if err != nil {
		return nil, "", errs.ErrImportFileType
	}
	// 过滤整行为空的行
	dataRows := make([][]string, 0, len(rows))
	rowNums := make([]int, 0, len(rows))
	for i, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		dataRows = append(dataRows, row)
		rowNums = append(rowNums, i+3) // Excel 实际行号（1 表头 + 1 示例）
	}
	if len(dataRows) == 0 {
		return nil, "", errs.ErrImportEmpty
	}
	if len(dataRows) > importMaxRows {
		return nil, "", errs.ErrImportTooMany
	}

	// 预载角色（内置 + 上下文租户自建）与上下文租户小区名称映射、已有用户名集合
	roleByName := map[string]model.SysRole{}
	{
		var roles []model.SysRole
		s.db.Where("tenant_id IS NULL OR tenant_id = ?", tenantID).Find(&roles)
		for _, r := range roles {
			roleByName[r.Name] = r
		}
	}
	commByName := map[string]string{}
	{
		var comms []model.Community
		s.db.Select("id", "name").Where("tenant_id = ?", tenantID).Find(&comms)
		for _, cm := range comms {
			commByName[cm.Name] = cm.ID
		}
	}
	existingPhones := map[string]bool{}
	{
		var phones []string
		s.db.Model(&model.SysUser{}).Pluck("phone", &phones)
		for _, p := range phones {
			existingPhones[p] = true
		}
		// username 即手机号，同样占用
		var usernames []string
		s.db.Model(&model.SysUser{}).Pluck("username", &usernames)
		for _, u := range usernames {
			existingPhones[u] = true
		}
	}

	result := &dto.ImportResult{Total: len(dataRows), FailDetails: []dto.FailDetail{}}
	seen := map[string]bool{}
	for i, row := range dataRows {
		cell := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		name, phone, roleText, commText := cell(0), cell(1), cell(2), cell(3)
		initPwd, statusText, remark := cell(4), cell(5), cell(6)

		fail := func(reason string) {
			result.FailDetails = append(result.FailDetails, dto.FailDetail{Row: rowNums[i], Phone: phone, Reason: reason})
		}
		// 1. 必填校验（所属小区改为选填：填写则同步写入项目编制，岗位在编制页维护）
		if name == "" || phone == "" || roleText == "" {
			fail("姓名/手机号/角色均为必填")
			continue
		}
		// 2. 手机号格式与唯一性（库中或本批次重复均失败）
		if !password.ValidPhone(phone) {
			fail("手机号格式错误")
			continue
		}
		if existingPhones[phone] || seen[phone] {
			fail("手机号已存在")
			continue
		}
		// 3. 角色逐一匹配
		roleNames := strings.Split(roleText, ",")
		roleIDs := make([]string, 0, len(roleNames))
		roleOK := true
		for _, rn := range roleNames {
			rn = strings.TrimSpace(rn)
			role, ok := roleByName[rn]
			if !ok {
				fail(fmt.Sprintf("角色「%s」不存在", rn))
				roleOK = false
				break
			}
			if role.Code == model.SuperAdminCode && !op.SuperAdmin {
				fail("仅超级管理员可导入超级管理员角色")
				roleOK = false
				break
			}
			roleIDs = append(roleIDs, role.ID)
		}
		if !roleOK {
			continue
		}
		// 4. 小区逐一匹配（选填；填写则导入后写入项目编制，岗位留空待编制页维护）
		commIDs := make([]string, 0)
		if commText != "" {
			commOK := true
			for _, cn := range strings.Split(commText, ",") {
				cn = strings.TrimSpace(cn)
				id, ok := commByName[cn]
				if !ok {
					fail(fmt.Sprintf("小区「%s」不存在", cn))
					commOK = false
					break
				}
				commIDs = append(commIDs, id)
			}
			if !commOK {
				continue
			}
		}
		// 初始密码：E 列或手机号后 6 位（文档约定默认值，不受手动创建的密码强度规则约束）
		if initPwd != "" && !password.ValidPassword(initPwd) {
			fail("初始密码须为 8–32 位且含字母与数字")
			continue
		}
		if initPwd == "" {
			initPwd = phone[len(phone)-6:]
		}
		status := model.StatusEnabled
		if statusText == "停用" {
			status = model.StatusDisabled
		}
		hash, err := password.Hash(initPwd)
		if err != nil {
			return nil, "", errs.ErrInternal
		}
		user := model.SysUser{
			TenantID:           tenantID,
			Username:           phone,
			Password:           hash,
			Name:               name,
			Phone:              phone,
			RoleIDs:            roleIDs,
			Status:             status,
			MustChangePassword: true, // 导入用户首次登录强制改密
			Remark:             remark,
		}
		if err := s.db.Create(&user).Error; err != nil {
			fail("写入失败：" + err.Error())
			continue
		}
		// 所属小区 → 项目编制（岗位留空，uk_project_staff 冲突忽略）
		for _, cid := range commIDs {
			s.db.Create(&model.ProjectStaff{
				TenantID:  &tenantID,
				ProjectID: cid, UserID: user.ID,
				Posts: types.StringArray{}, BuildingIDs: types.IDArray{},
				Status: model.StatusEnabled,
			})
		}
		seen[phone] = true
		result.SuccessCount++
	}
	result.FailCount = len(result.FailDetails)
	if result.SuccessCount > 0 {
		authz.SyncAllQuiet(s.db) // 导入的新用户需重建 casbin g 规则，否则登录后无任何权限
	}
	msg := fmt.Sprintf("导入完成：成功 %d 条，失败 %d 条", result.SuccessCount, result.FailCount)
	return result, msg, nil
}

// isEmptyRow 判断整行为空。
func isEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func uniqueStrings(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// UpdateProfile 修改当前用户基本资料（姓名、手机号；手机号全局唯一校验）。
// sigKey 为 nil 表示不改动签名；空串表示移除签名；其余为手写签名图 file_id。
// avatar 为 nil 表示不改动头像；空串表示清除头像；其余为 upload_file.id。
// v16 起签名写入签章资产表（创建/替换当前用户的 user_signature 资产），不再写 sys_user 列。
func (s *UserService) UpdateProfile(uid string, name, phone string, sigKey *string, avatar *string) *errs.Error {
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", uid).Error; err != nil {
		return errs.ErrNotFound
	}
	if strings.TrimSpace(name) == "" {
		return errs.ErrParam.WithMsg("name 为必填项")
	}
	if !password.ValidPhone(phone) {
		return errs.ErrParam.WithMsg("phone 手机号格式错误")
	}
	// 手机号同时可能作为他人登录名（导入用户 username=手机号），两处都要排重
	var count int64
	s.db.Model(&model.SysUser{}).Where("id <> ? AND (phone = ? OR username = ?)", uid, phone, phone).Count(&count)
	if count > 0 {
		return errs.ErrPhoneExists
	}
	if sigKey != nil && s.signAssets != nil {
		value := strings.TrimSpace(*sigKey)
		if value != "" {
			f, err := uploadfile.ByID(s.db, value)
			if err != nil || f.UserID != uid {
				return errs.ErrPhotoNotUploaded
			}
			value = f.ID // SetUserSignature 收文件 ID（统一口径）
		}
		if be := s.signAssets.SetUserSignature(uid, value); be != nil {
			return be
		}
	}
	updates := map[string]any{"name": name, "phone": phone}
	if avatar != nil {
		value := strings.TrimSpace(*avatar)
		if value != "" {
			f, err := uploadfile.ByID(s.db, value)
			if err != nil || f.Scene != "avatar" || !sameTenantFile(f.TenantID, u.TenantID) {
				return errs.ErrPhotoNotUploaded
			}
			value = f.ID
		}
		updates["avatar"] = value
	}
	if err := s.db.Model(&u).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func sameTenantFile(fileTenantID *string, userTenantID string) bool {
	if fileTenantID == nil || userTenantID == "" {
		return fileTenantID == nil && userTenantID == ""
	}
	return *fileTenantID == userTenantID
}

// ChangePassword 修改当前用户密码：校验旧密码，新密码强度同创建规则；
// 成功后清除 must_change_password，并使该用户全部会话失效（含当前，需重新登录）。
func (s *UserService) ChangePassword(ctx context.Context, uid string, oldPwd, newPwd string) *errs.Error {
	var u model.SysUser
	if err := s.db.First(&u, "id = ?", uid).Error; err != nil {
		return errs.ErrNotFound
	}
	if !password.Compare(u.Password, oldPwd) {
		return errs.ErrBadCredentials
	}
	if !password.ValidPassword(newPwd) {
		return errs.ErrParam.WithMsg("新密码须为 8–32 位且含字母与数字")
	}
	if oldPwd == newPwd {
		return errs.ErrParam.WithMsg("新密码不能与旧密码相同")
	}
	hash, err := password.Hash(newPwd)
	if err != nil {
		return errs.ErrInternal
	}
	if err := s.db.Model(&u).Updates(map[string]any{
		"password":             hash,
		"must_change_password": false,
	}).Error; err != nil {
		return errs.ErrInternal
	}
	s.killSessions(ctx, uid)
	return nil
}
