package service

import (
	"testing"

	insmodel "anxuncloud/internal/module/inspection/model"
)

func TestNormalizeLocText(t *testing.T) {
	// 全角数字/字母转半角、去空白（含全角空格）
	if got := normalizeLocText("１栋　２F"); got != "1栋2F" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeLocText("1 栋 2 层"); got != "1栋2层" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDeviceLocation(t *testing.T) {
	cases := []struct {
		name  string
		texts []string // 安装位置/管控区域/设备名称/机房名称
		want  string
		ok    bool
	}{
		{"设备名称解析", []string{"", "", "消火栓1栋1F-001", ""}, "b1|f1", true},
		{"管控区域", []string{"", "1栋4层", "灭火器1栋4F-002", ""}, "b1|f4", true},
		{"安装位置优先", []string{"2栋3层", "1栋4层", "", ""}, "b2|f3", true},
		{"车库负一层", []string{"车库负1层", "", "", ""}, "b车库|f-1", true},
		{"架空层", []string{"1栋架空层", "", "", ""}, "b1|f0", true},
		{"范围楼栋保守跳过", []string{"1-5栋楼道", "", "", ""}, "", false}, // 楼栋只能取到 5栋 且无楼层
		{"全空", []string{"", "", "", ""}, "", false},
		{"只有楼栋无楼层", []string{"11栋", "", "", ""}, "", false},
		{"机房名称兜底", []string{"", "", "", "2栋B1层水泵房"}, "b2|f-1", true},
		{"全角", []string{"３栋５楼", "", "", ""}, "b3|f5", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseDeviceLocation(tc.texts)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseDeviceLocation(%v) = %q,%v want %q,%v", tc.texts, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestParseBuildingKey(t *testing.T) {
	if got := parseBuildingKey("11栋"); got != "b11" {
		t.Fatalf("11栋 → %q", got)
	}
	if got := parseBuildingKey("1号楼"); got != "b1" {
		t.Fatalf("1号楼 → %q", got)
	}
	if got := parseBuildingKey("地下车库"); got != "b车库" {
		t.Fatalf("地下车库 → %q", got)
	}
	if got := parseBuildingKey("商铺"); got != "" {
		t.Fatalf("商铺 → %q", got)
	}
	if got := parseBuildingKey("01栋"); got != "b1" {
		t.Fatalf("01栋 → %q", got)
	}
}

func TestPointBindIndex(t *testing.T) {
	mkBuilding := func(id, name string) insmodel.Building {
		var b insmodel.Building
		b.ID = id
		b.Name = name
		return b
	}
	mkPoint := func(id, name string, buildingID *string) insmodel.InspectionPoint {
		var p insmodel.InspectionPoint
		p.ID = id
		p.Name = name
		p.BuildingID = buildingID
		return p
	}
	b1, b2 := "b-1", "b-2"
	buildings := []insmodel.Building{mkBuilding(b1, "1栋"), mkBuilding(b2, "2栋")}
	points := []insmodel.InspectionPoint{
		mkPoint("p-1-1", "1栋1楼大厅消火栓", &b1),
		mkPoint("p-1-2", "1栋2楼通道消火栓", &b1),
		mkPoint("p-2-1", "2栋1楼大厅消火栓", &b2),
		mkPoint("p-2-1b", "2栋1楼通道消火栓", &b2), // 与 p-2-1 同键 → 歧义
		mkPoint("p-no-b", "3栋5楼消火栓", nil),    // 无楼栋信息时从点位名解析
	}
	idx := buildPointBindIndex(buildings, points)

	// 唯一匹配才绑
	c := idx.lookup("b1|f1")
	if c == nil || c.pointID != "p-1-1" || c.buildingID == nil || *c.buildingID != b1 {
		t.Fatalf("b1|f1 应唯一命中 p-1-1: %+v", c)
	}
	// 歧义跳过
	if c := idx.lookup("b2|f1"); c != nil {
		t.Fatalf("b2|f1 有两个候选应跳过: %+v", c)
	}
	// 无对应跳过
	if c := idx.lookup("b9|f9"); c != nil {
		t.Fatalf("b9|f9 无对应应跳过: %+v", c)
	}
	// 无楼栋点位从点位名解析
	c = idx.lookup("b3|f5")
	if c == nil || c.pointID != "p-no-b" || c.buildingID != nil {
		t.Fatalf("b3|f5 应命中无楼栋点位: %+v", c)
	}
	// "11栋"不被"1栋"误匹配：b1 键不会命中 11栋 的点位
	points2 := []insmodel.InspectionPoint{mkPoint("p-11", "11栋1楼消火栓", nil)}
	idx2 := buildPointBindIndex(nil, points2)
	if c := idx2.lookup("b1|f1"); c != nil {
		t.Fatalf("11栋 的点位不应被 b1 键命中: %+v", c)
	}
	if c := idx2.lookup("b11|f1"); c == nil || c.pointID != "p-11" {
		t.Fatalf("b11|f1 应命中: %+v", c)
	}
}
