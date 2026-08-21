package service

import (
	"os"
	"testing"
	"time"

	"anxuncloud/internal/config"
	"anxuncloud/internal/pkg/database"
)

// TestBuildStatsPatrolTypeManual 手动验证 buildStats 巡查类型过滤口径（连 dev 库，只读）：
// 同一小区同一期间，专项口径（patrol_type 非空）的任务数/巡检员名单应不大于综合口径（全类型）。
//
//	用法：REPORT_STATS=1 STATS_COMMUNITY_ID=<小区UUID> STATS_PERIOD=YYYY-MM STATS_PATROL_TYPE=equipment \
//		go test ./internal/module/report/service/ -run TestBuildStatsPatrolTypeManual -v
func TestBuildStatsPatrolTypeManual(t *testing.T) {
	if os.Getenv("REPORT_STATS") == "" {
		t.Skip("未设置 REPORT_STATS，跳过")
	}
	communityID := os.Getenv("STATS_COMMUNITY_ID")
	period := os.Getenv("STATS_PERIOD")
	patrolType := os.Getenv("STATS_PATROL_TYPE")
	if communityID == "" || period == "" || patrolType == "" {
		t.Skip("缺 STATS_COMMUNITY_ID/STATS_PERIOD/STATS_PATROL_TYPE，跳过")
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
	svc := NewReportService(db, nil, nil, nil)

	allStats, allIDs, be := svc.buildStats(communityID, period, "")
	if be != nil {
		t.Fatalf("综合口径取数失败: %v", be)
	}
	typedStats, typedIDs, be := svc.buildStats(communityID, period, patrolType)
	if be != nil {
		t.Fatalf("专项口径取数失败: %v", be)
	}
	num := func(m map[string]any, key string) int64 {
		v, _ := m[key].(int64)
		return v
	}
	for _, key := range []string{"task_total", "task_done", "abnormal_count", "wo_created"} {
		if num(typedStats, key) > num(allStats, key) {
			t.Fatalf("专项口径 %s=%d 不应大于综合口径 %d", key, num(typedStats, key), num(allStats, key))
		}
		t.Logf("%s: 综合=%d 专项(%s)=%d", key, num(allStats, key), patrolType, num(typedStats, key))
	}
	allSet := map[string]bool{}
	for _, id := range allIDs {
		allSet[id] = true
	}
	for _, id := range typedIDs {
		if !allSet[id] {
			t.Fatalf("专项巡检员 %s 不在综合名单内", id)
		}
	}
	t.Logf("巡检员名单：综合 %d 人，专项 %d 人（专项为综合子集）", len(allIDs), len(typedIDs))
}
