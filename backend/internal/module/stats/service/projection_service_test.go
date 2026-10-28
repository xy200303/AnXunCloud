package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/stats/model"
)

func TestProjectionSyncDeleteAndRebuild(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Task{}, &model.Checkin{}, &model.ConfigAlert{}, &model.ProjectionMeta{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	loadTask := func(*gorm.DB, string) (contract.TaskProjection, []contract.CheckinProjection, error) {
		return contract.TaskProjection{ID: "task-1", TenantID: "tenant-1", PlanID: "plan-1", CommunityID: "community-1", InspectorID: "user-1", TaskDate: now, TotalPoints: 3, DonePoints: 2, BlockedPoints: 1, SourceUpdatedAt: now}, []contract.CheckinProjection{{ID: "record-1", TaskID: "task-1", TenantID: "tenant-1", CommunityID: "community-1", PointID: "point-1", InspectorID: "user-1", CheckinTime: now, SourceUpdatedAt: now}}, nil
	}
	loadAlert := func(*gorm.DB, string) (contract.ConfigAlertProjection, error) {
		return contract.ConfigAlertProjection{ID: "block-1", TenantID: "tenant-1", CommunityID: "community-1", PointID: "point-1", Reason: "缺坐标", StartedAt: now, SourceUpdatedAt: now}, nil
	}
	ids := func(*gorm.DB) ([]string, error) { return []string{"task-1"}, nil }
	alertIDs := func(*gorm.DB) ([]string, error) { return []string{"point-1"}, nil }
	svc := NewProjectionService(db, loadTask, loadAlert, ids, alertIDs)
	if err := svc.SyncTask(db, "task-1"); err != nil {
		t.Fatal(err)
	}
	var task model.Task
	if err := db.First(&task, "id = ?", "task-1").Error; err != nil {
		t.Fatal(err)
	}
	if task.BlockedPoints != 1 || task.TotalPoints-task.BlockedPoints != 2 {
		t.Fatalf("任务投影可执行分母错误: %+v", task)
	}
	if err := svc.SyncConfigAlert(db, "point-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTask(db, "task-1"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.Task{}).Count(&count)
	db.Model(&model.Checkin{}).Count(&count)
	if count != 0 {
		t.Fatal("删除任务必须同时删除打卡投影")
	}
	if err := svc.Rebuild(); err != nil {
		t.Fatal(err)
	}
	var meta model.ProjectionMeta
	if err := db.First(&meta, "source = ?", "inspection").Error; err != nil {
		t.Fatal(err)
	}
	if meta.LastRebuildAt.IsZero() || meta.SourceUpdatedAt.IsZero() {
		t.Fatalf("重建水位不能为空: %+v", meta)
	}
}
