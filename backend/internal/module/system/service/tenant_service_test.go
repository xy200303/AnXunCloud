package service

import "testing"

// TestIsTenantConfigKey 租户可覆盖配置白名单：品牌类 key 放行，密钥类与非白名单 key 拒绝。
func TestIsTenantConfigKey(t *testing.T) {
	allowed := []string{
		"site.company_name", "site.slogan", "site.theme_color",
		"site.footer_note", "site.contact_phone", "site.contact_email",
	}
	for _, k := range allowed {
		if !IsTenantConfigKey(k) {
			t.Errorf("白名单 key 应放行：%s", k)
		}
	}
	// 密钥类配置永不下放租户（设计方案 §9.2）
	denied := []string{
		"ai.api_key", "ai.base_url", "ai.model",
		"security.login_fail_limit", "auth.register_enabled",
		"site.icp", "", "site.theme_color2",
	}
	for _, k := range denied {
		if IsTenantConfigKey(k) {
			t.Errorf("非白名单 key 应拒绝：%s", k)
		}
	}
}
