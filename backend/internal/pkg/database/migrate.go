package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate 使用 goose 执行结构迁移（migrations/ 目录以 embed 内嵌，启动即迁移）。
// 多副本同时启动时通过 PG advisory lock 串行化，避免迁移竞态；
// 迁移失败返回错误由调用方终止启动，绝不带错误库结构对外服务。
func Migrate(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("创建迁移会话锁失败: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("初始化 goose 失败: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("goose 迁移执行失败: %w", err)
	}
	// 按月分区表：确保当月与下月分区存在（持续滚动，不属于一次性迁移，goose 不管这个）
	if err := EnsurePartitions(db, "checkin_record"); err != nil {
		return err
	}
	return EnsurePartitions(db, "sys_operation_log")
}

// EnsurePartitions 为分区表创建当月与下月分区（不存在才建）。
func EnsurePartitions(db *gorm.DB, parent string) error {
	now := time.Now()
	for i := 0; i < 2; i++ {
		first := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.Local)
		next := first.AddDate(0, 1, 0)
		partName := fmt.Sprintf("%s_%04d_%02d", parent, first.Year(), int(first.Month()))
		sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)`,
			partName, parent,
			quoteLiteral(first.Format("2006-01-02")), quoteLiteral(next.Format("2006-01-02")))
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("创建分区 %s 失败: %w", partName, err)
		}
	}
	return nil
}

func quoteLiteral(s string) string { return "'" + s + "'" }
