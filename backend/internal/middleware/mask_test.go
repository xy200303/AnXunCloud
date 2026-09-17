package middleware

import "testing"

// TestMaskParamsSensitiveKeys 通用敏感键脱敏：password/secret/api_key/apikey/token/credential（大小写不敏感）。
func TestMaskParamsSensitiveKeys(t *testing.T) {
	in := `{"password":"p1","New_Secret":"s2","api_key":"k3","APIKEY":"k4","access_Token":"t5","Credential":"c6","name":"n7"}`
	out := maskParams("/api/admin/system/users", []byte(in))
	want := `{"password":"******","New_Secret":"******","api_key":"******","APIKEY":"******","access_Token":"******","Credential":"******","name":"n7"}`
	if out != want {
		t.Fatalf("敏感键脱敏不符预期:\n got: %s\nwant: %s", out, want)
	}
}

// TestMaskParamsSystemConfigValue /system/configs 的 value 键一律打码（AI key 等密钥不落日志），key 保留。
func TestMaskParamsSystemConfigValue(t *testing.T) {
	in := `{"value":"sk-abc123","key":"ai.api_key"}`
	out := maskParams("/api/admin/system/configs/cfg1", []byte(in))
	want := `{"value":"******","key":"ai.api_key"}`
	if out != want {
		t.Fatalf("系统配置 value 脱敏不符预期:\n got: %s\nwant: %s", out, want)
	}
}

// TestMaskParamsValueKeptOutsideConfigs 非系统配置路径的 value 键不打码（避免误伤普通业务参数）。
func TestMaskParamsValueKeptOutsideConfigs(t *testing.T) {
	in := `{"value":"v1","key":"k1"}`
	out := maskParams("/api/admin/system/dict-data", []byte(in))
	if out != in {
		t.Fatalf("非 configs 路径 value 不应打码:\n got: %s\nwant: %s", out, in)
	}
}
