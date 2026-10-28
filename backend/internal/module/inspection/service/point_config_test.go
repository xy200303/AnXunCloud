package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
)

func TestRequirePointExecutableBlocksAndRecoversConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.InspectionPoint{}, &model.PointConfigBlock{}, &model.CheckinRecord{}); err != nil {
		t.Fatal(err)
	}
	tenantID := "01a00000-0000-7000-8000-000000000001"
	point := &model.InspectionPoint{TenantID: &tenantID, CommunityID: "01a00000-0000-7000-8000-000000000002", Credential: "none", RequireFence: true}
	if err := db.Create(point).Error; err != nil {
		t.Fatal(err)
	}
	if be := RequirePointExecutable(db, point); be == nil || be.Code != 43110 {
		t.Fatalf("围栏缺坐标必须 blocked_by_config: %v", be)
	}
	point.Longitude, point.Latitude, point.FenceRadius = 116.3, 39.9, 100
	if be := RequirePointExecutable(db, point); be != nil {
		t.Fatalf("配置修复后应恢复可执行: %v", be)
	}
	var block model.PointConfigBlock
	if err := db.First(&block, "point_id = ?", point.ID).Error; err != nil {
		t.Fatal(err)
	}
	if block.EndedAt == nil {
		t.Fatal("配置修复后应关闭阻塞并保留起止时间")
	}
	point.RequireFence = true
	point.Longitude, point.Latitude = 0, 0
	if be := RequirePointExecutable(db, point); be == nil {
		t.Fatal("重新出现配置缺陷时应重新登记")
	}
	blocked, err := BlockedUnfinishedPointIDs(db, "task-1", []string{point.ID})
	if err != nil || len(blocked) != 1 {
		t.Fatalf("未完成阻断点位应剔除: ids=%v err=%v", blocked, err)
	}
	record := model.CheckinRecord{TaskID: "task-1", PointID: point.ID}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	blocked, err = BlockedUnfinishedPointIDs(db, "task-1", []string{point.ID})
	if err != nil || len(blocked) != 0 {
		t.Fatalf("已有有效记录不得重复剔除: ids=%v err=%v", blocked, err)
	}
}
