package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	eqmodel "anxuncloud/internal/module/equipment/model"
	filemodel "anxuncloud/internal/module/file/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/types"
)

func TestDueMaintenanceReviewWritesConfirmedLedgerOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.WorkOrder{}, &model.WorkOrderIssue{}, &model.WorkOrderFixAttempt{}, &model.WorkOrderEvent{}, &model.WorkOrderMaintenanceItem{},
		&eqmodel.Equipment{}, &eqmodel.EquipmentMaintenance{}, &sysmodel.SysRole{}, &sysmodel.SysUser{}, &sysmodel.ProjectStaff{}, &sysmodel.Community{}, &sysmodel.SysDictData{}, &sysmodel.UploadFile{}, &filemodel.ObjectRef{}); err != nil {
		t.Fatal(err)
	}
	tenantID, communityID := "tenant-a", "community-a"
	role := sysmodel.SysRole{Code: sysmodel.EngineerSupervisorCode, ScopeLevel: sysmodel.ScopeProject, Status: sysmodel.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	reviewer := sysmodel.SysUser{TenantID: tenantID, Username: "reviewer", PersonKey: "reviewer", RoleID: &role.ID, Status: sysmodel.StatusEnabled}
	assignee := sysmodel.SysUser{TenantID: tenantID, Username: "assignee", PersonKey: "assignee", RoleID: &role.ID, Status: sysmodel.StatusEnabled}
	for _, user := range []*sysmodel.SysUser{&reviewer, &assignee} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&sysmodel.ProjectStaff{TenantID: &tenantID, ProjectID: communityID, UserID: user.ID, Status: sysmodel.StatusEnabled}).Error; err != nil {
			t.Fatal(err)
		}
	}
	community := sysmodel.Community{TenantID: tenantID, Name: "小区", Status: sysmodel.StatusEnabled}
	community.ID = communityID
	if err := db.Create(&community).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sysmodel.SysDictData{TypeCode: "equipment_type", Value: "extinguisher", Label: "灭火器", Attrs: types.JSONMap{"first_months": 60, "cycle_months": 24}}).Error; err != nil {
		t.Fatal(err)
	}
	due := time.Now().AddDate(0, 0, 1)
	equipment := eqmodel.Equipment{TenantID: &tenantID, CommunityID: communityID, Type: "extinguisher", Name: "灭火器", Code: "MHQ-1", Status: eqmodel.StatusInService, NextDueDate: &due}
	if err := db.Create(&equipment).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewWorkOrderService(db)
	for i := range 2 {
		created, err := svc.CreateDueMaintenance(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		expected := 0
		if i == 0 {
			expected = 1
		}
		if created != expected {
			t.Fatalf("第 %d 次扫描应生成 %d 个工单项，实际 %d", i+1, expected, created)
		}
	}
	var order model.WorkOrder
	if err := db.First(&order).Error; err != nil {
		t.Fatal(err)
	}
	order.AssigneeID = &assignee.ID
	order.Status = model.StatusPendingReview
	if err := db.Save(&order).Error; err != nil {
		t.Fatal(err)
	}
	var issue model.WorkOrderIssue
	if err := db.First(&issue, "order_id = ?", order.ID).Error; err != nil {
		t.Fatal(err)
	}
	issue.Status = model.StatusPendingReview
	if err := db.Save(&issue).Error; err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	middleware.SetIdentity(c, &middleware.Identity{UserID: reviewer.ID, TenantID: tenantID, DataScopeAll: true})
	if be := svc.Review(c, order.ID, ReviewReq{IssueID: issue.ID, Action: "approve"}); be != nil {
		t.Fatal(be.Msg)
	}
	var count int64
	if err := db.Model(&eqmodel.EquipmentMaintenance{}).Where("equipment_id = ? AND confirm_status = ?", equipment.ID, eqmodel.ConfirmConfirmed).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("审核后应只有一条 confirmed 流水: %d, %v", count, err)
	}
	var saved eqmodel.Equipment
	if err := db.First(&saved, "id = ?", equipment.ID).Error; err != nil || saved.LastMaintenanceDate == nil {
		t.Fatalf("审核后未回写台账: %+v, %v", saved, err)
	}
}
