// Package database 负责 PostgreSQL 连接、结构迁移与初始化数据。
package database

import (
	"fmt"
	"net/url"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"anxuncloud/internal/config"
)

// Connect 建立 GORM 连接池。
func Connect(cfg config.PostgresConfig) (*gorm.DB, error) {
	query := url.Values{"sslmode": {cfg.SSLMode}}
	dsn := fmt.Sprintf("postgres://%s@%s:%d/%s?%s",
		url.UserPassword(cfg.User, cfg.Password).String(), cfg.Host, cfg.Port, cfg.DBName, query.Encode()+"&TimeZone=Asia/Shanghai")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}
