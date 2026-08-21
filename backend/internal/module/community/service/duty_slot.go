// 职责槽位目录组装（静态目录 + 按字典动态衍生的维度槽位）。
package service

import (
	"strings"

	"gorm.io/gorm"

	sysmodel "anxuncloud/internal/module/system/model"
)

// AllDutySlots 全部职责槽位（《专项巡检与专项检查报告设计方案》§3.1）：
// 静态目录 sysmodel.DutySlots + 按 patrol_type 字典动态衍生的汇报线维度槽位——
// 字典每个启用值 v 约定衍生 patrol_report_line.v（已在静态目录的不重复），
// 追加在静态维度槽位之后；字典查询失败（如 seed 未跑）退回静态目录。
// 新增专项类型只需加字典项，槽位目录零代码衍生。
func AllDutySlots(db *gorm.DB) []sysmodel.DutySlot {
	slots := make([]sysmodel.DutySlot, len(sysmodel.DutySlots))
	copy(slots, sysmodel.DutySlots)
	known := make(map[string]bool, len(slots))
	// insertAt 静态维度槽位（patrol_report_line.*）之后的位置，衍生槽位插在这里保持分组
	insertAt := len(slots)
	for i, ds := range slots {
		known[ds.Slot] = true
		if strings.HasPrefix(ds.Slot, sysmodel.SlotPatrolReportLine+".") {
			insertAt = i + 1
		}
	}
	var rows []sysmodel.SysDictData
	if err := db.Select("value", "label").
		Where("type_code = ? AND status = ?", "patrol_type", sysmodel.StatusEnabled).
		Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return slots
	}
	derived := make([]sysmodel.DutySlot, 0, len(rows))
	for _, r := range rows {
		slot := sysmodel.SlotPatrolReportLine + "." + r.Value
		if known[slot] {
			continue
		}
		known[slot] = true
		derived = append(derived, sysmodel.DutySlot{Slot: slot, Name: "巡查打卡审核 · " + r.Label})
	}
	if len(derived) == 0 {
		return slots
	}
	out := make([]sysmodel.DutySlot, 0, len(slots)+len(derived))
	out = append(out, slots[:insertAt]...)
	out = append(out, derived...)
	out = append(out, slots[insertAt:]...)
	return out
}
