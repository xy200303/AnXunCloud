package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/equipment/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"

	"go.uber.org/zap"
)

// ExpireService 设备到期提醒 job（Scheduler 注册名 equipment_expire；§3.8 提醒与升级链）。
// 临期：每周期只提醒一次（last_notified_at 早于本周期起点才提醒）；
// 逾期：每 equipment.overdue_remind_interval_days 天重复提醒；
// 逾期超 equipment.escalate_days 天额外升级 tenant_admin（文案含"请督促"）；
// 存在 pending 登记的设备暂停催办。
type ExpireService struct {
	db       *gorm.DB
	notifier *notify.Notifier
	lastRun  string // 当日已跑标记（幂等防重，进程内存即可：重启当天补跑无害）
}

func NewExpireService(db *gorm.DB, notifier *notify.Notifier) *ExpireService {
	return &ExpireService{db: db, notifier: notifier}
}

// Run 供 Scheduler 注册的入口：每日 equipment.expire_check_time（默认 08:00）后执行一次。
func (s *ExpireService) Run(now time.Time) error {
	today := now.Format("20060102")
	if today == s.lastRun {
		return nil
	}
	checkTime := "08:00"
	var v string
	if err := s.db.Model(&sysmodel.SysConfig{}).Where("key = ?", "equipment.expire_check_time").
		Select("value").Scan(&v).Error; err == nil && strings.TrimSpace(v) != "" {
		checkTime = strings.TrimSpace(v)
	}
	if now.Format("15:04") < checkTime {
		return nil
	}
	n, err := s.ScanExpiring(now)
	if err != nil {
		return err
	}
	s.lastRun = today
	if n > 0 {
		logger.L.Info("设备到期提醒扫描完成", zap.Int("notified", n))
	}
	return nil
}

// ScanExpiring 扫描在用、启用催办类型、next_due_date 非空的设备，按规则推送提醒；返回本次提醒设备数。
func (s *ExpireService) ScanExpiring(now time.Time) (int, error) {
	globalWarn := cfgInt(s.db, "equipment.expire_warn_days", 30)
	escalateDays := cfgInt(s.db, "equipment.escalate_days", 7)
	intervalDays := cfgInt(s.db, "equipment.overdue_remind_interval_days", 7)

	// 报废催办链（v1.7）：scrap_date 已过 → 报废提醒（巡检员 + 经理升级）
	scrapNotified, err := s.scanScrap(now, intervalDays)
	if err != nil {
		return 0, err
	}

	// 仅启用催办的类型参与（零值规则/外包合同驱动型不催办）
	rules, _ := NewEquipmentService(s.db).typeRules()
	remindTypes := make([]string, 0, len(rules))
	for t, r := range rules {
		if r.Remind {
			remindTypes = append(remindTypes, t)
		}
	}
	if len(remindTypes) == 0 {
		return scrapNotified, nil
	}
	var rows []model.Equipment
	if err := s.db.
		// 标签缺失设备退出自动判定与提醒（走经理处置通道）
		Where("status = ? AND next_due_date IS NOT NULL AND tenant_id IS NOT NULL AND label_missing = ?", model.StatusInService, false).
		Where("type IN ?", remindTypes).
		// 存在 pending 登记的设备暂停催办（已登记待确认，不该再催；驳回后自然恢复）
		Where("NOT EXISTS (SELECT 1 FROM equipment_maintenance m WHERE m.equipment_id = equipment.id AND m.confirm_status = ?)", model.ConfirmPending).
		Find(&rows).Error; err != nil {
		return scrapNotified, err
	}
	today := truncateDay(now)
	notified := 0
	for i := range rows {
		e := &rows[i]
		due := truncateDay(*e.NextDueDate)
		days := int(due.Sub(today).Hours() / 24) // >0 未到期；0 当天；<0 已逾期
		warn := globalWarn
		if e.WarnDays != nil && *e.WarnDays > 0 {
			warn = *e.WarnDays
		}
		var title, content string
		var escalate bool
		switch {
		case days >= 0 && days <= warn:
			// 临期：本周期只提醒一次——周期起点 = 到期日 - warn 天，last_notified_at 早于起点才算未提醒过
			cycleStart := due.AddDate(0, 0, -warn)
			if e.LastNotifiedAt != nil && !truncateDay(*e.LastNotifiedAt).Before(cycleStart) {
				continue
			}
			title = "设备维保临期提醒"
			content = fmt.Sprintf("设备「%s（%s）」将于 %d 天后到维保期（到期日 %s），请安排维保。", e.Name, e.Code, days, dateStr(e.NextDueDate))
		case days < 0:
			overdueDays := -days
			// 逾期重复提醒间隔（防打扰）
			if e.LastNotifiedAt != nil && now.Sub(*e.LastNotifiedAt) < time.Duration(intervalDays)*24*time.Hour {
				continue
			}
			title = "设备维保逾期提醒"
			content = fmt.Sprintf("设备「%s（%s）」维保已逾期 %d 天（到期日 %s），请尽快完成维保登记。", e.Name, e.Code, overdueDays, dateStr(e.NextDueDate))
			escalate = overdueDays > escalateDays
		default:
			continue
		}
		// 接收人从简：该设备租户内 field_staff/project_admin 角色且 enabled 的用户
		recipients := s.userIDsByRoles(*e.TenantID, []string{sysmodel.FieldStaffCode, sysmodel.ProjectAdminCode})
		if len(recipients) > 0 {
			if err := s.notifier.SendBatch(recipients, e.TenantID, MsgTypeExpire, title, content, &e.ID); err != nil {
				logger.L.Warn("设备到期提醒发送失败", zap.String("equipment_id", e.ID), zap.Error(err))
				continue
			}
		}
		// 逾期升级：额外通知 tenant_admin（督促口径）
		if escalate {
			admins := s.userIDsByRoles(*e.TenantID, []string{sysmodel.TenantAdminCode})
			if len(admins) > 0 {
				escContent := fmt.Sprintf("设备「%s（%s）」维保已逾期 %d 天仍未登记，请督促巡检员尽快完成维保。", e.Name, e.Code, -days)
				if err := s.notifier.SendBatch(admins, e.TenantID, MsgTypeExpire, "设备维保逾期升级", escContent, &e.ID); err != nil {
					logger.L.Warn("设备逾期升级提醒发送失败", zap.String("equipment_id", e.ID), zap.Error(err))
				}
			}
		}
		if err := s.db.Model(e).Update("last_notified_at", now).Error; err != nil {
			logger.L.Warn("设备提醒时间回写失败", zap.String("equipment_id", e.ID), zap.Error(err))
			continue
		}
		notified++
	}
	return scrapNotified + notified, nil
}

// scanScrap 报废催办链（v1.7）：scrap_date 已过的在用设备 → 报废提醒。
// 节奏复用逾期链配置（equipment.overdue_remind_interval_days 间隔重复）；
// 报废属违规使用风险，提醒同时升级 tenant_admin（督促口径）。标签缺失/非在用设备不参与。
func (s *ExpireService) scanScrap(now time.Time, intervalDays int) (int, error) {
	today := truncateDay(now)
	var rows []model.Equipment
	if err := s.db.
		Where("status = ? AND scrap_date IS NOT NULL AND scrap_date <= ? AND tenant_id IS NOT NULL AND label_missing = ?",
			model.StatusInService, today.Format("2006-01-02"), false).
		Where("NOT EXISTS (SELECT 1 FROM equipment_maintenance m WHERE m.equipment_id = equipment.id AND m.confirm_status = ?)", model.ConfirmPending).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	notified := 0
	for i := range rows {
		e := &rows[i]
		// 防打扰：与逾期重复提醒同间隔
		if e.LastNotifiedAt != nil && now.Sub(*e.LastNotifiedAt) < time.Duration(intervalDays)*24*time.Hour {
			continue
		}
		title := "设备报废提醒"
		content := fmt.Sprintf("设备「%s（%s）」已过报废日期（%s），请立即停用并安排更换。", e.Name, e.Code, dateStr(e.ScrapDate))
		recipients := s.userIDsByRoles(*e.TenantID, []string{sysmodel.FieldStaffCode, sysmodel.ProjectAdminCode})
		if len(recipients) > 0 {
			if err := s.notifier.SendBatch(recipients, e.TenantID, MsgTypeScrap, title, content, &e.ID); err != nil {
				logger.L.Warn("设备报废提醒发送失败", zap.String("equipment_id", e.ID), zap.Error(err))
				continue
			}
		}
		admins := s.userIDsByRoles(*e.TenantID, []string{sysmodel.TenantAdminCode})
		if len(admins) > 0 {
			escContent := fmt.Sprintf("设备「%s（%s）」已过报废日期（%s）仍在用，请督促尽快停用更换。", e.Name, e.Code, dateStr(e.ScrapDate))
			if err := s.notifier.SendBatch(admins, e.TenantID, MsgTypeScrap, "设备报废升级", escContent, &e.ID); err != nil {
				logger.L.Warn("设备报废升级提醒发送失败", zap.String("equipment_id", e.ID), zap.Error(err))
			}
		}
		if err := s.db.Model(e).Update("last_notified_at", now).Error; err != nil {
			logger.L.Warn("设备提醒时间回写失败", zap.String("equipment_id", e.ID), zap.Error(err))
			continue
		}
		notified++
	}
	return notified, nil
}

// userIDsByRoles 租户内挂指定角色编码（启用角色）且账号启用的用户 ID 列表。
func (s *ExpireService) userIDsByRoles(tenantID string, roleCodes []string) []string {
	var roleIDs []string
	s.db.Model(&sysmodel.SysRole{}).
		Where("code IN ? AND status = ?", roleCodes, sysmodel.StatusEnabled).
		Pluck("id", &roleIDs)
	if len(roleIDs) == 0 {
		return nil
	}
	// role_ids 为 jsonb 数组：任一角色命中即可
	conds := make([]string, 0, len(roleIDs))
	args := make([]any, 0, len(roleIDs))
	for _, rid := range roleIDs {
		conds = append(conds, "role_ids @> ?::jsonb")
		args = append(args, fmt.Sprintf(`["%s"]`, rid))
	}
	var userIDs []string
	s.db.Model(&sysmodel.SysUser{}).
		Where("tenant_id = ? AND status = ?", tenantID, sysmodel.StatusEnabled).
		Where(strings.Join(conds, " OR "), args...).
		Pluck("id", &userIDs)
	return userIDs
}
