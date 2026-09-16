package service

import (
	"testing"
	"time"

	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/types"
)

func spotDay(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, time.Local)
	return t
}

func spotCfg(ratio, fixed int, strategy string, noRepeat bool) model.SpotConfig {
	return model.SpotConfig{RatioPercent: ratio, FixedCount: fixed, Strategy: strategy, NoRepeat: noRepeat, DueDay: -1}
}

// TestSpotConfigOf 抽查配置解析：缺省全默认值（20% / longest_unseen / 连中保护开 / 期限月末），非法 strategy 回落默认。
func TestSpotConfigOf(t *testing.T) {
	def := model.SpotConfigOf(nil)
	if def.RatioPercent != 20 || def.Strategy != model.SpotStrategyLongestUnseen || !def.NoRepeat || def.DueDay != -1 {
		t.Fatalf("默认值不符: %+v", def)
	}
	cfg := model.SpotConfigOf(types.JSONMap{
		"ratio_percent": float64(30), "fixed_count": float64(5),
		"strategy": "random", "no_repeat": false, "due_day": float64(15),
	})
	if cfg.RatioPercent != 30 || cfg.FixedCount != 5 || cfg.Strategy != model.SpotStrategyRandom || cfg.NoRepeat || cfg.DueDay != 15 {
		t.Fatalf("解析不符: %+v", cfg)
	}
	if got := model.SpotConfigOf(types.JSONMap{"strategy": "bogus"}); got.Strategy != model.SpotStrategyLongestUnseen {
		t.Fatalf("非法 strategy 应回落默认: %+v", got)
	}
}

// TestSpotConfigDueDate 完成期限：due_day 落月内取该日，-1/0/超月按月末（含闰年 2 月）。
func TestSpotConfigDueDate(t *testing.T) {
	if got := spotCfg(0, 0, "", true).DueDate(spotDay("2026-03-05")); got.Day() != 31 {
		t.Fatalf("缺省月末: %v", got)
	}
	c := spotCfg(0, 0, "", true)
	c.DueDay = 15
	if got := c.DueDate(spotDay("2026-03-01")); got.Day() != 15 {
		t.Fatalf("due_day=15: %v", got)
	}
	c.DueDay = 31
	if got := c.DueDate(spotDay("2024-02-01")); got.Month() != 2 || got.Day() != 29 {
		t.Fatalf("闰年 2 月超月应回落月末: %v", got)
	}
}

// TestSampleSpotPointsRatio 比例抽样：向上取整、至少 1、超池取全池。
func TestSampleSpotPointsRatio(t *testing.T) {
	pool := []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"}
	now := spotDay("2026-09-01")
	if got := SampleSpotPoints(pool, nil, nil, spotCfg(20, 0, model.SpotStrategyLongestUnseen, false), now); len(got) != 2 {
		t.Fatalf("10×20%% 应取 2，实际 %d", len(got))
	}
	if got := SampleSpotPoints(pool, nil, nil, spotCfg(25, 0, model.SpotStrategyLongestUnseen, false), now); len(got) != 3 {
		t.Fatalf("10×25%% 应向上取整为 3，实际 %d", len(got))
	}
	if got := SampleSpotPoints(pool[:3], nil, nil, spotCfg(1, 0, model.SpotStrategyLongestUnseen, false), now); len(got) != 1 {
		t.Fatalf("3×1%% 应至少取 1，实际 %d", len(got))
	}
	if got := SampleSpotPoints(pool, nil, nil, spotCfg(100, 0, model.SpotStrategyLongestUnseen, false), now); len(got) != 10 {
		t.Fatalf("100%% 应取全池，实际 %d", len(got))
	}
}

// TestSampleSpotPointsFixed 固定数量优先于比例。
func TestSampleSpotPointsFixed(t *testing.T) {
	pool := []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"}
	now := spotDay("2026-09-01")
	if got := SampleSpotPoints(pool, nil, nil, spotCfg(50, 3, model.SpotStrategyLongestUnseen, false), now); len(got) != 3 {
		t.Fatalf("fixed_count=3 应取 3，实际 %d", len(got))
	}
	if got := SampleSpotPoints(pool, nil, nil, spotCfg(0, 20, model.SpotStrategyLongestUnseen, false), now); len(got) != 10 {
		t.Fatalf("fixed_count 超池应取全池，实际 %d", len(got))
	}
}

// TestSampleSpotPointsOffAndEmpty 关闭（比例/数量均 0）与空池均返回空。
func TestSampleSpotPointsOffAndEmpty(t *testing.T) {
	now := spotDay("2026-09-01")
	if got := SampleSpotPoints([]string{"p1"}, nil, nil, spotCfg(0, 0, model.SpotStrategyLongestUnseen, false), now); len(got) != 0 {
		t.Fatalf("比例/数量均 0 应关闭，实际 %v", got)
	}
	if got := SampleSpotPoints(nil, nil, nil, spotCfg(20, 0, model.SpotStrategyLongestUnseen, false), now); len(got) != 0 {
		t.Fatalf("空池应返回空，实际 %v", got)
	}
}

// TestSampleSpotPointsLongestUnseen 最久未查优先：从未查过（零值）排最前，其余按 lastSeen 升序。
func TestSampleSpotPointsLongestUnseen(t *testing.T) {
	pool := []string{"a", "b", "c", "d"}
	lastSeen := map[string]time.Time{
		"a": spotDay("2026-08-01"),
		"b": spotDay("2026-06-01"), // 最久 → 第一
		"c": spotDay("2026-08-20"),
		// d 从未查过 → 零值最前
	}
	got := SampleSpotPoints(pool, lastSeen, nil, spotCfg(50, 0, model.SpotStrategyLongestUnseen, false), spotDay("2026-09-01"))
	if len(got) != 2 || got[0] != "d" || got[1] != "b" {
		t.Fatalf("longest_unseen 应为 [d b]，实际 %v", got)
	}
}

// TestSampleSpotPointsNoRepeat 连中保护：上月命中先剔除；剔除后不够数再从被剔除者按最久未查补足。
func TestSampleSpotPointsNoRepeat(t *testing.T) {
	now := spotDay("2026-09-01")
	// 剔除后够数：上月命中的 a 不再入选
	pool := []string{"a", "b", "c", "d"}
	got := SampleSpotPoints(pool, nil, map[string]bool{"a": true}, spotCfg(75, 0, model.SpotStrategyLongestUnseen, true), now)
	if len(got) != 3 {
		t.Fatalf("应取 3，实际 %v", got)
	}
	for _, id := range got {
		if id == "a" {
			t.Fatalf("连中保护应剔除上月命中的 a，实际 %v", got)
		}
	}
	// 剔除后不够数：从被剔除者补足（b 上月命中但池仅剩 c，需补 b）
	pool = []string{"b", "c"}
	got = SampleSpotPoints(pool, nil, map[string]bool{"b": true}, spotCfg(100, 2, model.SpotStrategyLongestUnseen, true), now)
	if len(got) != 2 || got[0] != "c" || got[1] != "b" {
		t.Fatalf("剔除不足应按最久未查补足为 [c b]，实际 %v", got)
	}
	// 关闭连中保护：上月命中照常参与
	got = SampleSpotPoints([]string{"a", "b"}, nil, map[string]bool{"a": true}, spotCfg(0, 1, model.SpotStrategyLongestUnseen, false), now)
	if len(got) != 1 {
		t.Fatalf("应取 1，实际 %v", got)
	}
}

// TestSampleSpotPointsRandom 随机策略：取自池内、不重复、数量正确，同一 now 结果可复现。
func TestSampleSpotPointsRandom(t *testing.T) {
	pool := []string{"p1", "p2", "p3", "p4", "p5", "p6"}
	now := time.Now()
	cfg := spotCfg(0, 3, model.SpotStrategyRandom, false)
	got := SampleSpotPoints(pool, nil, nil, cfg, now)
	if len(got) != 3 {
		t.Fatalf("应取 3，实际 %v", got)
	}
	inPool := map[string]bool{}
	for _, id := range pool {
		inPool[id] = true
	}
	seen := map[string]bool{}
	for _, id := range got {
		if !inPool[id] {
			t.Fatalf("抽样结果含池外点位: %v", got)
		}
		if seen[id] {
			t.Fatalf("抽样结果重复: %v", got)
		}
		seen[id] = true
	}
	again := SampleSpotPoints(pool, nil, nil, cfg, now)
	for i := range got {
		if got[i] != again[i] {
			t.Fatalf("同一 now 应可复现: %v vs %v", got, again)
		}
	}
}
