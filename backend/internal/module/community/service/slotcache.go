package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/types"
)

// SlotCache 环节名单批次缓存：列表/批量判定场景按 project|slot 复用名单，
// 消除逐行重复的 SlotUserIDs 查询。非线程安全，仅供单请求/单批次内使用。
type SlotCache struct {
	db    *gorm.DB
	users map[string]types.IDArray
}

// NewSlotCache 创建环节名单批次缓存。
func NewSlotCache(db *gorm.DB) *SlotCache {
	return &SlotCache{db: db, users: map[string]types.IDArray{}}
}

// Users 环节名单（SlotUserIDs 的缓存版：communityID|slot → 名单，首次查询后批次内复用）。
func (c *SlotCache) Users(communityID, slot string) types.IDArray {
	key := communityID + "|" + slot
	users, ok := c.users[key]
	if !ok {
		users = SlotUserIDs(c.db, communityID, slot)
		c.users[key] = users
	}
	return users
}

// Authorized 名单制授权判定（与 SlotAuthorized 同口径：超管/租户管理员放行，
// 其余用户须为该项目该槽位名单成员；名单按 project|slot 走批次缓存）。
func (c *SlotCache) Authorized(communityID, slot string, id *middleware.Identity) bool {
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
	for _, uid := range c.Users(communityID, slot) {
		if uid == id.UserID {
			return true
		}
	}
	return false
}
