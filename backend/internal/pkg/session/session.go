// Package session 基于 Redis 的登录会话与 JWT 黑名单（key 设计见《数据库设计文档》第七章）。
package session

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store 会话存储。
type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) *Store { return &Store{rdb: rdb} }

// Info 会话内容。
type Info struct {
	TokenID   string // 当前 access token 的 jti
	RefreshID string // 当前 refresh token 的 jti
	Name      string
	Roles     string // 角色编码，逗号分隔
	LoginAt   string
}

func sessionKey(channel string, userID string) string {
	return "session:" + channel + ":" + userID
}

// Save 保存会话，TTL 取 refresh token 有效期（access 过期由 JWT exp 控制）。
func (s *Store) Save(ctx context.Context, channel string, userID string, info Info, ttl time.Duration) error {
	key := sessionKey(channel, userID)
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, key, map[string]any{
		"tokenId":   info.TokenID,
		"refreshId": info.RefreshID,
		"name":      info.Name,
		"roles":     info.Roles,
		"loginAt":   info.LoginAt,
	})
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Get 读取会话；不存在返回 nil, nil。
func (s *Store) Get(ctx context.Context, channel string, userID string) (*Info, error) {
	m, err := s.rdb.HGetAll(ctx, sessionKey(channel, userID)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	return &Info{
		TokenID:   m["tokenId"],
		RefreshID: m["refreshId"],
		Name:      m["name"],
		Roles:     m["roles"],
		LoginAt:   m["loginAt"],
	}, nil
}

// Delete 删除会话（登出/改密/停用账号时调用）。
func (s *Store) Delete(ctx context.Context, channel string, userID string) error {
	return s.rdb.Del(ctx, sessionKey(channel, userID)).Err()
}

// Blacklist 将 jti 写入黑名单，TTL 为该 token 剩余有效期。
func (s *Store) Blacklist(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return s.rdb.Set(ctx, "jwtbl:"+jti, "1", ttl).Err()
}

// IsBlacklisted 判断 jti 是否在黑名单中。
func (s *Store) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, "jwtbl:"+jti).Result()
	return n > 0, err
}
