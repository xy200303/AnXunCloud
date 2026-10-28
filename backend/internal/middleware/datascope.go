package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
)

// StaffProjectIDs 返回有效任职覆盖的小区；无任职时不产生项目范围。
func StaffProjectIDs(db *gorm.DB, userID string) ([]string, error) {
	var ids []string
	err := db.Model(&model.ProjectStaff{}).
		Where("user_id = ? AND status = ?", userID, model.StatusEnabled).
		Where("(effective_from IS NULL OR effective_from <= NOW())").
		Where("(effective_to IS NULL OR effective_to > NOW())").
		Order("project_id ASC").Pluck("project_id", &ids).Error
	return ids, err
}

// ApplyCommunityFilter 为查询追加项目（小区）数据权限过滤。
// column 为含表前缀的字段名，如 "inspection_task.community_id"。
// 无 identity（调度/内部调用）放行；超管按租户上下文收窄（未指定 = 默认租户，与 EffectiveTenantID 同规），
// 解析失败按空结果（1=0）处理并记日志（宁可拒不可放）；非超管先加租户边界
// （data_scope=all 也只是「本租户全部项目」，不得跨租户）；
// project 档按可见项目过滤；self 档或无可见项目返回空结果（1=0）。
func ApplyCommunityFilter(db *gorm.DB, c *gin.Context, column string) *gorm.DB {
	identity := CurrentIdentity(c)
	if identity == nil {
		return db.Where("1 = 0")
	}
	if identity.SuperAdmin {
		clean := db.Session(&gorm.Session{NewDB: true})
		tid, be := ExplicitTenantID(c, clean)
		if be != nil || tid == "" {
			return db.Where("1 = 0")
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

// UserCanSeeCommunity 配置时校验用：该用户对该小区是否具备数据可见性（「能审必能看」的「看」）。
// 与登录态 buildIdentity + CheckCommunity 同口径：超管可见；租户不一致不可见；
// data_scope=all 可见；project 档看编制推导的可见项目集合；self 档（含角色上限 self）不可见。
// 用户不存在/停用/租户停用一律不可见。审批链配置页据此把「审了也看不到数据」的名单成员标红。
func UserCanSeeCommunity(db *gorm.DB, userID, communityID string) bool {
	var user model.SysUser
	if err := db.Select("id", "tenant_id", "username", "name", "status", "role_id").
		First(&user, "id = ?", userID).Error; err != nil || user.Status != model.StatusEnabled {
		return false
	}
	identity, err := buildIdentity(db, &user, "")
	if err != nil {
		return false
	}
	if identity.SuperAdmin {
		return true
	}
	if identity.TenantID != "" {
		t := CommunityTenantID(db.Session(&gorm.Session{NewDB: true}), communityID)
		if t == nil || *t != identity.TenantID {
			return false
		}
	}
	if identity.DataScopeAll {
		return true
	}
	for _, id := range identity.ProjectIDs {
		if id == communityID {
			return true
		}
	}
	return false
}

// CheckCommunity 校验当前用户是否有权访问指定项目数据；后台任务必须使用显式 scope 的 owner 契约。
func CheckCommunity(db *gorm.DB, c *gin.Context, communityID string) *errs.Error {
	identity := CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	if identity.SuperAdmin {
		clean := db.Session(&gorm.Session{NewDB: true})
		tid, be := ExplicitTenantID(c, clean)
		if be != nil || tid == "" {
			logger.L.Warn("超管缺少或传入无效租户上下文",
				zap.String("uid", identity.UserID))
			return errs.ErrDataScope
		}
		t := CommunityTenantID(clean, communityID)
		if t == nil || *t != tid {
			return errs.ErrDataScope
		}
		return nil
	}
	if identity.TenantID == "" {
		return errs.ErrDataScope
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
