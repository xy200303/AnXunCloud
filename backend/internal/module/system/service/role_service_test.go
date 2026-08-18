package service

import (
	"testing"

	"anxuncloud/internal/module/system/model"
)

// TestShouldFilterPlatform 平台级菜单（is_platform）剔除判定：
// 仅 super_admin 可持有平台级菜单（其绑定由 seed 维护、接口禁止改动），
// 其余任何角色（租户自建/内置 tenant_admin/内置共享角色）经接口分配一律剔除。
func TestShouldFilterPlatform(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{model.SuperAdminCode, false},
		{model.TenantAdminCode, true},
		{"project_admin", true},
		{"custom_role", true},
		{"", true},
	}
	for _, tc := range cases {
		if got := shouldFilterPlatform(tc.code); got != tc.want {
			t.Errorf("code=%q：shouldFilterPlatform = %v，期望 %v", tc.code, got, tc.want)
		}
	}
}
