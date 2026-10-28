package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/module/system/model"
)

func IdentityForUser(db *gorm.DB, userID string) (*Identity, error) {
	var user model.SysUser
	if err := db.Select("id", "tenant_id", "username", "name", "status", "role_id").First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return buildIdentity(db, &user, "")
}

func SetIdentity(c *gin.Context, identity *Identity) {
	c.Set(ctxIdentity, identity)
}
