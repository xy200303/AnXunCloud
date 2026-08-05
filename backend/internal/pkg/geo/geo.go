// Package geo 提供 GCJ-02 坐标距离计算（Haversine）。
package geo

import "math"

const earthRadiusM = 6371000.0

// Haversine 计算两点球面距离（米）。
func Haversine(lng1, lat1, lng2, lat2 float64) float64 {
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
