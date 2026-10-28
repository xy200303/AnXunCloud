package service

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/types"
)

// RoleCandidateUserIDs 按业务对象覆盖的小区解析节点候选人，不按账号所属小区猜测范围。
func RoleCandidateUserIDs(db *gorm.DB, communityIDs []string, roleCode, excludePersonKey string) types.IDArray {
	if roleCode == "" || len(communityIDs) == 0 {
		return types.IDArray{}
	}
	now := time.Now()
	var users []sysmodel.SysUser
	q := db.Model(&sysmodel.SysUser{}).
		Select("DISTINCT sys_user.id, sys_user.person_key").
		Joins("JOIN sys_role ON sys_role.id = sys_user.role_id AND sys_role.deleted_at IS NULL").
		Where("sys_user.deleted_at IS NULL AND sys_user.status = ? AND sys_role.status = ? AND sys_role.code = ?", sysmodel.StatusEnabled, sysmodel.StatusEnabled, roleCode).
		Where("EXISTS (SELECT 1 FROM community c WHERE c.id IN ? AND c.deleted_at IS NULL AND c.tenant_id = sys_user.tenant_id AND (sys_role.tenant_id IS NULL OR sys_role.tenant_id = c.tenant_id))", communityIDs)
	projectScope := db.Model(&sysmodel.ProjectStaff{}).Select("1").
		Where("project_staff.user_id = sys_user.id AND project_staff.project_id IN ? AND project_staff.status = ?", communityIDs, sysmodel.StatusEnabled).
		Where("(project_staff.effective_from IS NULL OR project_staff.effective_from <= ?)", now).
		Where("(project_staff.effective_to IS NULL OR project_staff.effective_to > ?)", now)
	q = q.Where("sys_role.scope_level = ? OR (sys_role.scope_level = ? AND EXISTS (?))", sysmodel.ScopeTenant, sysmodel.ScopeProject, projectScope)
	if excludePersonKey != "" {
		q = q.Where("sys_user.person_key <> ?", excludePersonKey)
	}
	if err := q.Order("sys_user.id").Find(&users).Error; err != nil {
		return types.IDArray{}
	}
	ids := make(types.IDArray, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}

func RoleCandidateUserIDsForCommunity(db *gorm.DB, communityID, roleCode string) types.IDArray {
	return RoleCandidateUserIDs(db, []string{communityID}, roleCode, "")
}

func RoleCandidateAuthorized(db *gorm.DB, communityIDs []string, roleCode string, identity *middleware.Identity) bool {
	if identity == nil {
		return false
	}
	for _, id := range RoleCandidateUserIDs(db, communityIDs, roleCode, "") {
		if id == identity.UserID {
			return true
		}
	}
	return false
}

func RoleExecutorUserIDs(db *gorm.DB, communityID, roleCode string) types.IDArray {
	return RoleCandidateUserIDsForCommunity(db, communityID, roleCode)
}

func SameNaturalPerson(db *gorm.DB, firstUserID, secondUserID string) bool {
	if firstUserID == "" || secondUserID == "" {
		return false
	}
	if firstUserID == secondUserID {
		return true
	}
	var keys []string
	if err := db.Model(&sysmodel.SysUser{}).Where("id IN ?", []string{firstUserID, secondUserID}).Pluck("person_key", &keys).Error; err != nil || len(keys) != 2 {
		return false
	}
	return keys[0] != "" && keys[0] == keys[1]
}

// CandidateCheckinRecordIDs 返回当前用户可审的待审记录，供一线人员的“仅本人”深度补充候选记录权限。
func CandidateCheckinRecordIDs(db *gorm.DB, identity *middleware.Identity) types.IDArray {
	if identity == nil || len(identity.RoleCodes) == 0 {
		return types.IDArray{}
	}
	var records []insmodel.CheckinRecord
	if err := db.Select("id", "community_id", "tenant_id", "inspector_id", "audit_step", "flow_snapshot").
		Where("audit_status = ? AND tenant_id = ?", insmodel.AuditPending, identity.TenantID).Limit(5000).Find(&records).Error; err != nil {
		return types.IDArray{}
	}
	flows := map[string]types.FlowStepArray{}
	ids := make(types.IDArray, 0)
	for _, record := range records {
		flow, ok := flows[record.CommunityID]
		if !ok {
			flow = ResolveFlow(db, record.CommunityID, sysmodel.FlowCheckinReview)
			flows[record.CommunityID] = flow
		}
		flow = FlowOrResolve(db, record.FlowSnapshot, record.CommunityID, sysmodel.FlowCheckinReview)
		idx := int(record.AuditStep)
		if idx >= len(flow) || flow[idx].Kind == sysmodel.FlowStepKindAI || flow[idx].RoleCode != identity.RoleCodes[0] {
			continue
		}
		if SameNaturalPerson(db, identity.UserID, record.InspectorID) {
			continue
		}
		if RoleCandidateAuthorized(db, []string{record.CommunityID}, flow[idx].RoleCode, identity) {
			ids = append(ids, record.ID)
		}
	}
	return ids
}
