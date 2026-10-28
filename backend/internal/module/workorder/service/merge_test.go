package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/model"
)

func TestMergePendingOrdersAtomicScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.WorkOrder{}, &model.WorkOrderIssue{}, &model.WorkOrderFixAttempt{}, &model.WorkOrderEvent{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}); err != nil {
		t.Fatal(err)
	}
	tenantID, communityA, communityB := "tenant-a", "community-a", "community-b"
	orders := make([]model.WorkOrder, 3)
	for i := range orders {
		orders[i] = model.WorkOrder{TenantID: tenantID, CommunityID: communityA, OrderNo: string(rune('A' + i)), WorkType: model.WorkTypeInspectionFix, Title: "整改", Status: model.StatusPending, ReviewFlow: nil}
		if i == 2 {
			orders[i].CommunityID = communityB
		}
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.WorkOrderIssue{TenantID: tenantID, CommunityID: orders[i].CommunityID, OrderID: orders[i].ID, SourceRecordID: "record", SourceItemID: string(rune('a' + i)), Status: model.StatusPending}).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := NewWorkOrderService(db)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	middleware.SetIdentity(c, &middleware.Identity{UserID: "manager", TenantID: tenantID, DataScopeAll: true})
	result, be := svc.Merge(c, MergeReq{OrderIDs: []string{orders[0].ID, orders[1].ID}, OperationID: "merge-1"})
	if be != nil {
		t.Fatal(be.Msg)
	}
	if result["id"] != orders[0].ID || result["merged"] != 1 {
		t.Fatalf("合并结果错误: %+v", result)
	}
	var issueCount int64
	if err := db.Model(&model.WorkOrderIssue{}).Where("order_id = ?", orders[0].ID).Count(&issueCount).Error; err != nil || issueCount != 2 {
		t.Fatalf("事项未原子迁移: %d, %v", issueCount, err)
	}
	var closed model.WorkOrder
	if err := db.First(&closed, "id = ?", orders[1].ID).Error; err != nil || closed.Status != model.StatusClosed {
		t.Fatalf("被合并工单未关闭: %+v, %v", closed, err)
	}
	if _, be := svc.Merge(c, MergeReq{OrderIDs: []string{orders[0].ID, orders[2].ID}, OperationID: "merge-2"}); be == nil {
		t.Fatal("跨小区工单不得合并")
	}
	var other model.WorkOrder
	if err := db.First(&other, "id = ?", orders[2].ID).Error; err != nil || other.Status != model.StatusPending {
		t.Fatalf("跨小区合并失败后不得改变状态: %+v, %v", other, err)
	}
}
