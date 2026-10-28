package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/pkg/types"
)

type CandidateCache struct {
	db    *gorm.DB
	users map[string]types.IDArray
}

func NewCandidateCache(db *gorm.DB) *CandidateCache {
	return &CandidateCache{db: db, users: map[string]types.IDArray{}}
}

func (c *CandidateCache) Users(communityID, roleCode string) types.IDArray {
	key := communityID + "|" + roleCode
	users, ok := c.users[key]
	if !ok {
		users = RoleCandidateUserIDsForCommunity(c.db, communityID, roleCode)
		c.users[key] = users
	}
	return users
}

func (c *CandidateCache) Authorized(communityID, roleCode string, id *middleware.Identity) bool {
	if id == nil {
		return false
	}
	for _, uid := range c.Users(communityID, roleCode) {
		if uid == id.UserID {
			return true
		}
	}
	return false
}
