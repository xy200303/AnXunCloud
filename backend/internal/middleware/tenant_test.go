package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 平台级菜单判定已由 IsPlatformPerms（perms 前缀）改为菜单行 is_platform 列（见迁移 00023），
// 此处改测租户上下文助手 EffectiveTenantID 的免库分支。

// TestEffectiveTenantIDNonSuper 非超管：固定本人租户，请求参数一律忽略（防越权切换上下文）。
func TestEffectiveTenantIDNonSuper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/admin/system/users?tenant_id=other-tenant", nil)
	c.Set(ctxIdentity, &Identity{UserID: "u1", TenantID: "my-tenant"})
	tid, be := EffectiveTenantID(c, nil) // 非超管分支不触库，db 传 nil
	if be != nil {
		t.Fatalf("非超管解析不应报错: %v", be)
	}
	if tid != "my-tenant" {
		t.Fatalf("非超管应固定本人租户，得到 %s", tid)
	}
}

// TestEffectiveTenantIDNoIdentity 未登录：返回 401。
func TestEffectiveTenantIDNoIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	if _, be := EffectiveTenantID(c, nil); be == nil {
		t.Fatal("未登录应返回错误")
	}
}
