// Package redis 封装 go-redis 客户端。
package redis

import (
	"context"

	"github.com/redis/go-redis/v9"

	"property-inspection/internal/config"
)

// Connect 建立 Redis 客户端并探测连通性。
func Connect(cfg config.RedisConfig) (*redis.Client, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := cli.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return cli, nil
}
