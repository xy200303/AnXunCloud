package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	eqmodel "anxuncloud/internal/module/equipment/model"
	eqsvc "anxuncloud/internal/module/equipment/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/types"

	"go.uber.org/zap"
)

// 打卡巡检 × 设备维保融合：巡检员看到问题只拍照片——台账有效期合成项携带新标签照片提交时，
// 服务端逐项同步 AI 核验（prompt/解析与维保登记预检同源），结论供经理审批参考；
// 「可信即自动 confirmed 回写台账」由 equipment.ai_auto_confirm 控制（默认关：甲方口径维保一律经理确认），
// 未开启时一律生成 pending 流水进现有维保确认链（ConfirmList）并通知项目经理/租户管理员。

// checkinMaintAction 打卡触发的维保动作：AI 核验在打卡事务前完成（网络调用带超时降级），
// 事务内只落库（persistCheckinMaintenances）；提交成功后由 notifyPendingMaintenances 通知经理。
type checkinMaintAction struct {
	equipmentID   string
	equipName     string
	equipCode     string
	fileIDs       []string
	confirmed     bool   // AI 核验可信且 ai_auto_confirm 开：自动 confirmed + 回写台账（confirm_mode=ai）
	aiVerdict     string // pass/review；空=AI 未启用/调用失败（ai_verdict 落 NULL）
	aiReason      string
	maintenanceID string // 落库后回填（通知 biz_id 用）
}

// resolveCheckinMaintenances 收集带新标签照片的有效期合成项（judge_config 携 equipment_id 快照）
// → 逐项同步 AI 核验 → 生成维保动作：
// 可信（pass 且标签维修年月与当天年月差 ≤1 个月、钢印与台账一致）→ 项翻转正常 + confirmed 动作；
// 存疑/读不出/AI 未启用/超时失败 → 项维持判定 + pending 动作（照常进 ConfirmList）。
// 生成动作的项由服务端回填 disposition=maintenance_registered。
func (s *CheckinService) resolveCheckinMaintenances(ctx context.Context, point *insmodel.InspectionPoint, items []insmodel.CheckinRecordItem) []checkinMaintAction {
	var actions []checkinMaintAction
	for i := range items {
		it := &items[i]
		if it.JudgeType != ai.JudgeEquipmentValidity || len(it.Photos) == 0 {
			continue
		}
		equipmentID, _ := it.JudgeConfig["equipment_id"].(string)
		if equipmentID == "" {
			continue
		}
		var e eqmodel.Equipment
		if err := s.db.First(&e, "id = ?", equipmentID).Error; err != nil {
			logger.L.Warn("打卡维保：设备不存在，跳过", zap.String("equipment_id", equipmentID))
			continue
		}
		action := checkinMaintAction{equipmentID: equipmentID, equipName: e.Name, equipCode: e.Code, fileIDs: []string(it.Photos)}
		if s.aiCli.Enabled() {
			timeout := s.cfgInt("ai.sync_timeout_seconds", 15)
			actx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			verdict, reason, reading, blocked, err := eqsvc.CheckMaintenanceLabel(actx, s.aiCli, s.db, point.Name, point.Type, action.fileIDs, false)
			cancel()
			switch {
			case err != nil:
				// 调用失败/超时：降级 pending（ai_verdict 落 NULL），不阻塞打卡
				logger.L.Warn("打卡维保标签同步核验失败，降级待确认", zap.String("equipment_id", equipmentID), zap.Error(err))
			case blocked:
				// 明显不合格（质量/内容判不像）：硬拦截不落流水——未登记，备注说明，设备维持逾期可重新拍
				it.Note = truncateStr(appendItemNote(it.Note, "新标签照片未通过系统核验（"+reason+"），未登记维保，请重新拍摄"), 512)
				continue
			default:
				trusted, suspectReplace, trustNote := eqsvc.JudgeLabelTrust(verdict, reading, time.Now(), e.ManufactureDate)
				action.aiVerdict, action.aiReason = verdict, reason
				switch {
				case trusted:
					// AI 核对通过：ai_auto_confirm 开才自动生效；默认关（甲方口径）则转经理确认
					if eqsvc.CfgBool(s.db, "equipment.ai_auto_confirm", false) {
						action.confirmed = true
					}
				case suspectReplace:
					action.aiVerdict = eqmodel.AIVerdictReview
					action.aiReason = truncateStr(appendItemNote(reason, trustNote), 500)
				case verdict == eqmodel.AIVerdictPass:
					// 模型判 pass 但未读出可信维修年月：兜底转人工
					action.aiVerdict = eqmodel.AIVerdictReview
					action.aiReason = truncateStr(appendItemNote(reason, "未读出可信维修日期，待人工确认"), 500)
				}
			}
		}
		// 走到这里 = 生成维保流水（blocked 已 continue）；登记的项回填处置方式
		it.Disposition = insmodel.DispositionMaintenanceReg
		if action.confirmed {
			it.Pass = true
			it.Note = truncateStr(appendItemNote(it.Note, "已拍新标签，系统核对通过，维保已生效"), 512)
		} else if action.aiVerdict == eqmodel.AIVerdictPass {
			it.Note = truncateStr(appendItemNote(it.Note, "已拍新标签，系统核对通过，待经理确认"), 512)
		} else {
			it.Note = truncateStr(appendItemNote(it.Note, "已登记维保待确认"), 512)
		}
		actions = append(actions, action)
	}
	return actions
}

// persistCheckinMaintenances 打卡事务内落维保流水（source=checkin、checkin_record_id 关联本次打卡）；
// confirmed 动作同事务回写台账（设备行 FOR UPDATE；台账已被更新的记录推进则跳过回写、流水保留并注明）。
// 覆盖修改（supersede）不删除旧记录已产生的维保流水——维保是真实事件，有照片留痕。
func (s *CheckinService) persistCheckinMaintenances(tx *gorm.DB, rec *insmodel.CheckinRecord, actions []checkinMaintAction, inspectorID string) error {
	if len(actions) == 0 {
		return nil
	}
	operatorName := ""
	var u sysmodel.SysUser
	if tx.Select("name").First(&u, "id = ?", inspectorID).Error == nil {
		operatorName = u.Name
	}
	rules := eqsvc.NewEquipmentService(tx).TypeRules()
	today := truncateDayLocal(time.Now())
	for i := range actions {
		a := &actions[i]
		m := eqmodel.EquipmentMaintenance{
			TenantID:        rec.TenantID,
			EquipmentID:     a.equipmentID,
			MaintenanceType: eqmodel.MaintenanceRepair,
			MaintenanceDate: today,
			OperatorName:    operatorName,
			Note:            "打卡拍新标签自动登记",
			FileIDs:         types.IDArray(a.fileIDs),
			Source:          eqmodel.SourceCheckin,
			CheckinRecordID: &rec.ID,
			ConfirmStatus:   eqmodel.ConfirmPending,
			CreatedBy:       inspectorID,
		}
		if a.aiVerdict != "" {
			v := truncateStr(a.aiVerdict, 16)
			m.AIVerdict = &v
		}
		if a.aiReason != "" {
			r := truncateStr(a.aiReason, 500)
			m.AIReason = &r
		}
		if a.confirmed {
			// AI 可信自动确认：confirmed_by 置空（无人工确认人）
			m.ConfirmStatus = eqmodel.ConfirmConfirmed
			m.ConfirmMode = eqmodel.ConfirmModeAI
			now := time.Now()
			m.ConfirmedAt = &now
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		a.maintenanceID = m.ID
		if !a.confirmed {
			continue
		}
		var e eqmodel.Equipment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&e, "id = ?", a.equipmentID).Error; err != nil {
			return err
		}
		skipped, err := eqsvc.ApplyLedgerWriteback(tx, &m, &e, rules[e.Type])
		if err != nil {
			return err
		}
		if skipped {
			// 台账已被更新的记录推进：流水保留，注明跳过原因
			note := truncateStr(appendItemNote(strVal(m.AIReason), "台账最近维保日期已不早于本次维保，跳过回写（流水保留）"), 500)
			if err := tx.Model(&m).Update("ai_reason", note).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// notifyPendingMaintenances 打卡提交成功后（事务外）：pending 维保流水通知项目经理/租户管理员
// （甲方口径：经理确认或安排整改；接收人解析与 expire_job 同口径）。发送失败仅记日志。
func (s *CheckinService) notifyPendingMaintenances(rec *insmodel.CheckinRecord, actions []checkinMaintAction) {
	if s.notifier == nil || rec.TenantID == nil {
		return
	}
	for i := range actions {
		a := &actions[i]
		if a.confirmed || a.maintenanceID == "" {
			continue
		}
		recipients := eqsvc.UserIDsByRoleCodes(s.db, *rec.TenantID, []string{sysmodel.ProjectAdminCode, sysmodel.TenantAdminCode})
		if len(recipients) == 0 {
			continue
		}
		content := "设备「" + a.equipName + "（" + a.equipCode + "）」已在打卡中拍新标签登记维保，请到维保确认页核实或安排整改。"
		if err := s.notifier.SendBatch(recipients, rec.TenantID, eqsvc.MsgTypeMaintPending, "维保待确认", content, &a.maintenanceID); err != nil {
			logger.L.Warn("打卡维保待确认通知发送失败", zap.String("maintenance_id", a.maintenanceID), zap.Error(err))
		}
	}
}

// appendItemNote 追加备注/理由（分号连接，忽略空段）。
func appendItemNote(base, add string) string {
	base = strings.TrimSpace(base)
	add = strings.TrimSpace(add)
	if base == "" {
		return add
	}
	if add == "" {
		return base
	}
	return base + "；" + add
}

// truncateDayLocal 按本地时区截断到日（maintenance_date 入库日粒度，与 equipment 模块 truncateDay 同口径）。
func truncateDayLocal(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}
