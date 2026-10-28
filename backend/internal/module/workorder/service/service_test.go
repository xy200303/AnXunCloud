package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/model"
)

func TestCreateFromInspectionCreatesOneOrderPerItemAndIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.WorkOrder{}, &model.WorkOrderIssue{}, &model.WorkOrderFixAttempt{}, &model.WorkOrderEvent{}, &sysmodel.ApprovalFlow{}, &sysmodel.Community{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}); err != nil {
		t.Fatal(err)
	}
	svc := NewWorkOrderService(db)
	equipmentID := "01a00000-0000-7000-8000-000000000010"
	req := CreateFromInspectionReq{
		TenantID: "01a00000-0000-7000-8000-000000000001", CommunityID: "01a00000-0000-7000-8000-000000000002",
		RecordID: "01a00000-0000-7000-8000-000000000003", SourceRevision: 3, OperatorID: "01a00000-0000-7000-8000-000000000004",
		Items: []CreateFromInspectionItem{
			{ItemID: "01a00000-0000-7000-8000-000000000005", EquipmentID: &equipmentID, Title: "灭火器压力异常", Problem: "压力不足"},
			{ItemID: "01a00000-0000-7000-8000-000000000006", Title: "栓箱通道占用", Problem: "堆放杂物"},
		},
	}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return svc.CreateFromInspection(tx, req) }); err != nil {
			t.Fatal(err)
		}
	}
	var orders []model.WorkOrder
	if err := db.Find(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Fatalf("每个异常项应独立成单，实际 %d", len(orders))
	}
	var issues []model.WorkOrderIssue
	if err := db.Order("source_item_id").Find(&issues).Error; err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("重复入池不得重复建事项，实际 %d", len(issues))
	}
	if issues[0].EquipmentID == nil || *issues[0].EquipmentID != equipmentID {
		t.Fatal("设备身份未传入工单事项")
	}
	for _, issue := range issues {
		if issue.OrderID != orders[0].ID && issue.OrderID != orders[1].ID {
			t.Fatal("事项未绑定有效工单")
		}
		if issue.SourceRevision != 3 {
			t.Fatalf("来源 revision 丢失: %d", issue.SourceRevision)
		}
	}
}
