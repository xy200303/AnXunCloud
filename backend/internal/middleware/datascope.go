package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
)

// 数据权限（设计方案 §5.2）：可见项目集合由 project_staff 岗位编制推导，角色 data_scope 作上限。
// 任一为项目经理或主管级（is_supervisor）岗位 → 这些项目可见（project 档）；
// 只有一线岗位 → self 档（项目级查询不可见，仅本人相关数据）；角色 data_scope=self 强制收窄为 self。
// 业务模块（小区/点位/任务/打卡）在查询层调用 ApplyCommunityFilter 即可完成按项目隔离。

// StaffProjectIDs 用户在职编制覆盖的全部项目集合（任意岗位；登录/档案响应 projects 字段用）。
func StaffProjectIDs(db *gorm.DB, userID string) ([]string, error) {
	var ids []string
	err := db.Model(&model.ProjectStaff{}).
		Where("user_id = ? AND status = ?", userID, model.StatusEnabled).
		Order("project_id ASC").Pluck("project_id", &ids).Error
	return ids, err
}

// ManagedProjectIDs 用户在职编制中「项目经理或主管级（is_supervisor）岗位」覆盖的项目集合（project 档推导）。
func ManagedProjectIDs(db *gorm.DB, userID string) ([]string, error) {
	var staffs []model.ProjectStaff
	if err := db.Select("project_id", "posts").
		Where("user_id = ? AND status = ?", userID, model.StatusEnabled).
		Find(&staffs).Error; err != nil {
		return nil, err
	}
	if len(staffs) == 0 {
		return nil, nil
	}
	// 主管级岗位集合（仅启用岗位参与推导）
	supervisorPosts := map[string]bool{}
	var posts []model.PostDict
	if err := db.Select("code").Where("is_supervisor = ? AND status = ?", true, model.StatusEnabled).Find(&posts).Error; err != nil {
		return nil, err
	}
	for _, p := range posts {
		supervisorPosts[p.Code] = true
	}
	seen := map[string]bool{}
	var ids []string
	for _, st := range staffs {
		if seen[st.ProjectID] {
			continue
		}
		managed := false
		for _, post := range st.Posts {
			if post == model.PostProjectManager || supervisorPosts[post] {
				managed = true
				break
			}
		}
		if managed {
			seen[st.ProjectID] = true
			ids = append(ids, st.ProjectID)
		}
	}
	return ids, nil
}

// ApplyCommunityFilter 为查询追加项目（小区）数据权限过滤。
// column 为含表前缀的字段名，如 "inspection_task.community_id"。
// 无 identity（调度/内部调用）放行；超管未指定租户时按平台级查询，显式指定租户时收窄，
// 解析失败按空结果（1=0）处理并记日志（宁可拒不可放）；非超管先加租户边界
// （data_scope=all 也只是「本租户全部项目」，不得跨租户）；
// project 档按可见项目过滤；self 档或无可见项目返回空结果（1=0）。
func ApplyCommunityFilter(db *gorm.DB, c *gin.Context, column string) *gorm.DB {
	identity := CurrentIdentity(c)
	if identity == nil {
		return db
	}
	if identity.SuperAdmin {
		// NewDB 会话：db 可能已带业务查询条件，租户解析必须用干净句柄
		clean := db.Session(&gorm.Session{NewDB: true})
		tid, be := ExplicitTenantID(c, clean)
		if be != nil {
			logger.L.Warn("超管租户上下文解析失败，按空结果过滤",
				zap.String("uid", identity.UserID), zap.String("err", be.Msg))
			return db.Where("1 = 0")
		}
		if tid == "" {
			return db
		}
		sub := db.Session(&gorm.Session{NewDB: true}).Model(&model.Community{}).
			Select("id").Where("tenant_id = ?", tid)
		return db.Where(column+" IN (?)", sub)
	}
	if identity.TenantID != "" {
		// NewDB 干净会话：防止调用方的 WHERE 条件（如点位名模糊、状态过滤）泄漏进小区子查询
		sub := db.Session(&gorm.Session{NewDB: true}).Model(&model.Community{}).
			Select("id").Where("tenant_id = ?", identity.TenantID)
		db = db.Where(column+" IN (?)", sub)
	}
	if identity.DataScopeAll {
		return db
	}
	if identity.ScopeSelf || len(identity.ProjectIDs) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where(column+" IN ?", identity.ProjectIDs)
}

// CheckCommunity 校验当前用户是否有权访问指定项目数据，越权返回 40302。
// 无 identity（调度/内部调用）放行；超管未指定租户时允许访问任意租户小区，
// 显式指定租户时校验小区归属该租户；
// 不一致或小区不存在一律按越权处理（不暴露存在性）；
// 非超管先校验小区归属本租户（data_scope=all 不得跨租户），再按数据范围校验。
// self 档用户不持有项目级可见范围，一律拒绝（本人相关数据由业务层按名单/归属另行放行）。
func CheckCommunity(db *gorm.DB, c *gin.Context, communityID string) *errs.Error {
	identity := CurrentIdentity(c)
	if identity == nil {
		return nil
	}
	if identity.SuperAdmin {
		// NewDB 会话：防御调用方传入带条件的句柄，租户解析必须用干净句柄
		clean := db.Session(&gorm.Session{NewDB: true})
		tid, be := ExplicitTenantID(c, clean)
		if be != nil {
			logger.L.Warn("超管租户上下文解析失败，按越权处理",
				zap.String("uid", identity.UserID), zap.String("err", be.Msg))
			return errs.ErrDataScope
		}
		t := CommunityTenantID(clean, communityID)
		// 小区不存在一律按越权处理，不暴露存在性
		if t == nil {
			return errs.ErrDataScope
		}
		// 未指定租户时为平台级操作；指定租户时必须匹配。
		if tid != "" && *t != tid {
			return errs.ErrDataScope
		}
		return nil
	}
	if identity.TenantID != "" {
		// NewDB 干净会话（与超管分支同规）：防御调用方传入带 WHERE 条件的句柄污染小区归属查询
		t := CommunityTenantID(db.Session(&gorm.Session{NewDB: true}), communityID)
		// 小区不存在或属于其他租户：一律按越权处理，不暴露存在性
		if t == nil || *t != identity.TenantID {
			return errs.ErrDataScope
		}
	}
	if identity.DataScopeAll {
		return nil
	}
	if identity.ScopeSelf {
		return errs.ErrDataScope
	}
	for _, id := range identity.ProjectIDs {
		if id == communityID {
			return nil
		}
	}
	return errs.ErrDataScope
}
