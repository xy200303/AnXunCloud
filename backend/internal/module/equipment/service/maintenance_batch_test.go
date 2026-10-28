package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/equipment/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/types"
)

const (
	batchTenant   = "01a00000-0000-7000-8000-000000000001"
	batchCommune  = "01a00000-0000-7000-8000-000000000002"
	batchCreator  = "01a00000-0000-7000-8000-000000000003"
	batchReviewer = "01a00000-0000-7000-8000-000000000004"
)

func newMaintenanceBatchTest(t *testing.T) (*MaintenanceService, *gin.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Equipment{}, &model.EquipmentMaintenance{}, &model.MaintenanceBatch{}, &model.MaintenanceBatchItem{}, &model.MaintenanceSelection{}, &sysmodel.SysRole{}, &sysmodel.SysUser{}, &sysmodel.ProjectStaff{}, &sysmodel.Community{}, &sysmodel.SysDictData{}, &sysmodel.ApprovalFlow{}); err != nil {
		t.Fatal(err)
	}
	role := sysmodel.SysRole{Code: sysmodel.QualitySupervisorCode, Name: "品质主管", ScopeLevel: sysmodel.ScopeProject, Status: sysmodel.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	community := sysmodel.Community{TenantID: batchTenant, Name: "测试小区", Status: sysmodel.StatusEnabled}
	community.ID = batchCommune
	if err := db.Create(&community).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sysmodel.ApprovalFlow{Scope: sysmodel.ScopeProject, ScopeID: batchCommune, FlowCode: sysmodel.FlowMaintReview,
		Steps: types.FlowStepArray{{RoleCode: sysmodel.QualitySupervisorCode, Name: "维保审核", Mode: "any"}}}).Error; err != nil {
		t.Fatal(err)
	}
	tenant := batchTenant
	reviewer := sysmodel.SysUser{TenantID: tenant, Username: "reviewer", Name: "reviewer", PersonKey: "reviewer", RoleID: &role.ID, Status: sysmodel.StatusEnabled}
	if err := db.Create(&reviewer).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sysmodel.ProjectStaff{TenantID: &tenant, ProjectID: batchCommune, UserID: reviewer.ID, Status: sysmodel.StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sysmodel.SysDictData{TypeCode: "equipment_type", Label: "灭火器", Value: "extinguisher", Attrs: types.JSONMap{"first_months": 60, "cycle_months": 24}}).Error; err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	middleware.SetIdentity(c, &middleware.Identity{UserID: reviewer.ID, TenantID: batchTenant, RoleCodes: []string{sysmodel.QualitySupervisorCode}, ProjectIDs: []string{batchCommune}})
	return NewMaintenanceService(db, nil, nil), c
}

func createBatchFixture(t *testing.T, db *gorm.DB, count int, scrappedLast bool) model.MaintenanceBatch {
	t.Helper()
	batch := model.MaintenanceBatch{TenantID: batchTenant, CommunityID: batchCommune, MaintenanceType: model.MaintenanceRefill,
		MaintenanceDate: time.Now(), Status: model.BatchPending, CreatedBy: batchCreator,
		ReviewFlow: types.FlowStepArray{{RoleCode: sysmodel.QualitySupervisorCode, Name: "维保审核", Mode: "any"}}}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		equipment := model.Equipment{TenantID: ptrString(batchTenant), CommunityID: batchCommune, Type: "extinguisher", Code: string(rune('A' + i)), Name: "灭火器", Status: model.StatusInService}
		if scrappedLast && i == count-1 {
			equipment.Status = model.StatusScrapped
		}
		if err := db.Create(&equipment).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.MaintenanceBatchItem{TenantID: batchTenant, BatchID: batch.ID, EquipmentID: equipment.ID, Status: model.BatchPending}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return batch
}

func TestConfirmMaintenanceBatchAtomicallyWritesLedger(t *testing.T) {
	svc, c := newMaintenanceBatchTest(t)
	batch := createBatchFixture(t, svc.db, 2, false)
	candidates := communitysvc.RoleCandidateUserIDsForCommunity(svc.db, batchCommune, sysmodel.QualitySupervisorCode)
	if len(candidates) != 1 {
		t.Fatalf("测试候选人应为 1 人，实际 %v", candidates)
	}
	result, be := svc.ConfirmMaintenanceBatch(c, batch.ID)
	if be != nil {
		t.Fatal(be.Msg)
	}
	if result["status"] != model.BatchConfirmed || result["confirmed"] != 2 {
		t.Fatalf("批量确认结果错误: %+v", result)
	}
	var records int64
	if err := svc.db.Model(&model.EquipmentMaintenance{}).Where("batch_registration_id = ?", batch.ID).Count(&records).Error; err != nil || records != 2 {
		t.Fatalf("生效流水数量错误: %d, %v", records, err)
	}
	var equipment model.Equipment
	if err := svc.db.First(&equipment, "type = ?", "extinguisher").Error; err != nil || equipment.LastMaintenanceDate == nil || equipment.NextDueDate == nil {
		t.Fatalf("台账未按确认流水回写: %+v, %v", equipment, err)
	}
}

func TestConfirmMaintenanceBatchConflictRollsBackAll(t *testing.T) {
	svc, c := newMaintenanceBatchTest(t)
	batch := createBatchFixture(t, svc.db, 2, true)
	if _, be := svc.ConfirmMaintenanceBatch(c, batch.ID); be == nil {
		t.Fatal("含报废设备时整单必须拒绝")
	}
	var records int64
	if err := svc.db.Model(&model.EquipmentMaintenance{}).Where("batch_registration_id = ?", batch.ID).Count(&records).Error; err != nil || records != 0 {
		t.Fatalf("冲突后不得有半批流水: %d, %v", records, err)
	}
	var equipment model.Equipment
	if err := svc.db.First(&equipment, "status = ?", model.StatusInService).Error; err != nil || equipment.LastMaintenanceDate != nil {
		t.Fatalf("冲突后不得回写任何设备日期: %+v, %v", equipment, err)
	}
	var saved model.MaintenanceBatch
	if err := svc.db.First(&saved, "id = ?", batch.ID).Error; err != nil || saved.Status != model.BatchPending {
		t.Fatalf("冲突后登记单应保持 pending: %+v, %v", saved, err)
	}
}

func TestPreviewCreateMaintenanceBatchUsesSnapshot(t *testing.T) {
	svc, c := newMaintenanceBatchTest(t)
	middleware.SetIdentity(c, &middleware.Identity{UserID: batchCreator, TenantID: batchTenant, RoleCodes: []string{sysmodel.InspectorCode}, ProjectIDs: []string{batchCommune}})
	ids := make([]string, 2)
	for i := range ids {
		equipment := model.Equipment{TenantID: ptrString(batchTenant), CommunityID: batchCommune, Type: "extinguisher", Code: string(rune('X' + i)), Name: "预览灭火器", Status: model.StatusInService}
		if err := svc.db.Create(&equipment).Error; err != nil {
			t.Fatal(err)
		}
		ids[i] = equipment.ID
	}
	req := &MaintenanceBatchPreviewReq{CommunityID: batchCommune, EquipmentType: "extinguisher", MaintenanceType: model.MaintenanceRefill,
		MaintenanceDate: time.Now().Format("2006-01-02"), IDs: ids, OperationID: "preview-operation-1"}
	req.PayloadHash = maintenanceBatchHash(req)
	preview, be := svc.PreviewMaintenanceBatch(c, req)
	if be != nil {
		t.Fatal(be.Msg)
	}
	if preview["count"] != 2 || preview["selection_token"] == "" {
		t.Fatalf("预览快照错误: %+v", preview)
	}
	created, be := svc.CreateMaintenanceBatch(c, preview["selection_token"].(string))
	if be != nil {
		t.Fatal(be.Msg)
	}
	if created["status"] != model.BatchPending || created["count"] != 2 {
		t.Fatalf("登记单创建错误: %+v", created)
	}
	again, be := svc.CreateMaintenanceBatch(c, preview["selection_token"].(string))
	if be != nil || again["id"] != created["id"] {
		t.Fatalf("重复创建未幂等返回原单: %+v, %v", again, be)
	}
	req.ExcludedIDs = []string{"not-in-snapshot"}
	if _, be := svc.PreviewMaintenanceBatch(c, req); be == nil {
		t.Fatal("伪造排除项必须拒绝")
	}
}

func ptrString(value string) *string { return &value }
