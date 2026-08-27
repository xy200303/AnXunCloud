package service

import (
	"testing"

	"anxuncloud/internal/pkg/types"
)

func rp(id, building string, lng, lat float64) routePoint {
	return routePoint{id: id, building: building, lng: lng, lat: lat, hasGeo: lng != 0 || lat != 0}
}

func rpUF(id, building string, unit int, floor int, lng, lat float64) routePoint {
	p := rp(id, building, lng, lat)
	p.unit = unit
	p.floor = &floor
	return p
}

// 楼栋交错名单 → 同楼栋连续 + 楼栋间最近邻
func TestOrderByRoute_GroupsByBuilding(t *testing.T) {
	ids := types.IDArray{"a1", "b1", "a2", "c1", "a3", "b2"}
	info := map[string]routePoint{
		"a1": rp("a1", "1栋", 114.0000, 30.0000),
		"a2": rp("a2", "1栋", 114.0001, 30.0001),
		"a3": rp("a3", "1栋", 114.0002, 30.0000),
		"b1": rp("b1", "2栋", 114.0010, 30.0000), // 距 1 栋约 1 百米
		"b2": rp("b2", "2栋", 114.0011, 30.0001),
		"c1": rp("c1", "3栋", 114.0200, 30.0000), // 距 1 栋约 2 公里
	}
	got := orderByRoute(ids, info)
	want := types.IDArray{"a1", "a2", "a3", "b1", "b2", "c1"}
	if len(got) != len(want) {
		t.Fatalf("长度不符: got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("顺序不符: got %v want %v", got, want)
		}
	}
}

// 起点楼栋为名单第一个点位所在楼栋
func TestOrderByRoute_StartsFromFirstGroup(t *testing.T) {
	ids := types.IDArray{"c1", "a1", "b1"}
	info := map[string]routePoint{
		"a1": rp("a1", "1栋", 114.0000, 30.0000),
		"b1": rp("b1", "2栋", 114.0010, 30.0000),
		"c1": rp("c1", "3栋", 114.0005, 30.0000), // 3 栋其实在 1、2 栋之间
	}
	got := orderByRoute(ids, info)
	// 从 3 栋出发：1 栋（50m）比 2 栋（约 55m… 以实际距离为准，此处 2 栋更远）更近 → 3→1→2
	want := types.IDArray{"c1", "a1", "b1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("顺序不符: got %v want %v", got, want)
		}
	}
}

// 无坐标点位：分组排最后且保持原相对顺序
func TestOrderByRoute_NoGeoFallback(t *testing.T) {
	ids := types.IDArray{"x1", "a1", "x2", "b1"}
	info := map[string]routePoint{
		"x1": rp("x1", "", 0, 0),
		"x2": rp("x2", "", 0, 0),
		"a1": rp("a1", "1栋", 114.0000, 30.0000),
		"b1": rp("b1", "2栋", 114.0010, 30.0000),
	}
	got := orderByRoute(ids, info)
	// 起点是未分区组（名单首位），之后 1 栋 → 2 栋
	want := types.IDArray{"x1", "x2", "a1", "b1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("顺序不符: got %v want %v", got, want)
		}
	}
}

// 少于 3 个点位或全部同楼栋：保持原顺序
func TestOrderByRoute_Trivial(t *testing.T) {
	ids := types.IDArray{"a1", "a2"}
	info := map[string]routePoint{"a1": rp("a1", "1栋", 1, 1), "a2": rp("a2", "1栋", 1, 1)}
	got := orderByRoute(ids, info)
	if got[0] != "a1" || got[1] != "a2" {
		t.Fatalf("应保持原顺序: got %v", got)
	}

	ids2 := types.IDArray{"a2", "a1", "a3"}
	got2 := orderByRoute(ids2, info)
	if got2[0] != "a2" || got2[1] != "a1" {
		t.Fatalf("同楼栋应保持原顺序: got %v", got2)
	}
}

// 楼内：单元升序 → 楼层降序（顶楼往下）；未设楼层排最后
func TestOrderByRoute_InBuildingUnitFloor(t *testing.T) {
	ids := types.IDArray{"u1f1", "u2f3", "u1f5", "u1fB1", "u1fNone", "u2f1"}
	b1 := -1
	info := map[string]routePoint{
		"u1f1":    rpUF("u1f1", "1栋", 1, 1, 114.0, 30.0),
		"u1f5":    rpUF("u1f5", "1栋", 1, 5, 114.0, 30.0),
		"u1fB1":   {id: "u1fB1", building: "1栋", unit: 1, floor: &b1, lng: 114.0, lat: 30.0, hasGeo: true},
		"u1fNone": rp("u1fNone", "1栋", 114.0, 30.0),
		"u2f3":    rpUF("u2f3", "1栋", 2, 3, 114.0, 30.0),
		"u2f1":    rpUF("u2f1", "1栋", 2, 1, 114.0, 30.0),
	}
	got := orderByRoute(ids, info)
	// 单元 1（5→1→B1）→ 单元 2（3→1）→ 未设单元/楼层排最后
	want := []string{"u1f5", "u1f1", "u1fB1", "u2f3", "u2f1", "u1fNone"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("楼内顺序不符: got %v want %v", got, want)
		}
	}
}
