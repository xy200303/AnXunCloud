package service

import (
	"time"

	"github.com/gin-gonic/gin"

	eqsvc "anxuncloud/internal/module/equipment/service"
	insmodel "anxuncloud/internal/module/inspection/model"

	eqmodel "anxuncloud/internal/module/equipment/model"
)

// 月报设备章节（v1.7）：
// - 台账状态快照：按 正常/临期/逾期/应报废/标签缺失 分桶（当前时点，非期间口径）；
// - 当期维保：登记数/确认数/驳回数（created_at 落在期间）；
// - 抽查：触发数/不符数（打卡快照 judge_type=equipment_date_spot，期间口径）；
// - 判定来源：期间逐项快照按 judge_type 分 系统判定（equipment_validity/equipment_date_spot）与 人工/AI 判定（其余）。
func (s *ReportService) buildEquipmentStats(communityID string, start, end time.Time) gin.H {
	out := gin.H{}
	// 台账状态快照（当前时点）
	var rows []eqmodel.Equipment
	s.db.Select("type", "next_due_date", "scrap_date", "warn_days", "status", "label_missing").
		Where("community_id = ?", communityID).Find(&rows)
	globalWarn := eqsvc.CfgInt(s.db, "equipment.expire_warn_days", 30)
	now := time.Now()
	today := truncateDayLocal(now)
	buckets := gin.H{"normal": 0, "warning": 0, "overdue": 0, "scrap": 0, "label_missing": 0}
	byType := map[string]gin.H{}
	for i := range rows {
		e := &rows[i]
		if e.Status != eqmodel.StatusInService {
			continue
		}
		bucket := "normal"
		switch {
		case e.LabelMissing:
			bucket = "label_missing"
		case e.ScrapDate != nil && !truncateDayLocal(*e.ScrapDate).After(today):
			bucket = "scrap"
		case e.NextDueDate == nil:
			bucket = "normal" // 无到期日归正常（不参与判定）
		default:
			warn := globalWarn
			if e.WarnDays != nil && *e.WarnDays > 0 {
				warn = *e.WarnDays
			}
			days := int(truncateDayLocal(*e.NextDueDate).Sub(today).Hours() / 24)
			if days < 0 {
				bucket = "overdue"
			} else if days <= warn {
				bucket = "warning"
			}
		}
		buckets[bucket] = buckets[bucket].(int) + 1
		tb, ok := byType[e.Type]
		if !ok {
			tb = gin.H{"normal": 0, "warning": 0, "overdue": 0, "scrap": 0, "label_missing": 0}
			byType[e.Type] = tb
		}
		tb[bucket] = tb[bucket].(int) + 1
	}
	out["status_buckets"] = buckets
	out["by_type"] = byType

	// 当期维保登记/确认/驳回（台账流水按登记时间）
	var maintSum struct {
		Registered int64
		Confirmed  int64
		Rejected   int64
	}
	s.db.Model(&eqmodel.EquipmentMaintenance{}).
		Joins("JOIN equipment ON equipment.id = equipment_maintenance.equipment_id").
		Where("equipment.community_id = ? AND equipment_maintenance.created_at >= ? AND equipment_maintenance.created_at < ?", communityID, start, end).
		Select(`COUNT(*) AS registered,
			COUNT(*) FILTER (WHERE confirm_status = 'confirmed') AS confirmed,
			COUNT(*) FILTER (WHERE confirm_status = 'rejected') AS rejected`).
		Scan(&maintSum)
	out["maintenance"] = gin.H{"registered": maintSum.Registered, "confirmed": maintSum.Confirmed, "rejected": maintSum.Rejected}

	// 抽查触发与不符 + 判定来源（期间打卡逐项快照；仅最新记录）
	var spotSum struct {
		Triggered int64
		Mismatch  int64
	}
	s.db.Model(&insmodel.CheckinRecordItem{}).
		Joins("JOIN checkin_record ON checkin_record.id = checkin_record_item.record_id").
		Where("checkin_record.community_id = ? AND checkin_record.checkin_time >= ? AND checkin_record.checkin_time < ? AND checkin_record.superseded_by IS NULL", communityID, start, end).
		Where("checkin_record_item.judge_type = ?", "equipment_date_spot").
		Select("COUNT(*) AS triggered, COUNT(*) FILTER (WHERE checkin_record_item.pass = false) AS mismatch").
		Scan(&spotSum)
	out["spotcheck"] = gin.H{"triggered": spotSum.Triggered, "mismatch": spotSum.Mismatch}

	var srcSum struct {
		System   int64
		ManualAI int64
	}
	s.db.Model(&insmodel.CheckinRecordItem{}).
		Joins("JOIN checkin_record ON checkin_record.id = checkin_record_item.record_id").
		Where("checkin_record.community_id = ? AND checkin_record.checkin_time >= ? AND checkin_record.checkin_time < ? AND checkin_record.superseded_by IS NULL", communityID, start, end).
		Select(`COUNT(*) FILTER (WHERE checkin_record_item.judge_type IN ('equipment_validity','equipment_date_spot')) AS system,
			COUNT(*) FILTER (WHERE checkin_record_item.judge_type NOT IN ('equipment_validity','equipment_date_spot')) AS manual_ai`).
		Scan(&srcSum)
	out["judge_source"] = gin.H{"system": srcSum.System, "manual_ai": srcSum.ManualAI}
	return out
}

// truncateDayLocal 按本地时区截断到日（报表分桶用，与 equipment 模块日粒度同口径）。
func truncateDayLocal(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}
