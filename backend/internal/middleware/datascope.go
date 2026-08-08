package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/pkg/errs"
)

// 数据权限基础能力：按当前用户的 community_ids 过滤查询。
// 业务模块（小区/点位/任务/工单）在查询层调用 ApplyCommunityFilter 即可完成按小区隔离。

// ApplyCommunityFilter 为查询追加小区数据权限过滤。
// column 为含表前缀的字段名，如 "inspection_task.community_id"。
// data_scope=all 不加过滤；custom 且无小区时返回空结果（1=0）。
func ApplyCommunityFilter(db *gorm.DB, c *gin.Context, column string) *gorm.DB {
	identity := CurrentIdentity(c)
	if identity == nil || identity.DataScopeAll {
		return db
	}
	if len(identity.CommunityIDs) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where(column+" IN ?", []string(identity.CommunityIDs))
}

// CheckCommunity 校验当前用户是否有权访问指定小区数据，越权返回 40302。
func CheckCommunity(c *gin.Context, communityID string) *errs.Error {
	identity := CurrentIdentity(c)
	if identity == nil || identity.DataScopeAll {
		return nil
	}
	for _, id := range identity.CommunityIDs {
		if id == communityID {
			return nil
		}
	}
	return errs.ErrDataScope
}
