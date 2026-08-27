package service

import (
	"sort"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/geo"
	"anxuncloud/internal/pkg/types"

	"gorm.io/gorm"
)

// 路线排序：楼栋聚类 + 楼栋间最近邻 + 楼内 单元升序→楼层降序（自上而下）。
// 楼栋之间按楼栋质心（楼栋内点位坐标均值）做最近邻排序，从名单第一个点位所在楼栋出发，
// 避免出现「1 栋没检完下一点位跳到 3 栋」的折返；楼内按单元分组、每个单元从顶楼往下走，
// 符合巡检员「坐电梯到顶楼一路下楼」的作业习惯。无坐标的分组排在最后，保持原相对顺序。

type routePoint struct {
	id       string
	building string // 空 = 未分区
	unit     int    // 单元号（0 = 未设置）
	floor    *int   // 楼层（nil = 未设置；负数 = 地下层）
	lng      float64
	lat      float64
	hasGeo   bool
}

type routeGroup struct {
	key     string
	ids     []string
	sumLng  float64
	sumLat  float64
	geoCnt  int
	lng     float64 // 质心
	lat     float64
	hasGeo  bool
	visited bool
}

// OrderPointsByRoute 按巡检路线重排点位名单（查库取楼栋与坐标后走纯函数 orderByRoute）。
func OrderPointsByRoute(db *gorm.DB, ids types.IDArray) types.IDArray {
	if len(ids) < 3 {
		return ids
	}
	var pts []insmodel.InspectionPoint
	if err := db.Select("id", "building_id", "unit_no", "floor", "longitude", "latitude").
		Where("id IN ?", []string(ids)).Find(&pts).Error; err != nil {
		return ids
	}
	info := make(map[string]routePoint, len(pts))
	for _, p := range pts {
		b := ""
		if p.BuildingID != nil {
			b = *p.BuildingID
		}
		u := 0
		if p.UnitNo != nil {
			u = *p.UnitNo
		}
		info[p.ID] = routePoint{
			id: p.ID, building: b, unit: u, floor: p.Floor,
			lng: p.Longitude, lat: p.Latitude,
			hasGeo: p.Longitude != 0 || p.Latitude != 0,
		}
	}
	return orderByRoute(ids, info)
}

// orderByRoute 纯函数：ids 为原顺序名单，info 为点位地理信息（缺失按无坐标未分区处理）。
func orderByRoute(ids types.IDArray, info map[string]routePoint) types.IDArray {
	if len(ids) < 3 {
		return ids
	}
	// 按楼栋分组，保持楼栋首次出现顺序
	groups := make([]*routeGroup, 0, len(ids))
	gIdx := make(map[string]*routeGroup)
	for _, id := range ids {
		inf := info[id] // 缺失得零值：未分区 + 无坐标
		g := gIdx[inf.building]
		if g == nil {
			g = &routeGroup{key: inf.building}
			gIdx[inf.building] = g
			groups = append(groups, g)
		}
		g.ids = append(g.ids, id)
		if inf.hasGeo {
			g.sumLng += inf.lng
			g.sumLat += inf.lat
			g.geoCnt++
		}
	}
	if len(groups) < 2 {
		// 单一分组（全同楼栋/全未分区）：楼内排序依然生效
		return sortGroupItems([]string(ids), info)
	}
	for _, g := range groups {
		if g.geoCnt > 0 {
			g.lng = g.sumLng / float64(g.geoCnt)
			g.lat = g.sumLat / float64(g.geoCnt)
			g.hasGeo = true
		}
	}
	// 楼栋间最近邻（组数很小，O(n²) 足够）：有坐标时选距离最近的未访问组，
	// 当前组或无坐标候选不参与比较时保持原相对顺序兜底。
	ordered := make([]*routeGroup, 0, len(groups))
	cur := groups[0]
	for len(ordered) < len(groups) {
		cur.visited = true
		ordered = append(ordered, cur)
		var next *routeGroup
		best := 0.0
		for _, g := range groups {
			if g.visited {
				continue
			}
			if next == nil {
				next = g // 兜底：第一个未访问组
				if cur.hasGeo && g.hasGeo {
					best = geo.Haversine(cur.lng, cur.lat, g.lng, g.lat)
				}
				continue
			}
			if !cur.hasGeo || !g.hasGeo {
				continue
			}
			d := geo.Haversine(cur.lng, cur.lat, g.lng, g.lat)
			if !next.hasGeo || d < best {
				next = g
				best = d
			}
		}
		if next == nil {
			break
		}
		cur = next
	}
	out := make(types.IDArray, 0, len(ids))
	for _, g := range ordered {
		out = append(out, sortGroupItems(g.ids, info)...)
	}
	return out
}

// sortGroupItems 楼内点位排序：单元号升序 → 楼层降序（自上而下，巡检员坐电梯到顶楼一路往下走）。
// 未设置单元/楼层的点位排在最后并保持原相对顺序（stable）。
func sortGroupItems(ids []string, info map[string]routePoint) []string {
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := info[ids[i]], info[ids[j]]
		au, bu := unitKey(a.unit), unitKey(b.unit)
		if au != bu {
			return au < bu
		}
		return floorKey(a.floor) < floorKey(b.floor)
	})
	return ids
}

// unitKey 单元排序键：升序；0（未设置）排最后。
func unitKey(u int) int {
	if u <= 0 {
		return 1 << 29
	}
	return u
}

// floorKey 楼层排序键：降序（楼层越大越靠前）；nil 排最后。
func floorKey(f *int) int {
	if f == nil {
		return 1 << 30
	}
	return -(*f)
}
