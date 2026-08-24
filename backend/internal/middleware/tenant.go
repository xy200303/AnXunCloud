package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
)

// 多租户（P3，设计方案 §4.3）助手：租户状态校验与租户归属解析。
// 隔离语义：行级隔离，Identity.TenantID 由中间件按用户注入，入口表（用户/角色/小区）按租户过滤，
// 业务表沿用 community 链路天然隔离（community 属于租户）。

// TenantEnabled 查询租户是否启用（tenantID 空视为平台级操作，放行）。
// 每请求校验租户停用直接查库：租户状态是低频变更，单条主键查询成本可忽略，暂不引入缓存。
func TenantEnabled(db *gorm.DB, tenantID string) (bool, error) {
	if tenantID == "" {
		return true, nil
	}
	var status string
	err := db.Model(&model.Tenant{}).Select("status").Where("id = ?", tenantID).Limit(1).Pluck("status", &status).Error
	if err != nil {
		return false, err
	}
	// 租户不存在（数据异常）按停用处理，宁可拒绝不可放行
	return status == model.StatusEnabled, nil
}

// CommunityTenantID 取小区所属租户 ID（业务表写 tenant_id 冗余列用；小区不存在返回 nil）。
func CommunityTenantID(db *gorm.DB, communityID string) *string {
	var tenantID string
	err := db.Model(&model.Community{}).Select("tenant_id").Where("id = ?", communityID).Limit(1).Pluck("tenant_id", &tenantID).Error
	if err != nil || tenantID == "" {
		return nil
	}
	return &tenantID
}

// ExplicitTenantID 读取并校验请求中显式指定的租户上下文。
// 未指定时返回空串，不引入默认租户语义；调用方可据此决定是否执行平台级查询。
func ExplicitTenantID(c *gin.Context, db *gorm.DB) (string, *errs.Error) {
	tid := c.Query("tenant_id")
	if tid == "" {
		tid = c.GetHeader("X-Tenant-Id")
	}
	if tid == "" {
		return "", nil
	}
	if db == nil {
		return "", errs.ErrInternal
	}
	var count int64
	if err := db.Model(&model.Tenant{}).Where("id = ?", tid).Count(&count).Error; err != nil {
		return "", errs.ErrInternal
	}
	if count == 0 {
		return "", errs.ErrParam.WithMsg("目标租户不存在")
	}
	return tid, nil
}

// EffectiveTenantID 系统管理（租户级）「租户上下文」解析（《管理后台信息架构与菜单归位方案》第二章）：
//   - 非超管：固定本人租户（请求参数一律忽略）；
//   - 超管：优先 ?tenant_id=（或 X-Tenant-Id 头），缺省 = 默认租户；
//     指定的租户必须存在，否则报参数错误（不做跨租户混合列表）。
//
// 系统管理（租户级）各 controller（用户/角色/签章/日志/公告/企业品牌）统一用它取上下文租户。
func EffectiveTenantID(c *gin.Context, db *gorm.DB) (string, *errs.Error) {
	identity := CurrentIdentity(c)
	if identity == nil {
		return "", errs.ErrUnauthorized
	}
	if !identity.SuperAdmin {
		return identity.TenantID, nil
	}
	tid, be := ExplicitTenantID(c, db)
	if be != nil {
		return "", be
	}
	if tid == "" {
		// 缺省 = 默认租户（私有化部署的唯一租户，体验与无租户一致）
		if err := db.Model(&model.Tenant{}).Select("id").Where("code = ?", model.DefaultTenantCode).
			Limit(1).Pluck("id", &tid).Error; err != nil || tid == "" {
			return "", errs.ErrInternal
		}
		return tid, nil
	}
	return tid, nil
}
