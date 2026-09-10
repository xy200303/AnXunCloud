package service

import (
	"testing"

	"anxuncloud/internal/pkg/types"
)

func TestSpotTriggered(t *testing.T) {
	// 确定性：同输入同输出
	a := SpotTriggered("task1", "point1", "eq1", "2026-09-09", "salt-x", 50)
	for i := 0; i < 5; i++ {
		if SpotTriggered("task1", "point1", "eq1", "2026-09-09", "salt-x", 50) != a {
			t.Fatal("同输入应同输出")
		}
	}
	// 不同日期结果可能不同（至少分布上两边都出现）
	hit := map[bool]int{}
	for d := 1; d <= 28; d++ {
		hit[SpotTriggered("task1", "point1", "eq1", "2026-09-"+pad(d), "salt-x", 50)]++
	}
	if len(hit) < 2 {
		t.Fatal("ratio=50 时多日采样应同时出现命中与不命中")
	}
	// 边界：0 永不触发，100 必触发
	if SpotTriggered("t", "p", "e", "2026-09-09", "s", 0) {
		t.Fatal("ratio=0 不应触发")
	}
	if !SpotTriggered("t", "p", "e", "2026-09-09", "s", 100) {
		t.Fatal("ratio=100 应必触发")
	}
	// 盐值影响结果（防推算）
	if SpotTriggered("t", "p", "e", "2026-09-09", "salt-a", 50) == SpotTriggered("t", "p", "e", "2026-09-09", "salt-b", 50) &&
		SpotTriggered("t", "p", "e2", "2026-09-10", "salt-a", 50) == SpotTriggered("t", "p", "e2", "2026-09-10", "salt-b", 50) {
		t.Fatal("不同盐值应产生不同序列")
	}
}

func pad(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestSpotRatioFor(t *testing.T) {
	now := day("2026-09-09")
	t.Run("类型覆盖全局", func(t *testing.T) {
		fresh := day("2026-08-01") // 近期有验证，不翻倍
		if r := SpotRatioFor(TypeRule{SpotRatio: 20}, 10, &fresh, now); r != 20 {
			t.Fatalf("类型覆盖: %d", r)
		}
	})
	// 长期未验证翻倍
	stale := day("2026-01-01")
	if r := SpotRatioFor(TypeRule{SpotRatio: 0}, 10, &stale, now); r != 20 {
		t.Fatalf("未验证翻倍: %d", r)
	}
	// cap 100
	if r := SpotRatioFor(TypeRule{SpotRatio: 80}, 10, &stale, now); r != 100 {
		t.Fatalf("cap 100: %d", r)
	}
	// 近期有验证不翻倍
	fresh := day("2026-08-01")
	if r := SpotRatioFor(TypeRule{}, 10, &fresh, now); r != 10 {
		t.Fatalf("近期已验证: %d", r)
	}
	// 类型规则解析 spot_ratio
	r := ParseTypeRule(types.JSONMap{"spot_ratio": float64(15)})
	if r.SpotRatio != 15 {
		t.Fatalf("spot_ratio 解析: %d", r.SpotRatio)
	}
}

func TestCompareSpot(t *testing.T) {
	now := day("2026-09-09")
	mfg := day("2020-05-10")
	maint := day("2025-03-15")
	base := SpotCompareInput{
		LedgerManufacture: &mfg, LedgerLastMaint: &maint,
		LabelManufacture: &mfg, LabelMaint: &maint,
		FirstMonths: 60, CycleMonths: 24, Now: now,
	}

	t.Run("全部相符→通过", func(t *testing.T) {
		if r := CompareSpot(base); !r.Pass {
			t.Fatalf("%v", r.Mismatches)
		}
	})
	t.Run("年月容差：同月不同日算一致", func(t *testing.T) {
		in := base
		d := day("2025-03-01")
		in.LabelMaint = &d
		if r := CompareSpot(in); !r.Pass {
			t.Fatalf("%v", r.Mismatches)
		}
	})
	t.Run("规则1 生产日期不符", func(t *testing.T) {
		in := base
		d := day("2021-06-01")
		in.LabelManufacture = &d
		r := CompareSpot(in)
		if r.Pass || len(r.Mismatches) == 0 {
			t.Fatal("应不符")
		}
	})
	t.Run("规则2 贴纸日期不符", func(t *testing.T) {
		in := base
		d := day("2024-03-01")
		in.LabelMaint = &d
		if r := CompareSpot(in); r.Pass {
			t.Fatal("应不符")
		}
	})
	t.Run("规则3 无贴纸但台账有维修记录→不符；双方都无→一致", func(t *testing.T) {
		in := base
		in.LabelMaint = nil
		in.NoSticker = true
		if r := CompareSpot(in); r.Pass {
			t.Fatal("无贴纸+台账有记录应不符")
		}
		// 双方都无：规则3 一致；出厂日期要足够新避免规则4实物超期干扰
		in.LedgerLastMaint = nil
		recent := day("2025-06-01")
		in.LabelManufacture = &recent
		in.LedgerManufacture = &recent
		if r := CompareSpot(in); !r.Pass {
			t.Fatalf("双方都无应一致: %v", r.Mismatches)
		}
	})
	t.Run("规则4 实物超期兜底（台账被做假也骗不过）", func(t *testing.T) {
		in := base
		fakeLedger := day("2026-08-01") // 台账造假成最近
		in.LedgerLastMaint = &fakeLedger
		in.LedgerManufacture = &fakeLedger
		in.LabelManufacture = &fakeLedger // 标签一致绕过规则1
		old := day("2020-01-01")
		in.LabelMaint = &old // 贴纸 2020-01 + 24 个月 = 2022-01 早超期
		// 规则2 会中（贴纸 vs 台账不符），同时验证规则4
		r := CompareSpot(in)
		if r.Pass {
			t.Fatal("应不符")
		}
		found := false
		for _, m := range r.Mismatches {
			if len(m) > 0 && contains(m, "实物超期") {
				found = true
			}
		}
		if !found {
			t.Fatalf("规则4 未触发: %v", r.Mismatches)
		}
	})
	t.Run("无贴纸按出厂+首保推有效期", func(t *testing.T) {
		in := base
		in.LedgerLastMaint = nil
		in.LabelMaint = nil
		in.NoSticker = true
		old := day("2019-01-01") // 出厂 2019-01 + 60 个月 = 2024-01 超期
		in.LabelManufacture = &old
		in.LedgerManufacture = &old
		if r := CompareSpot(in); r.Pass {
			t.Fatal("无贴纸超期应不符")
		}
	})
	t.Run("标签缺失→直接强制异常", func(t *testing.T) {
		in := base
		in.LabelMissing = true
		r := CompareSpot(in)
		if r.Pass || len(r.Mismatches) != 1 {
			t.Fatalf("%v", r.Mismatches)
		}
	})
	t.Run("台账缺出厂日期→不符", func(t *testing.T) {
		in := base
		in.LedgerManufacture = nil
		if r := CompareSpot(in); r.Pass {
			t.Fatal("应不符")
		}
	})
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// label_missing / 报废 状态机
func TestJudgeDeviceSpecialStates(t *testing.T) {
	now := day("2026-09-09")

	t.Run("label_missing 退出判定", func(t *testing.T) {
		e := eq("e1", "X-001", dayPtr("2026-08-01"), nil) // 已逾期
		e.LabelMissing = true
		j := JudgeDevice(e, false, 30, now)
		if j.State != AutoLabelMissing {
			t.Fatalf("got %s", j.State)
		}
		pass, note := DeviceJudgeSubmit(j, true, now)
		if !pass {
			t.Fatal("标签缺失不应判异常")
		}
		if note == "" {
			t.Fatal("应有处置提示")
		}
	})
	t.Run("报废日已过视同逾期且每次判异常（不去重）", func(t *testing.T) {
		e := eq("e1", "X-001", dayPtr("2027-01-01"), nil)
		e.ScrapDate = dayPtr("2026-09-01")
		j := JudgeDevice(e, false, 30, now)
		if !j.ScrapDue || j.State != DueOverdue {
			t.Fatalf("got %+v", j)
		}
		// firstOverdue=false（已报过）仍判异常
		pass, note := DeviceJudgeSubmit(j, false, now)
		if pass {
			t.Fatal("报废设备不应放行")
		}
		if !contains(note, "报废") {
			t.Fatalf("note=%q", note)
		}
	})
	t.Run("报废日未到不受影响", func(t *testing.T) {
		e := eq("e1", "X-001", dayPtr("2027-01-01"), nil)
		e.ScrapDate = dayPtr("2027-01-01")
		j := JudgeDevice(e, false, 30, now)
		if j.ScrapDue || j.State != DueNormal {
			t.Fatalf("got %+v", j)
		}
	})
	t.Run("label_missing 优先于报废", func(t *testing.T) {
		e := eq("e1", "X-001", nil, nil)
		e.LabelMissing = true
		e.ScrapDate = dayPtr("2020-01-01")
		j := JudgeDevice(e, false, 30, now)
		if j.State != AutoLabelMissing {
			t.Fatalf("got %s", j.State)
		}
	})
}
