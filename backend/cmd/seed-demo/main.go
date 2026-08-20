// 演示数据播种命令（独立二进制，与 server 主流程完全解耦）。
// 用法：seed-demo            全量播种（幂等，演示租户已存在则跳过）
//
//	seed-demo -photos     仅回填演示工单照片（老库升级用，幂等，不动其他数据）
//
// 自给自足：执行前先跑结构迁移 + 系统预置 seed（均幂等），空库也能直接生成演示数据。
// 注意：演示账号的 casbin 策略由 server 启动时 SyncAll 写入，运行中的服务需重启后生效。
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"anxuncloud/internal/config"
	"anxuncloud/internal/demo"
	"anxuncloud/internal/pkg/database"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/storage"
)

func main() {
	photosOnly := flag.Bool("photos", false, "仅回填演示工单照片（老库升级用）")
	flag.Parse()
	// 与 server 保持一致：统一东八区（演示任务/打卡时间按本地时区生成）
	time.Local = time.FixedZone("CST", 8*3600)

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
	if err := database.Migrate(db); err != nil {
		logger.L.Fatal("数据库迁移失败", zap.Error(err))
	}
	if err := database.Seed(db, cfg.Admin.Username, cfg.Admin.Password, cfg.Admin.Name); err != nil {
		logger.L.Fatal("初始化数据失败", zap.Error(err))
	}

	store := storage.New(cfg.Upload, cfg.OSS, cfg.COS, cfg.App.BaseURL)
	if *photosOnly {
		if err := demo.SeedOrderPhotos(db, store); err != nil {
			logger.L.Fatal("演示工单照片回填失败", zap.Error(err))
		}
		logger.L.Info("演示工单照片回填完成")
		return
	}
	if err := demo.Seed(db, store); err != nil {
		logger.L.Fatal("演示数据写入失败", zap.Error(err))
	}
	logger.L.Info("演示数据写入完成（如服务正在运行，请重启后端使演示账号权限策略生效）")
}
