package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
)

const (
	datascopeTenantA    = "11111111-1111-1111-1111-111111111111"
	datascopeTenantB    = "22222222-2222-2222-2222-222222222222"
	datascopeCommunityA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	datascopeCommunityB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func newDataScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	logger.L = zap.NewNop()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&sysmodel.Tenant{}, &sysmodel.Community{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	tenants := []sysmodel.Tenant{
		{Code: "tenant-a", Name: "租户A", Status: sysmodel.StatusEnabled},
		{Code: "tenant-b", Name: "租户B", Status: sysmodel.StatusEnabled},
	}
	for i := range tenants {
		tenant := &tenants[i]
		tenant.ID = []string{datascopeTenantA, datascopeTenantB}[i]
		if err := db.Create(tenant).Error; err != nil {
			t.Fatalf("创建租户失败: %v", err)
		}
	}
	communities := []sysmodel.Community{
		{TenantID: datascopeTenantA, Name: "小区A", Status: sysmodel.StatusEnabled},
		{TenantID: datascopeTenantB, Name: "小区B", Status: sysmodel.StatusEnabled},
	}
	communities[0].ID = datascopeCommunityA
	communities[1].ID = datascopeCommunityB
	for _, community := range communities {
		if err := db.Create(&community).Error; err != nil {
			t.Fatalf("创建小区失败: %v", err)
		}
	}
	return db
}

func superAdminContext(path string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", path, nil)
	c.Set(ctxIdentity, &Identity{UserID: "super", SuperAdmin: true})
	return c
}

func TestSuperAdminCommunityScopeWithoutTenantContext(t *testing.T) {
	db := newDataScopeTestDB(t)
	c := superAdminContext("/api/app/communities/tree")

	var count int64
	ApplyCommunityFilter(db.Model(&sysmodel.Community{}), c, "id").Count(&count)
	if count != 2 {
		t.Fatalf("超级管理员未指定租户时应看到全部小区，实际 %d", count)
	}
	if be := CheckCommunity(db, c, datascopeCommunityA); be != nil {
		t.Fatalf("超级管理员未指定租户时访问租户A小区不应被拒绝: %v", be)
	}
	if be := CheckCommunity(db, c, datascopeCommunityB); be != nil {
		t.Fatalf("超级管理员未指定租户时访问租户B小区不应被拒绝: %v", be)
	}
}

func TestSuperAdminCommunityScopeWithExplicitTenant(t *testing.T) {
	db := newDataScopeTestDB(t)
	c := superAdminContext("/api/app/communities/tree?tenant_id=" + datascopeTenantA)

	var count int64
	ApplyCommunityFilter(db.Model(&sysmodel.Community{}), c, "id").Count(&count)
	if count != 1 {
		t.Fatalf("超级管理员指定租户时应只看到目标租户小区，实际 %d", count)
	}
	if be := CheckCommunity(db, c, datascopeCommunityB); be == nil {
		t.Fatal("超级管理员指定租户A时不应访问租户B小区")
	}
}

func TestNonSuperAdminDefaultTenantCannotCrossTenant(t *testing.T) {
	db := newDataScopeTestDB(t)
	c := superAdminContext("/api/app/communities/tree")
	c.Set(ctxIdentity, &Identity{
		UserID:       "tenant-admin",
		TenantID:     datascopeTenantA,
		DataScopeAll: true,
	})

	var count int64
	ApplyCommunityFilter(db.Model(&sysmodel.Community{}), c, "id").Count(&count)
	if count != 1 {
		t.Fatalf("普通角色的 all 数据范围应仅限本人租户，实际可见 %d 个小区", count)
	}
	if be := CheckCommunity(db, c, datascopeCommunityB); be == nil {
		t.Fatal("普通角色不应访问其他租户小区")
	}
}
