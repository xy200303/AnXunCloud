package service

import "testing"

// TestCheckItemDisposition 异常项处置方式校验（纯函数）：
// 白名单 ''/on_site_resolved/maintenance_registered/report_pending；仅异常（!pass）项可填；
// on_site_resolved 必须带 ≥1 张处置照片。
func TestCheckItemDisposition(t *testing.T) {
	cases := []struct {
		name        string
		disposition string
		pass        bool
		resPhotos   int
		wantErr     bool
	}{
		{"空值正常项通过", "", true, 0, false},
		{"空值异常项通过", "", false, 0, false},
		{"现场已处理带照片通过", "on_site_resolved", false, 1, false},
		{"现场已处理无照片拒绝", "on_site_resolved", false, 0, true},
		{"现场已处理正常项拒绝", "on_site_resolved", true, 1, true},
		{"登记维保异常项通过", "maintenance_registered", false, 0, false},
		{"上报待处理异常项通过", "report_pending", false, 0, false},
		{"上报待处理正常项拒绝", "report_pending", true, 0, true},
		{"非法取值拒绝", "bad_value", false, 0, true},
	}
	for _, c := range cases {
		got := checkItemDisposition(c.disposition, c.pass, c.resPhotos)
		if (got != "") != c.wantErr {
			t.Errorf("%s: got %q, wantErr=%v", c.name, got, c.wantErr)
		}
	}
}
