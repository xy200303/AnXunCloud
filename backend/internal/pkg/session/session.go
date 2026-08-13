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

// 会话按「渠道 + 用户 + access jti」存储：同一账号多点登录各自独立会话，互不挤下线。
func sessionKey(channel, userID, tokenID string) string {
	return "session:" + channel + ":" + userID + ":" + tokenID
}

func sessionPrefix(channel, userID string) string {
	return "session:" + channel + ":" + userID + ":"
}

// Save 保存会话，TTL 取 refresh token 有效期（access 过期由 JWT exp 控制）。
func (s *Store) Save(ctx context.Context, channel string, userID string, info Info, ttl time.Duration) error {
	key := sessionKey(channel, userID, info.TokenID)
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

// Get 读取会话（按 access jti 精确定位）；不存在返回 nil, nil。
func (s *Store) Get(ctx context.Context, channel, userID, tokenID string) (*Info, error) {
	return s.read(ctx, sessionKey(channel, userID, tokenID))
}

// GetByRefresh 按 refresh jti 反查会话（刷新令牌场景；用户会话数量小，SCAN 代价可忽略）。
func (s *Store) GetByRefresh(ctx context.Context, channel, userID, refreshID string) (*Info, error) {
	infos, err := s.List(ctx, channel, userID)
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		if info.RefreshID == refreshID {
			return info, nil
		}
	}
	return nil, nil
}

// List 列出用户某渠道全部会话（停用/改密时批量失效用）。
func (s *Store) List(ctx context.Context, channel, userID string) ([]*Info, error) {
	var keys []string
	var cursor uint64
	prefix := sessionPrefix(channel, userID)
	for {
		ks, cur, err := s.rdb.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, ks...)
		cursor = cur
		if cursor == 0 {
			break
		}
	}
	infos := make([]*Info, 0, len(keys))
	for _, k := range keys {
		info, err := s.read(ctx, k)
		if err != nil {
			return nil, err
		}
		if info != nil {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

// Delete 删除单个会话（登出：只退出当前登录点）。
func (s *Store) Delete(ctx context.Context, channel, userID, tokenID string) error {
	return s.rdb.Del(ctx, sessionKey(channel, userID, tokenID)).Err()
}

// DeleteAll 删除用户某渠道全部会话（重置密码/停用/删除账号时调用）。
func (s *Store) DeleteAll(ctx context.Context, channel, userID string) error {
	var cursor uint64
	prefix := sessionPrefix(channel, userID)
	for {
		keys, cur, err := s.rdb.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = cur
		if cursor == 0 {
			break
		}
	}
	return nil
}

// read 读取单个 key 的会话内容。
func (s *Store) read(ctx context.Context, key string) (*Info, error) {
	m, err := s.rdb.HGetAll(ctx, key).Result()
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
