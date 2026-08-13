package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"anxuncloud/internal/pkg/errs"
)

// PlaceSuggestion 地点搜索结果（腾讯地点提示接口的精简视图）。
type PlaceSuggestion struct {
	Title   string  `json:"title"`
	Address string  `json:"address"`
	Lng     float64 `json:"lng"`
	Lat     float64 `json:"lat"`
}

// tencentSuggestionResp 腾讯 /ws/place/v1/suggestion 响应结构（只取需要字段）。
type tencentSuggestionResp struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    []struct {
		Title    string `json:"title"`
		Address  string `json:"address"`
		Location struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
	} `json:"data"`
}

// mapHTTPClient 腾讯 WebService 调用客户端（超时兜底，避免拖死请求）。
var mapHTTPClient = &http.Client{Timeout: 8 * time.Second}

// SearchPlaces 代理腾讯地点提示 API：按关键字搜索候选位置。
// location 为可选的偏向坐标（"lat,lng"，传当前地图中心可让结果优先排在附近）。
// key 优先取 map.tencent_ws_key（服务端 WebService 专用，无域名限制），未配置回退 map.tencent_key。
func (s *ConfigService) SearchPlaces(ctx context.Context, keyword, location string) ([]PlaceSuggestion, *errs.Error) {
	key, _ := s.Get("map.tencent_ws_key")
	if key == "" {
		key, _ = s.Get("map.tencent_key")
	}
	if key == "" {
		return nil, errs.ErrParam.WithMsg("腾讯地图 Key 未配置，请先在系统管理-参数配置填写 map.tencent_key")
	}
	q := url.Values{}
	q.Set("keyword", keyword)
	q.Set("key", key)
	if location != "" {
		q.Set("location", location)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://apis.map.qq.com/ws/place/v1/suggestion?"+q.Encode(), nil)
	if err != nil {
		return nil, errs.ErrInternal
	}
	resp, err := mapHTTPClient.Do(req)
	if err != nil {
		return nil, errs.ErrInternal.WithMsg("地图服务请求失败：" + err.Error())
	}
	defer resp.Body.Close()
	var tr tencentSuggestionResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, errs.ErrInternal.WithMsg("地图服务响应解析失败")
	}
	if tr.Status != 0 {
		return nil, errs.ErrInternal.WithMsg(fmt.Sprintf("地图服务错误(%d)：%s", tr.Status, tr.Message))
	}
	out := make([]PlaceSuggestion, 0, len(tr.Data))
	for _, d := range tr.Data {
		if d.Location.Lat == 0 && d.Location.Lng == 0 {
			continue // 无坐标的区划类结果对选点无意义
		}
		out = append(out, PlaceSuggestion{
			Title: d.Title, Address: d.Address, Lng: d.Location.Lng, Lat: d.Location.Lat,
		})
	}
	return out, nil
}
