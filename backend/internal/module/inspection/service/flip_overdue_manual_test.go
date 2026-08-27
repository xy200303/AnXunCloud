package service

import (
	"os"
	"testing"
	"time"

	"anxuncloud/internal/config"
	"anxuncloud/internal/pkg/database"
	"anxuncloud/internal/pkg/notify"
)

// TestFlipOverdueManual 一次性手动触发：执行逾期翻转（含汇报线/本人通知），验证通知落库。
// 用法：FLIP_OVERDUE=1 go test ./internal/module/inspection/service/ -run TestFlipOverdueManual -v
func TestFlipOverdueManual(t *testing.T) {
	if os.Getenv("FLIP_OVERDUE") == "" {
		t.Skip("未设置 FLIP_OVERDUE，跳过")
	}
	time.Local = time.FixedZone("CST", 8*3600)
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
	svc := NewPlanService(db, nil, notify.New(db, nil), nil)
	n, err := svc.FlipOverdue()
	if err != nil {
		t.Fatalf("逾期翻转失败: %v", err)
	}
	t.Logf("逾期翻转完成，共 %d 个任务置为 overdue", n)
}
