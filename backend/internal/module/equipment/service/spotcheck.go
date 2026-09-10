package service

import (
	"fmt"
	"hash/fnv"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/equipment/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
)

// ========== 日期标签抽查（equipment_date_spot，v1.7 二期；落到逐台模型） ==========

// SpotItemPrefix 抽查合成项名前缀（标签抽查·名称(编号)）。
const SpotItemPrefix = "标签抽查·"

// CfgBool 读取系统参数布尔值（缺失/非法回退默认值；与 cfgInt 同口径直读 sys_config，导出供 mp 模块用）。
func CfgBool(db *gorm.DB, key string, def bool) bool {
	var v string
	if err := db.Model(&sysmodel.SysConfig{}).Where("key = ?", key).Select("value").Scan(&v).Error; err == nil {
		switch v {
		case "true":
			return true
		case "false":
			return false
		}
	}
	return def
}

// CfgString 读取系统参数字符串（缺失回退默认值）。
func CfgString(db *gorm.DB, key, def string) string {
	var v string
	if err := db.Model(&sysmodel.SysConfig{}).Where("key = ?", key).Select("value").Scan(&v).Error; err == nil && v != "" {
		return v
	}
	return def
}

// SpotTriggered 确定性哈希抽查判定（纯函数）：同任务同点位同设备同日结果固定。
// hash(taskID + pointID + equipmentID + taskDate + salt) % 100 < ratio。
func SpotTriggered(taskID, pointID, equipmentID, taskDate, salt string, ratio int) bool {
	if ratio <= 0 {
		return false
	}
	if ratio >= 100 {
		return true
	}
	h := fnv.New32a()
	h.Write([]byte(taskID + "|" + pointID + "|" + equipmentID + "|" + taskDate + "|" + salt))
	return int(h.Sum32()%100) < ratio
}

// EffectiveSpotRatio 抽查比例：设备类型 spot_ratio（>0 生效）> 全局默认；长期未验证翻倍由调用方乘算后 cap 100。
func EffectiveSpotRatio(rule TypeRule, globalRatio int) int {
	if rule.SpotRatio > 0 {
		return rule.SpotRatio
	}
	return globalRatio
}

// LastVerifiedMap 批量取设备「最近验证时间」（confirmed 维保日期 与 抽查通过快照时间取大者）。
// 长期未验证（超 6 个月）的设备抽查概率翻倍。
func LastVerifiedMap(db *gorm.DB, equipmentIDs []string) map[string]time.Time {
	out := map[string]time.Time{}
	if len(equipmentIDs) == 0 {
		return out
	}
	// confirmed 维保流水
	var maintRows []struct {
		EquipmentID string `gorm:"column:equipment_id"`
		LastAt      string `gorm:"column:last_at"`
	}
	db.Model(&model.EquipmentMaintenance{}).
		Select("equipment_id, MAX(maintenance_date) AS last_at").
		Where("equipment_id IN ? AND confirm_status = ?", equipmentIDs, model.ConfirmConfirmed).
		Group("equipment_id").Scan(&maintRows)
	for _, r := range maintRows {
		if t, err := time.ParseInLocation("2006-01-02", r.LastAt, time.Local); err == nil {
			out[r.EquipmentID] = t
		}
	}
	// 抽查通过快照（判定期权在服务端，pass=true 的合成项即一次验证）
	var spotRows []struct {
		EquipmentID string    `gorm:"column:equipment_id"`
		LastAt      time.Time `gorm:"column:last_at"`
	}
	db.Model(&insmodel.CheckinRecordItem{}).
		Select("judge_config->>'equipment_id' AS equipment_id, MAX(created_at) AS last_at").
		Where("judge_type = ? AND pass = ?", "equipment_date_spot", true).
		Where("judge_config->>'equipment_id' IN ?", equipmentIDs).
		Group("equipment_id").Scan(&spotRows)
	for _, r := range spotRows {
		t := truncateDay(r.LastAt)
		if cur, ok := out[r.EquipmentID]; !ok || t.After(cur) {
			out[r.EquipmentID] = t
		}
	}
	return out
}

// spotStaleMonths 台账长期未验证阈值（月）：超过则抽查概率翻倍。
const spotStaleMonths = 6

// SpotRatioFor 单台设备最终抽查比例（纯函数）：长期未验证翻倍（cap 100）。
func SpotRatioFor(rule TypeRule, globalRatio int, lastVerified *time.Time, now time.Time) int {
	ratio := EffectiveSpotRatio(rule, globalRatio)
	if lastVerified == nil || lastVerified.AddDate(0, spotStaleMonths, 0).Before(now) {
		ratio *= 2
	}
	if ratio > 100 {
		ratio = 100
	}
	return ratio
}

// ========== 抽查比对（§3.7 四规则，年月粒度容差；只核对不写台账） ==========

// SpotCompareInput 抽查比对输入：台账侧 + 现场侧（日期统一截断到年月比对）。
type SpotCompareInput struct {
	LedgerManufacture *time.Time // 台账出厂日期
	LedgerLastMaint   *time.Time // 台账最近维保日期
	LabelManufacture  *time.Time // 瓶体生产日期（现场读数；nil=读不到）
	LabelMaint        *time.Time // 维修贴纸日期（nil=无贴纸/读不到）
	NoSticker         bool       // 巡检员选「无贴纸」
	LabelMissing      bool       // 巡检员勾选「标签缺失/无法辨认」
	FirstMonths       int        // 类型首保月数（规则 4：无贴纸时出厂+首保）
	CycleMonths       int        // 类型周期月数（规则 4：贴纸+周期；0 回落 24 个月）
	Now               time.Time
}

// SpotCompareResult 比对结果：Pass=false 时 Mismatches 为不符明细。
type SpotCompareResult struct {
	Pass       bool
	Mismatches []string
}

// sameMonth 年月粒度相等（钢印格式不统一，按日比对会误报）。
func sameMonth(a, b *time.Time) bool {
	if a == nil || b == nil {
		return false
	}
	return a.In(time.Local).Year() == b.In(time.Local).Year() && a.In(time.Local).Month() == b.In(time.Local).Month()
}

// CompareSpot 抽查四规则比对（纯函数）：
//  1. 瓶体生产日期 vs 台账 manufacture_date（年月容差）——不符=设备被私自更换/台账未跟上
//  2. 维修贴纸日期 vs 台账 last_maintenance_date（年月容差）——不符=换粉漏登记或假登记
//  3. 瓶上无贴纸 vs 台账存在维修记录——不符=假登记实锤
//  4. 实物日期独立推有效期：贴纸+周期（无贴纸则出厂+首保）≥ 今天？——实物超期终极兜底
//
// 标签缺失勾选 → 直接不符（强制异常进审核，等经理处置）。
func CompareSpot(in SpotCompareInput) SpotCompareResult {
	if in.LabelMissing {
		return SpotCompareResult{Pass: false, Mismatches: []string{"现场标签缺失/无法辨认，待经理处置"}}
	}
	var mm []string
	// 规则 1：生产日期 vs 台账出厂
	if in.LabelManufacture != nil {
		if in.LedgerManufacture == nil {
			mm = append(mm, "台账未录出厂日期，无法与瓶体钢印比对")
		} else if !sameMonth(in.LabelManufacture, in.LedgerManufacture) {
			mm = append(mm, fmt.Sprintf("瓶体生产日期 %s 与台账出厂日期 %s 不符（疑似设备被更换）",
				in.LabelManufacture.Format("2006-01"), in.LedgerManufacture.Format("2006-01")))
		}
	}
	// 规则 2+3：贴纸 vs 台账维修记录
	if in.NoSticker || in.LabelMaint == nil {
		if in.LedgerLastMaint != nil {
			mm = append(mm, fmt.Sprintf("瓶上无维修贴纸，但台账存在维修记录（%s）", in.LedgerLastMaint.Format("2006-01")))
		}
	} else {
		if in.LedgerLastMaint == nil {
			mm = append(mm, fmt.Sprintf("贴纸维修日期 %s 无台账维修记录（漏登记）", in.LabelMaint.Format("2006-01")))
		} else if !sameMonth(in.LabelMaint, in.LedgerLastMaint) {
			mm = append(mm, fmt.Sprintf("贴纸维修日期 %s 与台账最近维保 %s 不符",
				in.LabelMaint.Format("2006-01"), in.LedgerLastMaint.Format("2006-01")))
		}
	}
	// 规则 4：实物独立推有效期
	cycle := in.CycleMonths
	if cycle <= 0 {
		cycle = 24
	}
	first := in.FirstMonths
	if first <= 0 {
		first = cycle
	}
	var effective *time.Time
	if in.LabelMaint != nil {
		d := truncateDay(*in.LabelMaint).AddDate(0, cycle, 0)
		effective = &d
	} else if in.LabelManufacture != nil {
		d := truncateDay(*in.LabelManufacture).AddDate(0, first, 0)
		effective = &d
	}
	if effective != nil && effective.Before(truncateDay(in.Now)) {
		mm = append(mm, fmt.Sprintf("按实物日期推算有效期至 %s，已超过今天（实物超期）", effective.Format("2006-01")))
	}
	return SpotCompareResult{Pass: len(mm) == 0, Mismatches: mm}
}
