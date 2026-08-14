package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/jwtutil"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/session"
)

// AuthAny 跨通道认证中间件（/api/files 统一文件层用）：admin/app/mp 任一通道的合法会话均放行。
// 通道绑定语义不变——仍按「签发通道」逐个精确匹配会话，只是不限定单一通道；
// token 额外支持 ?token= 查询参数（<img>/浏览器直连/系统分享面板等无法带请求头的场景）。
func AuthAny(db *gorm.DB, sess *session.Store, jwtm *jwtutil.Manager, channels ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			token = strings.TrimSpace(c.Query("token"))
		}
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
		black, err := sess.IsBlacklisted(c.Request.Context(), claims.ID)
		if err != nil || black {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		// 按签发通道逐个尝试精确会话匹配（保留通道绑定：哪端签发的 token 只在哪端会话有效）
		found := false
		for _, ch := range channels {
			si, err := sess.Get(c.Request.Context(), ch, claims.UserID, claims.ID)
			if err == nil && si != nil {
				found = true
				break
			}
		}
		if !found {
			response.Fail(c, errs.ErrUnauthorized)
			return
		}
		// 加载用户最新状态（停用/改角色即时生效）
		var user model.SysUser
		if err := db.Select("id", "username", "name", "status", "role_ids", "community_ids").
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
			response.Fail(c, errs.ErrInternal)
			return
		}
		identity.AccessExpiresAt = claims.ExpiresAt.Time
		c.Set(ctxIdentity, identity)
		c.Next()
	}
}
