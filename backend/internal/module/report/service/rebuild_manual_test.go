package service

import (
	"os"
	"testing"
	"time"

	"anxuncloud/internal/config"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/database"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/redis"
	"anxuncloud/internal/pkg/storage"
)

// TestRebuildPDFManual 一次性手动触发：用当前模板重渲染指定报告的归档 PDF
// （状态/签字/统计数据不变，仅覆盖归档文件）。
// 用法：REBUILD_REPORT_ID=<报告UUID> go test ./internal/module/report/service/ -run TestRebuildPDFManual -v
func TestRebuildPDFManual(t *testing.T) {
	id := os.Getenv("REBUILD_REPORT_ID")
	if id == "" {
		t.Skip("未设置 REBUILD_REPORT_ID，跳过")
	}
	time.Local = time.FixedZone("CST", 8*3600)
	// go test 工作目录为包目录，切到模块根使 uploads 相对路径与服务运行一致
	if err := os.Chdir("../../../../"); err != nil {
		t.Fatalf("切换工作目录失败: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	db, err := database.Connect(cfg.Postgres)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	if err := logger.Init(cfg.Log.Level); err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}
	rdb, err := redis.Connect(cfg.Redis)
	if err != nil {
		t.Fatalf("连接 Redis 失败: %v", err)
	}
	configSvc := systemsvc.NewConfigService(db, rdb)
	svc := NewReportService(db, rdb, storage.New(cfg.Upload, cfg.OSS, cfg.COS, cfg.App.BaseURL), configSvc.Get, notify.New(db, nil))
	if err := svc.RebuildPDFByID(id); err != nil {
		t.Fatalf("重渲染失败: %v", err)
	}
	t.Logf("报告 %s 归档 PDF 已用新模板重渲染", id)
}
