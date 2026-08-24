// Package middleware 提供 JWT 认证、RBAC 鉴权、数据权限、操作日志、CORS、recovery 等中间件。
package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/session"
)

// 上下文键
const ctxIdentity = "pi_identity"

// Identity 当前登录用户身份与权限快照。
type Identity struct {
	UserID        string
	TenantID      string // 所属租户（P3 多租户：超管账号归属默认租户；业务数据可在未指定上下文时平台级访问）
	Username      string
	Name          string
	JTI           string // 当前 access token 的 jti
	SuperAdmin    bool
	Perms         map[string]struct{} // 按钮级权限点集合（超管为全集）
	DataScopeAll  bool                // true 表示全部项目（角色 data_scope=all 或超管）
	ScopeSelf     bool                // true 表示仅本人相关（纯一线岗位，或角色 data_scope=self 强制收窄）
	ProjectIDs    []string            // project 档可见项目集合（按岗位编制推导：项目经理/主管级岗位覆盖的项目）
	RoleCodes     []string
	AccessExpiresAt time.Time // 当前 access token 过期时间（登出时计算黑名单 TTL）
}

// CurrentIdentity 从上下文取当前身份；未登录返回 nil。
func CurrentIdentity(c *gin.Context) *Identity {
	if v, ok := c.Get(ctxIdentity); ok {
		if id, ok := v.(*Identity); ok {
			return id
		}
	}
	return nil
}

// CurrentUserID 取当前用户 ID（未登录返回空串）。
func CurrentUserID(c *gin.Context) string {
	if id := CurrentIdentity(c); id != nil {
		return id.UserID
	}
	return ""
}

// Auth JWT 认证中间件：解析 token → 黑名单校验 → 会话校验 → 加载用户与权限。
// channel 为会话渠道（admin / mp），对应 session:{channel}:{userId}。
func Auth(db *gorm.DB, sess *session.Store, jwtm *jwtutil.Manager, channel string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		claims, err := jwtm.Parse(token)
		if err != nil {
			if err == jwtutil.ErrExpired {
				response.Fail(c, errs.ErrTokenExpired)
			} else {
				response.Fail(c, errs.ErrUnauthorized)
			}
			return
		}
		if claims.Type != jwtutil.TypeAccess {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		// JWT 黑名单（登出/改密/停用后未过期 token 立即失效）
		black, err := sess.IsBlacklisted(c.Request.Context(), claims.ID)
		if err != nil || black {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		// 会话存活校验（按 access jti 精确匹配当前登录点；登出即删会话，旧 token 随之失效）
		sessInfo, err := sess.Get(c.Request.Context(), channel, claims.UserID, claims.ID)
		if err != nil || sessInfo == nil {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		// 加载用户最新状态（停用/改角色即时生效）
		var user model.SysUser
		if err := db.Select("id", "tenant_id", "username", "name", "status", "role_ids").
			First(&user, "id = ?", claims.UserID).Error; err != nil {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		if user.Status != model.StatusEnabled {
			response.Fail(c, errs.ErrAccountDisabled)
			return
		}
		identity, err := buildIdentity(db, &user, claims.ID)
		if err != nil {
			// 租户停用等业务错误直接透传（40108），其余按内部错误处理
			if be, ok := err.(*errs.Error); ok {
				response.Fail(c, be)
			} else {
				response.Fail(c, errs.ErrInternal)
			}
			return
		}
		identity.AccessExpiresAt = claims.ExpiresAt.Time
		c.Set(ctxIdentity, identity)
		c.Next()
	}
}

// extractToken 依次尝试 Authorization: Bearer 与 X-Token 请求头。
func extractToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h != "" {
		if parts := strings.SplitN(h, " ", 2); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	return c.GetHeader("X-Token")
}

// buildIdentity 汇总用户角色、权限点与数据范围。
// 租户语义（P3）：TenantID 取用户所属租户；租户已停用则返回 ErrTenantDisabled（每请求直查，见 tenant.go）。
func buildIdentity(db *gorm.DB, user *model.SysUser, jti string) (*Identity, error) {
	enabled, err := TenantEnabled(db, user.TenantID)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, errs.ErrTenantDisabled
	}
	identity := &Identity{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Username: user.Username,
		Name:     user.Name,
		JTI:      jti,
		Perms:    map[string]struct{}{},
	}
	var roles []model.SysRole
	// 有效角色并集（方案 §3）：手动分配角色 ∪ 在职编制岗位绑定角色（post_dict.role_id），实时计算不落库
	effectiveRoleIDs, err := EffectiveRoleIDs(db, user)
	if err != nil {
		return nil, err
	}
	if len(effectiveRoleIDs) > 0 {
		if err := db.Where("id IN ? AND status = ?", effectiveRoleIDs, model.StatusEnabled).Find(&roles).Error; err != nil {
			return nil, err
		}
	}
	// 角色 data_scope 上限：任一 all → all；否则任一 project → project；全为 self → self
	roleBound := model.ScopeSelf
	roleIDs := make([]string, 0, len(roles))
	for _, r := range roles {
		identity.RoleCodes = append(identity.RoleCodes, r.Code)
		roleIDs = append(roleIDs, r.ID)
		if r.Code == model.SuperAdminCode {
			identity.SuperAdmin = true
		}
		switch r.DataScope {
		case model.ScopeAll:
			roleBound = model.ScopeAll
		case model.ScopeProject:
			if roleBound != model.ScopeAll {
				roleBound = model.ScopeProject
			}
		}
	}
	// 超管数据范围必为全部
	if identity.SuperAdmin || roleBound == model.ScopeAll {
		identity.DataScopeAll = true
	}
	// 可见项目集合按岗位编制推导（角色上限为 self 时直接收窄，不再查编制）
	if !identity.DataScopeAll {
		if roleBound == model.ScopeSelf {
			identity.ScopeSelf = true
		} else {
			projectIDs, err := ManagedProjectIDs(db, user.ID)
			if err != nil {
				return nil, err
			}
			if len(projectIDs) == 0 {
				identity.ScopeSelf = true
			} else {
				identity.ProjectIDs = projectIDs
			}
		}
	}
	// 权限点集合：超管取全量菜单 perms，否则按角色关联菜单取
	q := db.Model(&model.SysMenu{}).Distinct("perms").
		Where("perms <> '' AND status = ?", model.StatusEnabled)
	if !identity.SuperAdmin {
		if len(roleIDs) == 0 {
			return identity, nil
		}
		q = q.Where("id IN (?)", db.Model(&model.SysRoleMenu{}).Select("menu_id").Where("role_id IN ?", roleIDs))
	}
	var perms []string
	if err := q.Pluck("perms", &perms).Error; err != nil {
		return nil, err
	}
	for _, p := range perms {
		identity.Perms[p] = struct{}{}
	}
	return identity, nil
}

// RequirePerm RBAC 权限点校验中间件（如 system:user:list）。
// 多个权限点时为「任一满足」。内部走 Casbin 鉴权（authz 包，domain=default）。
func RequirePerm(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := CurrentIdentity(c)
		if identity == nil {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		ok, err := authz.EnforceAny(identity.UserID, perms...)
		if err != nil {
			logger.L.Warn("casbin 鉴权异常", zap.Error(err), zap.String("uid", identity.UserID))
			response.Fail(c, errs.ErrInternal)
			return
		}
		if !ok {
			response.Fail(c, errs.ErrNoPerm)
			return
		}
		c.Next()
	}
}
