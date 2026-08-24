// 物业巡检管理系统后端入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"anxuncloud/internal/config"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/database"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/redis"
	"anxuncloud/internal/router"
)

func main() {
	// 统一东八区（接口文档 §1.6）
	time.Local = time.FixedZone("CST", 8*3600)

	migrateOnly := flag.Bool("migrate-only", false, "仅执行数据库迁移与种子数据后退出")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	if err := logger.Init(cfg.Log.Level); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.Postgres)
	if err != nil {
		logger.L.Fatal("连接 PostgreSQL 失败", zap.Error(err))
	}
	rdb, err := redis.Connect(cfg.Redis)
	if err != nil {
		logger.L.Fatal("连接 Redis 失败", zap.Error(err))
	}

	// 结构迁移 + 预置数据（幂等）
	if err := database.Migrate(db); err != nil {
		logger.L.Fatal("数据库迁移失败", zap.Error(err))
	}
	if cfg.Env == "prod" && cfg.Admin.Password == "Admin@123" {
		logger.L.Fatal("生产环境禁止使用默认超管密码，请配置 ADMIN_PASSWORD")
	}
	if cfg.Env == "prod" && (len(cfg.JWT.Secret) < 32 || strings.Contains(strings.ToLower(cfg.JWT.Secret), "change")) {
		logger.L.Fatal("生产环境 JWT_SECRET 不符合安全要求，请配置至少 32 位随机密钥")
	}
	if err := database.Seed(db, cfg.Admin.Username, cfg.Admin.Password, cfg.Admin.Name); err != nil {
		logger.L.Fatal("初始化数据失败", zap.Error(err))
	}
	// Casbin 鉴权初始化 + 全量策略同步（seed 之后）
	if err := authz.Init(db); err != nil {
		logger.L.Fatal("casbin 初始化失败", zap.Error(err))
	}
	if err := authz.SyncAll(db); err != nil {
		logger.L.Fatal("casbin 策略同步失败", zap.Error(err))
	}
	logger.L.Info("数据库迁移与初始化完成")
	if *migrateOnly {
		return
	}

	engine, scheduler := router.New(cfg, db, rdb)
	scheduler.Start() // 每日任务生成 / 逾期翻转 / 分区滚动
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: engine,
	}

	go func() {
		logger.L.Info("HTTP 服务启动", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.L.Fatal("HTTP 服务异常", zap.Error(err))
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.L.Info("正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.L.Error("服务关闭异常", zap.Error(err))
	}
}
