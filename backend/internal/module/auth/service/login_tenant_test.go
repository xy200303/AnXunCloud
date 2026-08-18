package service

import (
	"testing"

	"anxuncloud/internal/module/system/model"
)

// TestDistinctTenantIDs 登录候选用户租户去重：同名账号命中多租户时触发公司代码消歧。
func TestDistinctTenantIDs(t *testing.T) {
	users := []model.SysUser{
		{TenantID: "t1", Username: "admin"},
		{TenantID: "t1", Username: "admin"}, // 同租户重复（理论被唯一索引拦截，防御）
		{TenantID: "t2", Username: "admin"},
	}
	ids := distinctTenantIDs(users)
	if len(ids) != 2 {
		t.Fatalf("期望 2 个租户，实际 %d", len(ids))
	}

	single := distinctTenantIDs([]model.SysUser{{TenantID: "t1", Username: "admin"}})
	if len(single) != 1 || single[0] != "t1" {
		t.Fatalf("单租户期望 [t1]，实际 %v", single)
	}

	if ids := distinctTenantIDs(nil); len(ids) != 0 {
		t.Fatalf("空候选期望 0 个租户，实际 %v", ids)
	}
}

// TestFilterUsersByTenant 按租户过滤候选用户（公司代码消歧后应只剩该租户账号）。
func TestFilterUsersByTenant(t *testing.T) {
	users := []model.SysUser{
		{TenantID: "t1", Username: "u1"},
		{TenantID: "t2", Username: "u2"},
		{TenantID: "t3", Username: "u3"},
	}
	got := filterUsersByTenant(users, "t2")
	if len(got) != 1 || got[0].Username != "u2" {
		t.Fatalf("期望只命中 u2，实际 %+v", got)
	}
	if got := filterUsersByTenant(users, "t9"); len(got) != 0 {
		t.Fatalf("不存在租户期望空结果，实际 %+v", got)
	}
}
