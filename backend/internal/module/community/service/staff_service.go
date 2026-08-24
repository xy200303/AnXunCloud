// 项目岗位编制与职责槽位绑定业务逻辑（设计方案 §3.2/§4.1/§5.2）。
// 业务身份（月报签字、工单派单等）一律走「槽位绑定 → 岗位 → 项目编制名单」解析，不再从角色/权限点推导。
package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/community/dto"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// StaffService 项目岗位编制与职责槽位绑定服务。
type StaffService struct {
	db *gorm.DB
}

func NewStaffService(db *gorm.DB) *StaffService { return &StaffService{db: db} }

// ResolveSlotPosts 解析槽位绑定的岗位 code，三级回落（菜单归位方案 §3，最终决策）：
// 项目级覆盖 → 租户级默认（project_id 空 + tenant_id=项目所属租户）→ 平台默认（两级皆空）。
// 查不到绑定返回空数组（该环节按规则跳过/降级）。
func ResolveSlotPosts(db *gorm.DB, projectID, slot string) types.StringArray {
	codes, _ := resolveSlotPosts(db, projectID, slot)
	return codes
}

// reportLineSlotFor 巡查类型 → 汇报线维度槽位 code（《专项巡检与专项检查报告设计方案》§3.1：
// 约定 patrol_report_line.<patrol_type>，字典新增类型零代码生效；空类型返回空，直接用通用槽位）。
func reportLineSlotFor(patrolType string) string {
	if patrolType == "" {
		return ""
	}
	return sysmodel.SlotPatrolReportLine + "." + patrolType
}

// ResolveReportLineSlot 巡查汇报线槽位解析（《汇报线与审批链扩展设计方案》§2.2）：
// 维度槽位（patrol_report_line.<line>，任一级存在绑定即命中）→ 通用槽位（patrol_report_line 兜底）。
// 维度槽位绑定存在但岗位留空 = 该线该环节显式跳过（不再回落通用）。
// 返回的槽位 code 供 SlotUserIDs / SlotAuthorized 使用。
func ResolveReportLineSlot(db *gorm.DB, projectID, patrolType string) string {
	if dim := reportLineSlotFor(patrolType); dim != "" {
		if _, source := resolveSlotPosts(db, projectID, dim); source != "" {
			return dim
		}
	}
	return sysmodel.SlotPatrolReportLine
}

// resolveSlotPosts 三级回落解析，返回岗位 code 与来源（project/tenant/platform；空串=未配置）。
func resolveSlotPosts(db *gorm.DB, projectID, slot string) (types.StringArray, string) {
	var b sysmodel.DutyBinding
	if err := db.Where("project_id = ? AND slot = ?", projectID, slot).First(&b).Error; err == nil {
		return b.PostCodes, "project"
	}
	if tid := middleware.CommunityTenantID(db, projectID); tid != nil {
		if err := db.Where("project_id IS NULL AND tenant_id = ? AND slot = ?", *tid, slot).First(&b).Error; err == nil {
			return b.PostCodes, "tenant"
		}
	}
	if err := db.Where("project_id IS NULL AND tenant_id IS NULL AND slot = ?", slot).First(&b).Error; err == nil {
		return b.PostCodes, "platform"
	}
	return types.StringArray{}, ""
}

// SlotUserIDs 槽位默认人员名单：绑定岗位在该项目编制内的在职成员（用户须存在且启用，去重）。
// 名单即授权：只要人在名单里即可承担该职责，不再要求持有特定角色/权限点。
func SlotUserIDs(db *gorm.DB, projectID, slot string) types.IDArray {
	posts := ResolveSlotPosts(db, projectID, slot)
	if len(posts) == 0 {
		return types.IDArray{}
	}
	postSet := make(map[string]bool, len(posts))
	for _, p := range posts {
		postSet[p] = true
	}
	var staffs []sysmodel.ProjectStaff
	if err := db.Select("user_id", "posts").
		Where("project_id = ? AND status = ?", projectID, sysmodel.StatusEnabled).
		Order("created_at ASC").Find(&staffs).Error; err != nil {
		return types.IDArray{}
	}
	candidateIDs := make([]string, 0, len(staffs))
	for _, st := range staffs {
		for _, p := range st.Posts {
			if postSet[p] {
				candidateIDs = append(candidateIDs, st.UserID)
				break
			}
		}
	}
	if len(candidateIDs) == 0 {
		return types.IDArray{}
	}
	enabled := map[string]bool{}
	var valid []string
	db.Model(&sysmodel.SysUser{}).
		Where("id IN ? AND status = ?", candidateIDs, sysmodel.StatusEnabled).
		Pluck("id", &valid)
	for _, id := range valid {
		enabled[id] = true
	}
	out := make(types.IDArray, 0, len(candidateIDs))
	seen := map[string]bool{}
	for _, id := range candidateIDs {
		if enabled[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// SlotAuthorized 名单制授权统一判定（工单各槽位、巡查汇报线共用）：
// 超管与租户管理员默认放行——名单约束的是项目内职责分工，不约束平台/租户管理者；
// 其余用户须为该项目该槽位名单成员（SlotUserIDs 三级回落解析）。
func SlotAuthorized(db *gorm.DB, projectID, slot string, id *middleware.Identity) bool {
	if id == nil {
		return false
	}
	if id.SuperAdmin {
		return true
	}
	for _, code := range id.RoleCodes {
		if code == sysmodel.TenantAdminCode {
			return true
		}
	}
	for _, uid := range SlotUserIDs(db, projectID, slot) {
		if uid == id.UserID {
			return true
		}
	}
	return false
}

// enabledPostDict 小区所属租户的岗位（code → 岗位），仅启用状态参与校验/展示。
// 岗位按租户隔离（菜单归位方案 §3）：编制/槽位绑定的岗位口径 = 项目归属公司的岗位，不是平台模板。
func (s *StaffService) enabledPostDict(communityID string) (map[string]sysmodel.PostDict, *errs.Error) {
	tid := middleware.CommunityTenantID(s.db, communityID)
	if tid == nil {
		return nil, errs.ErrCommunityNotExist
	}
	var posts []sysmodel.PostDict
	if err := s.db.Where("tenant_id = ?", *tid).Find(&posts).Error; err != nil {
		return nil, errs.ErrInternal
	}
	byCode := make(map[string]sysmodel.PostDict, len(posts))
	for _, p := range posts {
		byCode[p.Code] = p
	}
	return byCode, nil
}

// ListPostDict 编制表单岗位下拉：小区所属租户的启用岗位（含业务线，按 line+sort 排序）。
func (s *StaffService) ListPostDict(communityID string) ([]gin.H, *errs.Error) {
	tid := middleware.CommunityTenantID(s.db, communityID)
	if tid == nil {
		return nil, errs.ErrCommunityNotExist
	}
	var posts []sysmodel.PostDict
	if err := s.db.Where("tenant_id = ?", *tid).Order("line ASC, sort ASC, created_at ASC").Find(&posts).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		items = append(items, gin.H{
			"id": p.ID, "code": p.Code, "name": p.Name, "line": p.Line,
			"is_supervisor": p.IsSupervisor, "status": sysmodel.StatusInt(p.Status), "remark": p.Remark,
		})
	}
	return items, nil
}

// ListStaff 项目编制名单（含用户信息与岗位/楼栋名称）。
func (s *StaffService) ListStaff(c *gin.Context, communityID string) ([]gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	var rows []sysmodel.ProjectStaff
	if err := s.db.Where("project_id = ?", communityID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	postByCode, be := s.enabledPostDict(communityID)
	if be != nil {
		return nil, be
	}
	userName, userExtra := s.staffUsers(rows)
	buildingName := s.projectBuildingNames(communityID)
	items := make([]gin.H, 0, len(rows))
	for _, st := range rows {
		postNames := make([]string, 0, len(st.Posts))
		for _, code := range st.Posts {
			if p, ok := postByCode[code]; ok {
				postNames = append(postNames, p.Name)
			}
		}
		buildingNames := make([]string, 0, len(st.BuildingIDs))
		for _, bid := range st.BuildingIDs {
			if n, ok := buildingName[bid]; ok {
				buildingNames = append(buildingNames, n)
			}
		}
		extra := userExtra[st.UserID]
		items = append(items, gin.H{
			"id": st.ID, "project_id": st.ProjectID, "user_id": st.UserID,
			"user_name": userName[st.UserID], "phone": extra[0], "avatar": extra[1],
			"posts": st.Posts, "post_names": postNames,
			"building_ids": st.BuildingIDs, "building_names": buildingNames,
			"status": sysmodel.StatusInt(st.Status), "created_at": timefmt.T(st.CreatedAt),
		})
	}
	return items, nil
}

// staffUsers 批量取编制成员的用户信息（姓名 + phone/avatar 附加字段）。
func (s *StaffService) staffUsers(rows []sysmodel.ProjectStaff) (map[string]string, map[string][2]string) {
	names := map[string]string{}
	extra := map[string][2]string{}
	ids := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, st := range rows {
		if !seen[st.UserID] {
			seen[st.UserID] = true
			ids = append(ids, st.UserID)
		}
	}
	if len(ids) == 0 {
		return names, extra
	}
	var users []sysmodel.SysUser
	s.db.Select("id", "name", "phone", "avatar").Where("id IN ?", ids).Find(&users)
	for _, u := range users {
		names[u.ID] = u.Name
		extra[u.ID] = [2]string{u.Phone, u.Avatar}
	}
	return names, extra
}

// projectBuildingNames 项目楼栋 id → 名称。
func (s *StaffService) projectBuildingNames(communityID string) map[string]string {
	names := map[string]string{}
	var buildings []insmodel.Building
	s.db.Select("id", "name").Where("community_id = ?", communityID).Find(&buildings)
	for _, b := range buildings {
		names[b.ID] = b.Name
	}
	return names
}

// CreateStaff 新增编制成员（同一项目同一用户仅一条编制，由 uk_project_staff 兜底）。
func (s *StaffService) CreateStaff(c *gin.Context, communityID string, req *dto.StaffSaveReq) (string, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return "", be
	}
	var count int64
	s.db.Model(&sysmodel.Community{}).Where("id = ?", communityID).Count(&count)
	if count == 0 {
		return "", errs.ErrCommunityNotExist
	}
	tenantID := middleware.CommunityTenantID(s.db, communityID)
	if tenantID == nil {
		return "", errs.ErrCommunityNotExist
	}
	s.db.Model(&sysmodel.SysUser{}).Where("id = ? AND tenant_id = ? AND status = ?", req.UserID, *tenantID, sysmodel.StatusEnabled).Count(&count)
	if count == 0 {
		return "", errs.ErrParam.WithMsg("编制成员须为存在且启用的用户")
	}
	s.db.Model(&sysmodel.ProjectStaff{}).Where("project_id = ? AND user_id = ?", communityID, req.UserID).Count(&count)
	if count > 0 {
		return "", errs.ErrConflict.WithMsg("该用户已在项目编制内")
	}
	if be := s.validateStaff(communityID, "", req); be != nil {
		return "", be
	}
	row := sysmodel.ProjectStaff{
		TenantID:    middleware.CommunityTenantID(s.db, communityID), // 冗余列（=所属项目租户，P3）
		ProjectID:   communityID,
		UserID:      req.UserID,
		Posts:       types.StringArray(uniqueStrings(req.Posts)),
		BuildingIDs: types.IDArray(req.BuildingIDs),
		Status:      sysmodel.StatusEnabled,
	}
	if row.BuildingIDs == nil {
		row.BuildingIDs = types.IDArray{}
	}
	if req.Status != nil {
		row.Status = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Create(&row).Error; err != nil {
		return "", errs.ErrInternal
	}
	// 编制变更影响有效角色并集（岗位绑定角色），重建 casbin g 规则
	authz.SyncAllQuiet(s.db)
	return row.ID, nil
}

// UpdateStaff 修改编制成员（user_id 不可换绑：换人等价于删除后新增）。
func (s *StaffService) UpdateStaff(c *gin.Context, communityID, staffID string, req *dto.StaffSaveReq) *errs.Error {
	var row sysmodel.ProjectStaff
	if err := s.db.First(&row, "id = ? AND project_id = ?", staffID, communityID).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, row.ProjectID); be != nil {
		return be
	}
	if req.UserID != "" && req.UserID != row.UserID {
		return errs.ErrParam.WithMsg("编制成员不可换绑用户，请删除后重新添加")
	}
	if be := s.validateStaff(communityID, row.ID, req); be != nil {
		return be
	}
	updates := map[string]any{
		"posts":        types.StringArray(uniqueStrings(req.Posts)),
		"building_ids": types.IDArray(req.BuildingIDs),
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	// 编制变更影响有效角色并集，重建 casbin g 规则
	authz.SyncAllQuiet(s.db)
	return nil
}

// DeleteStaff 移除编制成员（硬删除；恢复即重新添加）。
func (s *StaffService) DeleteStaff(c *gin.Context, communityID, staffID string) *errs.Error {
	var row sysmodel.ProjectStaff
	if err := s.db.First(&row, "id = ? AND project_id = ?", staffID, communityID).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, row.ProjectID); be != nil {
		return be
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return errs.ErrInternal
	}
	// 编制移除影响有效角色并集，重建 casbin g 规则
	authz.SyncAllQuiet(s.db)
	return nil
}

// validateStaff 编制校验：岗位须存在于岗位字典且启用；项目经理每项目至多一人；责任楼栋须属于该项目。
// excludeStaffID 用于编辑时排除自身。
func (s *StaffService) validateStaff(communityID, excludeStaffID string, req *dto.StaffSaveReq) *errs.Error {
	postByCode, be := s.enabledPostDict(communityID)
	if be != nil {
		return be
	}
	hasManager := false
	for _, code := range uniqueStrings(req.Posts) {
		p, ok := postByCode[code]
		if !ok || p.Status != sysmodel.StatusEnabled {
			return errs.ErrParam.WithMsg("岗位「" + code + "」不存在或已停用")
		}
		if code == sysmodel.PostProjectManager {
			hasManager = true
		}
	}
	if hasManager {
		// 新状态为停用时本条编制不再占用项目经理名额，无需查重
		willDisable := req.Status != nil && sysmodel.StatusStr(*req.Status) == sysmodel.StatusDisabled
		if !willDisable {
			q := s.db.Model(&sysmodel.ProjectStaff{}).
				Where("project_id = ? AND status = ? AND posts @> ?::jsonb",
					communityID, sysmodel.StatusEnabled, `["`+sysmodel.PostProjectManager+`"]`)
			if excludeStaffID != "" {
				q = q.Where("id <> ?", excludeStaffID)
			}
			var count int64
			q.Count(&count)
			if count > 0 {
				return errs.ErrConflict.WithMsg("每个项目项目经理至多一人")
			}
		}
	}
	if len(req.BuildingIDs) > 0 {
		var count int64
		s.db.Model(&insmodel.Building{}).
			Where("community_id = ? AND id IN ?", communityID, uniqueStrings(req.BuildingIDs)).Count(&count)
		if count != int64(len(uniqueStrings(req.BuildingIDs))) {
			return errs.ErrParam.WithMsg("责任楼栋须属于该项目")
		}
	}
	return nil
}

// ListDutyBindings 项目职责槽位绑定视图：逐槽位给出解析结果（项目级覆盖 → 平台默认）与来源。
func (s *StaffService) ListDutyBindings(c *gin.Context, communityID string) ([]gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	postByCode, be := s.enabledPostDict(communityID)
	if be != nil {
		return nil, be
	}
	overrides := map[string]types.StringArray{}
	var rows []sysmodel.DutyBinding
	if err := s.db.Where("project_id = ?", communityID).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	for _, b := range rows {
		overrides[b.Slot] = b.PostCodes
	}
	items := make([]gin.H, 0, len(sysmodel.DutySlots))
	for _, ds := range AllDutySlots(s.db) {
		var codes types.StringArray
		var source string
		if oc, ok := overrides[ds.Slot]; ok {
			codes, source = oc, "project"
		} else {
			// 三级回落解析（项目级无覆盖时：租户默认 → 平台默认）
			codes, source = resolveSlotPosts(s.db, communityID, ds.Slot)
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

// SaveDutyBindings 保存项目级槽位绑定覆盖（逐槽位 upsert；post_codes 空 = 本项目该环节跳过）。
func (s *StaffService) SaveDutyBindings(c *gin.Context, communityID string, req *dto.DutyBindingsSaveReq) *errs.Error {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return be
	}
	var count int64
	s.db.Model(&sysmodel.Community{}).Where("id = ?", communityID).Count(&count)
	if count == 0 {
		return errs.ErrCommunityNotExist
	}
	postByCode, be := s.enabledPostDict(communityID)
	if be != nil {
		return be
	}
	known := make(map[string]bool, len(sysmodel.DutySlots))
	for _, ds := range AllDutySlots(s.db) {
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
			var row sysmodel.DutyBinding
			err := tx.Where("project_id = ? AND slot = ?", communityID, b.Slot).First(&row).Error
			if err == nil {
				if err := tx.Model(&row).Update("post_codes", codes).Error; err != nil {
					return err
				}
				continue
			}
			row = sysmodel.DutyBinding{ProjectID: &communityID, Slot: b.Slot, PostCodes: codes}
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

// ---------- 审批链配置（项目级覆盖；扩展方案 §3） ----------

// GetReviewFlow 项目审核链视图（来源 project/tenant/platform/default）。
func (s *StaffService) GetReviewFlow(projectID string) (gin.H, *errs.Error) {
	steps, source := ResolveFlowWithSource(s.db, projectID, sysmodel.FlowCheckinReview)
	return gin.H{"flow_code": sysmodel.FlowCheckinReview, "steps": steps, "source": source}, nil
}

// SaveReviewFlow 保存项目级审核链覆盖（upsert project_id 行）。
func (s *StaffService) SaveReviewFlow(projectID string, steps types.FlowStepArray) *errs.Error {
	if be := ValidateFlowSteps(s.db, steps); be != nil {
		return be
	}
	var f sysmodel.ApprovalFlow
	err := s.db.Where("project_id = ? AND flow_code = ?", projectID, sysmodel.FlowCheckinReview).First(&f).Error
	if err != nil {
		f = sysmodel.ApprovalFlow{ProjectID: &projectID, FlowCode: sysmodel.FlowCheckinReview, Steps: steps}
		if err := s.db.Create(&f).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	if err := s.db.Model(&f).Update("steps", steps).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}
