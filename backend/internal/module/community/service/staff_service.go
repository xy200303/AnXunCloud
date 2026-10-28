package service

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/community/dto"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

type StaffService struct {
	db *gorm.DB
}

func NewStaffService(db *gorm.DB) *StaffService { return &StaffService{db: db} }

func (s *StaffService) ListRoleOptions(c *gin.Context, communityID string) ([]gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	tid := middleware.CommunityTenantID(s.db, communityID)
	if tid == nil {
		return nil, errs.ErrCommunityNotExist
	}
	var roles []sysmodel.SysRole
	if err := s.db.Where("(tenant_id IS NULL OR tenant_id = ?) AND status = ?", *tid, sysmodel.StatusEnabled).
		Order("scope_level, code").Find(&roles).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		items = append(items, gin.H{"id": role.ID, "code": role.Code, "name": role.Name,
			"scope_level": role.ScopeLevel, "is_supervisor": role.IsSupervisor})
	}
	return items, nil
}

func (s *StaffService) ListStaff(c *gin.Context, communityID string) ([]gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	var rows []sysmodel.ProjectStaff
	if err := s.db.Where("project_id = ?", communityID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	users := s.staffUsers(rows)
	buildingName := s.projectBuildingNames(communityID)
	items := make([]gin.H, 0, len(rows))
	for _, st := range rows {
		buildingNames := make([]string, 0, len(st.BuildingIDs))
		for _, bid := range st.BuildingIDs {
			if n, ok := buildingName[bid]; ok {
				buildingNames = append(buildingNames, n)
			}
		}
		u := users[st.UserID]
		items = append(items, gin.H{
			"id": st.ID, "project_id": st.ProjectID, "user_id": st.UserID,
			"user_name": u.Name, "phone": u.Phone, "avatar": u.Avatar,
			"role_id": u.RoleID, "building_ids": st.BuildingIDs, "building_names": buildingNames,
			"effective_from": st.EffectiveFrom, "effective_to": st.EffectiveTo,
			"delegate_user_id": st.DelegateUserID, "delegate_until": st.DelegateUntil,
			"status": sysmodel.StatusInt(st.Status), "created_at": timefmt.T(st.CreatedAt),
		})
	}
	return items, nil
}

func (s *StaffService) staffUsers(rows []sysmodel.ProjectStaff) map[string]sysmodel.SysUser {
	out := map[string]sysmodel.SysUser{}
	ids := make([]string, 0, len(rows))
	for _, st := range rows {
		ids = append(ids, st.UserID)
	}
	if len(ids) == 0 {
		return out
	}
	var users []sysmodel.SysUser
	s.db.Select("id", "name", "phone", "avatar", "role_id").Where("id IN ?", ids).Find(&users)
	for _, u := range users {
		out[u.ID] = u
	}
	return out
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

func (s *StaffService) CreateStaff(c *gin.Context, communityID string, req *dto.StaffSaveReq) (string, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return "", be
	}
	if be := s.validateStaff(communityID, req); be != nil {
		return "", be
	}
	var count int64
	s.db.Model(&sysmodel.ProjectStaff{}).Where("project_id = ? AND user_id = ?", communityID, req.UserID).Count(&count)
	if count > 0 {
		return "", errs.ErrConflict.WithMsg("该用户在此小区已有任职")
	}
	from, be := parseStaffDate(req.EffectiveFrom)
	if be != nil {
		return "", be
	}
	to, be := parseStaffDate(req.EffectiveTo)
	if be != nil {
		return "", be
	}
	delegateUntil, be := parseStaffDate(req.DelegateUntil)
	if be != nil {
		return "", be
	}
	row := sysmodel.ProjectStaff{
		TenantID: middleware.CommunityTenantID(s.db, communityID), ProjectID: communityID,
		UserID: req.UserID, BuildingIDs: types.IDArray(req.BuildingIDs), Status: sysmodel.StatusEnabled,
		EffectiveFrom: from, EffectiveTo: to, DelegateUserID: req.DelegateUserID, DelegateUntil: delegateUntil,
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
	return row.ID, nil
}

func (s *StaffService) UpdateStaff(c *gin.Context, communityID, staffID string, req *dto.StaffSaveReq) *errs.Error {
	var row sysmodel.ProjectStaff
	if err := s.db.First(&row, "id = ? AND project_id = ?", staffID, communityID).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, row.ProjectID); be != nil {
		return be
	}
	if req.UserID != "" && req.UserID != row.UserID {
		return errs.ErrParam.WithMsg("任职不可换绑用户")
	}
	if be := s.validateStaff(communityID, req); be != nil {
		return be
	}
	if req.Status != nil && *req.Status == 0 {
		if be := s.guardCandidateRemoval(communityID, row.UserID); be != nil {
			return be
		}
	}
	from, be := parseStaffDate(req.EffectiveFrom)
	if be != nil {
		return be
	}
	to, be := parseStaffDate(req.EffectiveTo)
	if be != nil {
		return be
	}
	delegateUntil, be := parseStaffDate(req.DelegateUntil)
	if be != nil {
		return be
	}
	updates := map[string]any{"building_ids": types.IDArray(req.BuildingIDs),
		"effective_from": from, "effective_to": to,
		"delegate_user_id": req.DelegateUserID, "delegate_until": delegateUntil}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(&row).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *StaffService) DeleteStaff(c *gin.Context, communityID, staffID string) *errs.Error {
	var row sysmodel.ProjectStaff
	if err := s.db.First(&row, "id = ? AND project_id = ?", staffID, communityID).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, row.ProjectID); be != nil {
		return be
	}
	if be := s.guardCandidateRemoval(communityID, row.UserID); be != nil {
		return be
	}
	if err := s.db.Delete(&row).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

func (s *StaffService) validateStaff(communityID string, req *dto.StaffSaveReq) *errs.Error {
	tid := middleware.CommunityTenantID(s.db, communityID)
	if tid == nil {
		return errs.ErrCommunityNotExist
	}
	var user sysmodel.SysUser
	if err := s.db.First(&user, "id = ? AND tenant_id = ? AND status = ?", req.UserID, *tid, sysmodel.StatusEnabled).Error; err != nil {
		return errs.ErrParam.WithMsg("任职成员须为本租户启用账号")
	}
	if user.RoleID == nil {
		return errs.ErrParam.WithMsg("请先在用户档案中指定角色")
	}
	var role sysmodel.SysRole
	if err := s.db.First(&role, "id = ?", *user.RoleID).Error; err != nil || role.ScopeLevel != sysmodel.ScopeProject {
		return errs.ErrParam.WithMsg("小区任职只能分配给项目级角色")
	}
	if len(req.BuildingIDs) > 0 {
		var count int64
		s.db.Model(&insmodel.Building{}).Where("community_id = ? AND id IN ?", communityID, uniqueStrings(req.BuildingIDs)).Count(&count)
		if count != int64(len(uniqueStrings(req.BuildingIDs))) {
			return errs.ErrParam.WithMsg("责任楼栋须属于该小区")
		}
	}
	if req.DelegateUserID != nil {
		var count int64
		s.db.Model(&sysmodel.SysUser{}).Where("id = ? AND tenant_id = ? AND status = ?", *req.DelegateUserID, *tid, sysmodel.StatusEnabled).Count(&count)
		if count == 0 || *req.DelegateUserID == req.UserID || req.DelegateUntil == nil {
			return errs.ErrParam.WithMsg("代理人须为本租户其他启用账号，并设置代理期限")
		}
	}
	return nil
}

func (s *StaffService) guardCandidateRemoval(communityID, userID string) *errs.Error {
	var user sysmodel.SysUser
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil || user.RoleID == nil {
		return nil
	}
	var role sysmodel.SysRole
	if err := s.db.First(&role, "id = ?", *user.RoleID).Error; err != nil {
		return nil
	}
	for _, code := range []string{sysmodel.FlowCheckinReview, sysmodel.FlowMaintReview, sysmodel.FlowReportReview} {
		for _, step := range ResolveFlow(s.db, communityID, code) {
			if step.Kind != sysmodel.FlowStepKindAI && step.RoleCode == role.Code && len(RoleCandidateUserIDs(s.db, []string{communityID}, role.Code, user.PersonKey)) == 0 {
				return errs.ErrConflict.WithMsg("该成员是审批节点的唯一候选人，请先指定接替人")
			}
		}
	}
	return nil
}

func parseStaffDate(value *string) (*time.Time, *errs.Error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, *value, time.Local); err == nil {
			return &parsed, nil
		}
	}
	return nil, errs.ErrParam.WithMsg("任职或代理日期格式无效")
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

func (s *StaffService) GetFlow(c *gin.Context, communityID, flowCode string) (gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	if !sysmodel.ValidApprovalFlowCode(flowCode) {
		return nil, errs.ErrParam.WithMsg("未知审批流程")
	}
	steps, source := ResolveFlowWithSource(s.db, communityID, flowCode)
	return gin.H{"flow_code": flowCode, "steps": steps, "source": source}, nil
}

func (s *StaffService) SaveFlow(c *gin.Context, communityID, flowCode string, steps types.FlowStepArray) *errs.Error {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return be
	}
	if !sysmodel.ValidApprovalFlowCode(flowCode) {
		return errs.ErrParam.WithMsg("未知审批流程")
	}
	var be *errs.Error
	if flowCode == sysmodel.FlowCheckinReview || flowCode == sysmodel.FlowMaintReview {
		be = ValidateFlowSteps(s.db, steps)
	} else {
		be = ValidateHumanFlowSteps(s.db, steps)
	}
	if be != nil {
		return be
	}
	var f sysmodel.ApprovalFlow
	err := s.db.Where("scope = ? AND scope_id = ? AND flow_code = ?", sysmodel.ScopeProject, communityID, flowCode).First(&f).Error
	if err != nil {
		f = sysmodel.ApprovalFlow{Scope: sysmodel.ScopeProject, ScopeID: communityID, FlowCode: flowCode, Steps: steps}
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
