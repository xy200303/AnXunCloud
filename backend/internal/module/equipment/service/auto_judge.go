package service

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/equipment/model"
	insmodel "anxuncloud/internal/module/inspection/model"
)

// 台账有效期（equipment_validity）逐台自动判定扩展状态（§3.5 v1.6：绑定即启用、逐台独立）
const (
	AutoNoData       = "no_data"       // 设备无到期日（无规则/缺日期数据，无法判定 → 需台账补录）
	AutoLabelMissing = "label_missing" // 标签缺失/无法辨认（确认链打标）：退出自动判定，走经理处置通道
)

// DeviceJudge 单台设备的台账有效期判定结果（任务详情合成检查项与打卡提交判定共用）。
type DeviceJudge struct {
	EquipmentID  string
	Code         string
	Name         string
	Type         string     // 设备类型（抽查比例按类型规则取）
	State        string     // normal/warning/overdue/no_data/label_missing
	NextDueDate  *time.Time // 到期日（no_data/label_missing 可能为 nil）
	ScrapDate    *time.Time // 报废日期（可空）
	WarnDays     int        // 生效的临期阈值（设备覆盖 > 全局）
	OverdueDays  int        // 已逾期天数（>0 表示逾期）
	ScrapDue     bool       // 报废日已过（scrap_date <= 今天）：视同逾期，用报废设备本身即违规
	HasPending   bool       // 存在待确认维保登记（暂停催办，展示「待确认」）
	ShowRegister bool       // 是否展示「已完成维保？登记」入口（warning/overdue/no_data）
}

// JudgeDevice 单台设备判定（纯函数）：pending 为该设备是否存在待确认登记。
// 优先级：label_missing（退出判定）> 报废日已过（视同逾期）> 到期日判定。
// 临期阈值逐设备取 warn_days，缺省用 globalWarn（与 EquipmentService.warnDaysOf 同口径）。
func JudgeDevice(e model.Equipment, pending bool, globalWarn int, now time.Time) DeviceJudge {
	warn := globalWarn
	if e.WarnDays != nil && *e.WarnDays > 0 {
		warn = *e.WarnDays
	}
	j := DeviceJudge{
		EquipmentID: e.ID, Code: e.Code, Name: e.Name, Type: e.Type,
		NextDueDate: e.NextDueDate, ScrapDate: e.ScrapDate, WarnDays: warn, HasPending: pending,
	}
	if e.LabelMissing {
		j.State = AutoLabelMissing
		return j
	}
	// 报废日已过：视同逾期（show_register 无意义——报废设备该换不该维保）
	if e.ScrapDate != nil && !truncateDay(*e.ScrapDate).After(truncateDay(now)) {
		j.ScrapDue = true
		j.State = DueOverdue
		if e.NextDueDate != nil && truncateDay(*e.NextDueDate).Before(truncateDay(now)) {
			j.OverdueDays = int(truncateDay(now).Sub(truncateDay(*e.NextDueDate)).Hours() / 24)
		}
		return j
	}
	if e.NextDueDate == nil {
		j.State = AutoNoData
		j.ShowRegister = true // 缺日期 → 触发台账补录
		return j
	}
	j.State = DueStatus(e.NextDueDate, warn, now)
	if j.State == DueOverdue {
		j.OverdueDays = int(truncateDay(now).Sub(truncateDay(*e.NextDueDate)).Hours() / 24)
	}
	j.ShowRegister = j.State == DueWarning || j.State == DueOverdue
	return j
}

// JudgeDevices 逐台判定（纯函数；pendingSet 由 PendingMaintenanceSet 批量查出）。
func JudgeDevices(eqs []model.Equipment, pendingSet map[string]bool, globalWarn int, now time.Time) []DeviceJudge {
	out := make([]DeviceJudge, 0, len(eqs))
	for i := range eqs {
		out = append(out, JudgeDevice(eqs[i], pendingSet[eqs[i].ID], globalWarn, now))
	}
	return out
}

// SyntheticItemName 合成检查项名（打卡快照落库同名）：设备维保·名称(编号)。
func SyntheticItemName(j DeviceJudge) string {
	return fmt.Sprintf("设备维保·%s(%s)", j.Name, j.Code)
}

// overdueNote 逾期备注基础文案（设备维保逾期：编号 xxx 到期日 xxx）。
func overdueNote(j DeviceJudge) string {
	return fmt.Sprintf("设备维保逾期：编号 %s 到期日 %s", j.Code, dateStr(j.NextDueDate))
}

// DeviceJudgeSubmit 打卡提交逐台判定映射（纯函数）：
// firstOverdue=true 表示该设备首次逾期打卡（此前无逾期异常快照）→ 产异常；
// 同一设备持续逾期的后续打卡只标「催办中」不再产新异常（逐台独立去重，互不影响）；
// 报废日已过视同逾期且每次都判异常（使用报废设备本身即违规，不去重）；
// 标签缺失设备退出判定（理论上不会进入打卡合成项，防御性按合格处理）。
func DeviceJudgeSubmit(j DeviceJudge, firstOverdue bool, now time.Time) (pass bool, note string) {
	if j.State == AutoLabelMissing {
		return true, "标签缺失，待经理处置"
	}
	if j.ScrapDue {
		return false, fmt.Sprintf("设备已过报废日期（%s），应立即停用更换", dateStr(j.ScrapDate))
	}
	switch j.State {
	case DueOverdue:
		switch {
		case j.HasPending:
			return true, overdueNote(j) + "（已登记维保待确认）"
		case firstOverdue:
			return false, overdueNote(j)
		default:
			return true, overdueNote(j) + "（维保逾期·催办中）"
		}
	case AutoNoData:
		return true, "台账数据缺失待补录"
	case DueWarning:
		days := int(truncateDay(*j.NextDueDate).Sub(truncateDay(now)).Hours() / 24)
		return true, fmt.Sprintf("将于 %d 天内到期（到期日 %s）", days, dateStr(j.NextDueDate))
	default: // normal
		return true, ""
	}
}

// View 任务详情合成项透出结构（每台设备一条；judge_config 另携 equipment_id/equipment_no 快照）。
func (j DeviceJudge) View() map[string]any {
	return map[string]any{
		"status":               j.State,
		"next_due_date":        dateStr(j.NextDueDate),
		"scrap_date":           dateStr(j.ScrapDate),
		"scrap_due":            j.ScrapDue,
		"warn_days":            j.WarnDays,
		"overdue_days":         j.OverdueDays,
		"has_pending_register": j.HasPending,
		"show_register":        j.ShowRegister,
		"equipment_id":         j.EquipmentID,
		"equipment_code":       j.Code,
		"equipment_name":       j.Name,
	}
}

// LoadPointEquipment 批量按点位加载在用关联设备（point_id IN，软删自动排除；任务详情组装消除 N+1）。
// 标签缺失设备排除（已退出自动判定，走经理处置通道，不再出现合成项）。
func LoadPointEquipment(db *gorm.DB, pointIDs []string) (map[string][]model.Equipment, error) {
	out := map[string][]model.Equipment{}
	if len(pointIDs) == 0 {
		return out, nil
	}
	var rows []model.Equipment
	if err := db.Where("point_id IN ? AND status = ? AND label_missing = ?", pointIDs, model.StatusInService, false).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].PointID == nil {
			continue
		}
		out[*rows[i].PointID] = append(out[*rows[i].PointID], rows[i])
	}
	return out, nil
}

// GlobalWarnDays 全局临期阈值（sys_config equipment.expire_warn_days，默认 30）。
func GlobalWarnDays(db *gorm.DB) int {
	return cfgInt(db, "equipment.expire_warn_days", 30)
}

// HasPendingMaintenance 设备是否存在待确认维保登记。
func HasPendingMaintenance(db *gorm.DB, equipmentID string) bool {
	var count int64
	db.Model(&model.EquipmentMaintenance{}).
		Where("equipment_id = ? AND confirm_status = ?", equipmentID, model.ConfirmPending).
		Count(&count)
	return count > 0
}

// PendingMaintenanceSet 批量查待确认登记（设备 id → 是否有 pending 流水）。
func PendingMaintenanceSet(db *gorm.DB, equipmentIDs []string) map[string]bool {
	out := map[string]bool{}
	if len(equipmentIDs) == 0 {
		return out
	}
	var ids []string
	db.Model(&model.EquipmentMaintenance{}).
		Where("equipment_id IN ? AND confirm_status = ?", equipmentIDs, model.ConfirmPending).
		Distinct().Pluck("equipment_id", &ids)
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// OverdueReportedSet 批量查「已产过逾期异常快照」的设备（逐台去重键控：
// checkin_record_item 合成项 judge_config->>'equipment_id' 命中且 pass=false，记录未被覆盖）。
func OverdueReportedSet(db *gorm.DB, equipmentIDs []string) map[string]bool {
	out := map[string]bool{}
	if len(equipmentIDs) == 0 {
		return out
	}
	var ids []string
	db.Model(&insmodel.CheckinRecordItem{}).
		Where("judge_type = ?", "equipment_validity").
		Where("judge_config->>'equipment_id' IN ?", equipmentIDs).
		Where("pass = ?", false).
		Where("record_id IN (SELECT id FROM checkin_record WHERE superseded_by IS NULL)").
		Distinct().Pluck("judge_config->>'equipment_id'", &ids)
	for _, id := range ids {
		out[id] = true
	}
	return out
}
