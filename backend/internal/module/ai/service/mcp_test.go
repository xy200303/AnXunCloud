package service

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/module/ai/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	inspectionsvc "anxuncloud/internal/module/inspection/service"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/authz"
)

func TestMCPTokenLifecycleAndDataScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.MCPToken{}, &sysmodel.SysUser{}, &sysmodel.SysRole{}, &sysmodel.SysMenu{}, &sysmodel.SysRoleMenu{}, &sysmodel.Tenant{}, &sysmodel.Community{}, &insmodel.InspectionTask{}, &insmodel.CheckinRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE casbin_rule (id INTEGER PRIMARY KEY AUTOINCREMENT, ptype VARCHAR(100), v0 VARCHAR(100), v1 VARCHAR(100), v2 VARCHAR(100), v3 VARCHAR(100), v4 VARCHAR(100), v5 VARCHAR(100))").Error; err != nil {
		t.Fatal(err)
	}
	tenant := sysmodel.Tenant{Code: "tenant-a", Name: "租户 A", Status: sysmodel.StatusEnabled}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	community := sysmodel.Community{TenantID: tenant.ID, Name: "测试小区", Status: sysmodel.StatusEnabled}
	if err := db.Create(&community).Error; err != nil {
		t.Fatal(err)
	}
	role := sysmodel.SysRole{Code: "tenant_admin", Name: "租户管理员", ScopeLevel: model.ScopeTenant, Status: sysmodel.StatusEnabled}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	user := sysmodel.SysUser{TenantID: tenant.ID, Username: "mcp-user", Name: "MCP 用户", RoleID: &role.ID, Status: sysmodel.StatusEnabled}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	menu := sysmodel.SysMenu{Title: "任务监控", Type: sysmodel.MenuTypeMenu, Perms: "inspection:task:monitor", Status: sysmodel.StatusEnabled}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sysmodel.SysRoleMenu{RoleID: role.ID, MenuID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := authz.Init(db); err != nil {
		t.Fatal(err)
	}
	if err := authz.SyncAll(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&insmodel.InspectionTask{TenantID: &tenant.ID, CommunityID: community.ID, TaskDate: time.Now(), Status: "pending"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewMCPService(db, inspectionsvc.NewTaskService(db, nil, nil).SummaryForAI)
	actor := Actor{TenantID: tenant.ID, UserID: user.ID}
	issued, be := svc.Issue(actor, "本地只读", []string{"inspection.summary"}, 7)
	if be != nil {
		t.Fatal(be.Msg)
	}
	secret := issued["token"].(string)
	if secret == issued["prefix"] || len(secret) != 64 {
		t.Fatal("unexpected token format")
	}
	result, be := svc.Call(secret, "inspection.summary", map[string]any{})
	if be != nil {
		t.Fatal(be.Msg)
	}
	if result.(gin.H)["task_total"] != int64(1) {
		t.Fatalf("unexpected summary: %#v", result)
	}
	var token model.MCPToken
	if err := db.First(&token, "id = ?", issued["id"]).Error; err != nil {
		t.Fatal(err)
	}
	if token.SecretHash == secret || token.Prefix == secret {
		t.Fatal("raw token must not be persisted")
	}
	if be := svc.Revoke(actor, token.ID); be != nil {
		t.Fatal(be.Msg)
	}
	if _, be := svc.Call(secret, "inspection.summary", map[string]any{}); be == nil {
		t.Fatal("revoked token must be rejected immediately")
	}
}
