package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
)

func TestRoleCandidatesAndNaturalPersonExclusion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&sysmodel.SysRole{}, &sysmodel.SysUser{}, &sysmodel.ProjectStaff{}, &sysmodel.Community{}); err != nil {
		t.Fatal(err)
	}
	roles := []sysmodel.SysRole{
		{Code: sysmodel.QualitySupervisorCode, Name: "品质主管", ScopeLevel: sysmodel.ScopeProject, IsSupervisor: true, Status: sysmodel.StatusEnabled},
		{Code: sysmodel.TenantAdminCode, Name: "租户管理员", ScopeLevel: sysmodel.ScopeTenant, Status: sysmodel.StatusEnabled},
	}
	for i := range roles {
		roles[i].ID = []string{"10000000-0000-0000-0000-000000000001", "10000000-0000-0000-0000-000000000002"}[i]
		if err := db.Create(&roles[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	tenantID := "20000000-0000-0000-0000-000000000001"
	communityA := "30000000-0000-0000-0000-000000000001"
	communityB := "30000000-0000-0000-0000-000000000002"
	for _, id := range []string{communityA, communityB} {
		community := sysmodel.Community{TenantID: tenantID, Name: id, Status: sysmodel.StatusEnabled}
		community.ID = id
		if err := db.Create(&community).Error; err != nil {
			t.Fatal(err)
		}
	}
	users := []sysmodel.SysUser{
		{TenantID: tenantID, Username: "supervisor", Name: "主管", PersonKey: "person-a", RoleID: &roles[0].ID, Status: sysmodel.StatusEnabled},
		{TenantID: tenantID, Username: "other-account", Name: "另一账号", PersonKey: "person-a", RoleID: &roles[0].ID, Status: sysmodel.StatusEnabled},
		{TenantID: tenantID, Username: "tenant-admin", Name: "公司管理员", PersonKey: "person-b", RoleID: &roles[1].ID, Status: sysmodel.StatusEnabled},
	}
	for i := range users {
		users[i].ID = []string{"40000000-0000-0000-0000-000000000001", "40000000-0000-0000-0000-000000000002", "40000000-0000-0000-0000-000000000003"}[i]
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	expired := time.Now().Add(-time.Hour)
	staff := []sysmodel.ProjectStaff{
		{TenantID: &tenantID, ProjectID: communityA, UserID: users[0].ID, Status: sysmodel.StatusEnabled},
		{TenantID: &tenantID, ProjectID: communityA, UserID: users[1].ID, Status: sysmodel.StatusEnabled},
		{TenantID: &tenantID, ProjectID: communityB, UserID: users[0].ID, Status: sysmodel.StatusEnabled, EffectiveTo: &expired},
	}
	for i := range staff {
		if err := db.Create(&staff[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	ids := RoleCandidateUserIDs(db, []string{communityA, communityB}, sysmodel.QualitySupervisorCode, "")
	if len(ids) != 2 {
		t.Fatalf("预期两名有效候选人，实际 %v", ids)
	}
	ids = RoleCandidateUserIDs(db, []string{communityA}, sysmodel.QualitySupervisorCode, "person-a")
	if len(ids) != 0 {
		t.Fatalf("同一自然人的关联账号必须全部排除，实际 %v", ids)
	}
	ids = RoleCandidateUserIDs(db, []string{communityB}, sysmodel.TenantAdminCode, "")
	if len(ids) != 1 || ids[0] != users[2].ID {
		t.Fatalf("跨小区租户级候选人解析错误: %v", ids)
	}
	if RoleCandidateAuthorized(db, []string{communityB}, sysmodel.QualitySupervisorCode, &middleware.Identity{UserID: users[0].ID, SuperAdmin: true}) {
		t.Fatal("过期任职不能因管理员身份重新获得候选人资格")
	}
	if !SameNaturalPerson(db, users[0].ID, users[1].ID) || SameNaturalPerson(db, users[0].ID, users[2].ID) {
		t.Fatal("自然人关联账号识别错误")
	}
}

func TestWalkFlowWithVotersBlocksEmptyCandidates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&sysmodel.SysRole{}, &sysmodel.SysUser{}, &sysmodel.ProjectStaff{}, &sysmodel.Community{}); err != nil {
		t.Fatal(err)
	}
	walk := WalkFlowWithVoters(db, "30000000-0000-0000-0000-000000000001", DefaultCheckinReviewFlow(), 0, "", false)
	if walk.Finish || !walk.Blocked {
		t.Fatalf("空候选必须挂起，实际 %+v", walk)
	}
}
