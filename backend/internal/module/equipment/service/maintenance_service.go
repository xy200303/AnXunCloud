package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/equipment/dto"
	"anxuncloud/internal/module/equipment/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"

	"go.uber.org/zap"
)

// 站内消息类型
const (
	MsgTypeMaintReject  = "equipment_maint_reject"  // 维保登记驳回
	MsgTypeMaintPending = "equipment_maint_pending" // 维保待确认（巡检员拍标签提交后通知经理）
	MsgTypeExpire       = "equipment_expire"        // 设备到期提醒
	MsgTypeScrap        = "equipment_scrap"         // 设备报废提醒（v1.7 报废催办链）
)

// 台账补录（ledger_fix）随单补录日期存 payload 列：{manufacture_date, last_maintenance_date}（YYYY-MM-DD），
// 确认时解析回写台账；与 AI 预检的 ai_verdict/ai_reason 互不干扰。
const (
	ledgerKeyManufacture = "manufacture_date"
	ledgerKeyLastMaint   = "last_maintenance_date"
)

// 合法维保类型
var maintenanceTypes = map[string]bool{
	model.MaintenanceRepair:  true,
	model.MaintenanceKeep:    true,
	model.MaintenanceInspect: true,
	model.MaintenanceReplace: true,
	model.MaintenanceLedger:  true,
}

// MaintenanceService 维保登记与确认链服务（台账唯一写入口：confirmed 才回写）。
type MaintenanceService struct {
	db       *gorm.DB
	notifier *notify.Notifier
	aiCli    *ai.Client // AI 预检（可空：未装配/未启用时 ai_verdict 留 NULL）
}

func NewMaintenanceService(db *gorm.DB, notifier *notify.Notifier, aiCli *ai.Client) *MaintenanceService {
	return &MaintenanceService{db: db, notifier: notifier, aiCli: aiCli}
}

// checkRegisterAccess 登记侧访问校验：field_staff 为 self 档（CheckCommunity 一律拒绝），
// 登记是本人动作，退化为租户边界校验（设备租户 = 本人租户）；管理档照常走 CheckCommunity。
func checkRegisterAccess(db *gorm.DB, c *gin.Context, e *model.Equipment) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil
	}
	if identity.ScopeSelf && !identity.SuperAdmin {
		if e.TenantID == nil || *e.TenantID != identity.TenantID {
			return errs.ErrDataScope
		}
		return nil
	}
	return middleware.CheckCommunity(db, c, e.CommunityID)
}

// Register 维保登记（一键+一拍）：照片必传且归属本人；台账不直接更新，confirmed 才回写。
// v2.0 打卡融合同步化：AI 可用且非标签缺失登记时，落库后同步核验（超时 ai.sync_timeout_seconds 降级）——
// 核验结论同步写入 ai_verdict/ai_reason 供审批参考；明显不合格（质量/内容判不像）建单前 43107 拦截。
// v2.1 通用审批引擎：是否生效/进谁的队列由维保审核链（approval_flow flow_code=maint_review）驱动——
// 空流程 = 登记即生效回写台账；AI 环节 = 闸门（可信/存疑三分支路由可配）；人工环节 = pending 待确认。
// 内置默认链由 equipment.ai_auto_confirm 映射（开=[AI 闸门]，关=[经理确认]，甲方口径默认关）。
// 返回 confirmed=true 表示已自动生效（App 反馈「已生效」/「已提交待确认」用）。
func (s *MaintenanceService) Register(c *gin.Context, req *dto.MaintenanceRegisterReq) (string, bool, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return "", false, errs.ErrUnauthorized
	}
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", req.EquipmentID).Error; err != nil {
		return "", false, errs.ErrNotFound
	}
	if be := checkRegisterAccess(s.db, c, &e); be != nil {
		return "", false, be
	}
	if e.Status == model.StatusScrapped {
		return "", false, errs.ErrConflict.WithMsg("设备已报废，不可登记维保")
	}
	maintType := req.MaintenanceType
	if maintType == "" {
		maintType = model.MaintenanceRepair
	}
	if !maintenanceTypes[maintType] {
		return "", false, errs.ErrParam.WithMsg("maintenance_type 取值非法（repair/maintain/inspect/replace/ledger_fix）")
	}
	// 维保日期：缺省今天（巡检员可改为标签上的实际日期）
	maintDate := truncateDay(time.Now())
	if strings.TrimSpace(req.MaintenanceDate) != "" {
		d, be := parseDate(req.MaintenanceDate)
		if be != nil {
			return "", false, be
		}
		maintDate = *d
	}
	// 照片：至少 1 张，逐一上传确认（存在且归属本人，与 checkin_service 同口径）
	if len(req.FileIDs) > 9 {
		return "", false, errs.ErrParam.WithMsg("照片最多 9 张")
	}
	for _, fid := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, fid)
		if err != nil || f.UserID != identity.UserID {
			return "", false, errs.ErrPhotoNotUploaded
		}
	}
	operatorName := strings.TrimSpace(req.OperatorName)
	if operatorName == "" {
		operatorName = identity.Name
	}
	m := model.EquipmentMaintenance{
		TenantID:        e.TenantID,
		EquipmentID:     e.ID,
		MaintenanceType: maintType,
		MaintenanceDate: maintDate,
		OperatorName:    operatorName,
		Note:            strings.TrimSpace(req.Note),
		FileIDs:         types.IDArray(req.FileIDs),
		LabelMissing:    req.LabelMissing, // 标签缺失登记：照片拍设备本体作证，日期免填
		ConfirmStatus:   model.ConfirmPending,
		CreatedBy:       identity.UserID,
	}
	if v := strings.TrimSpace(req.Vendor); v != "" {
		m.Vendor = &v
	}
	// 台账补录：允许随单提交出厂/最近维保日期（确认时一并回写台账），暂存 payload 列
	if maintType == model.MaintenanceLedger {
		manufacture, be := parseDate(req.ManufactureDate)
		if be != nil {
			return "", false, be
		}
		lastMaint, be := parseDate(req.LastMaintenanceDate)
		if be != nil {
			return "", false, be
		}
		if manufacture != nil || lastMaint != nil {
			m.Payload = types.JSONMap{
				ledgerKeyManufacture: dateStr(manufacture),
				ledgerKeyLastMaint:   dateStr(lastMaint),
			}
		}
	}
	// 同步 AI 核验在建单之前（标签缺失登记无日期可读，跳过同步走异步）：明显不合格
	// （质量不达标/内容判不像）直接 43107 拦截不落库——与打卡质量拦截同口径，
	// 烂照片不应生成待确认流水进经理队列；调用失败降级为建单后异步预检兜底。
	syncChecked := false
	syncFailed := false
	syncVerdict, syncReason, syncReading := "", "", ""
	if s.aiCli != nil && s.aiCli.Enabled() && !m.LabelMissing {
		timeout := time.Duration(cfgInt(s.db, "ai.sync_timeout_seconds", 15)) * time.Second
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		verdict, reason, reading, blocked, aiErr := CheckMaintenanceLabel(ctx, s.aiCli, s.db, e.Name, e.Type, m.FileIDs, false)
		cancel()
		switch {
		case aiErr != nil:
			syncFailed = true
		case blocked:
			return "", false, errs.ErrPhotoQuality.WithMsg("照片未通过系统核验（" + reason + "），请重新拍摄")
		default:
			syncChecked = true
			syncVerdict, syncReason, syncReading = verdict, truncateStr2(reason, 500), reading
			m.AIVerdict = &syncVerdict
			m.AIReason = &syncReason
		}
	}
	if err := s.db.Create(&m).Error; err != nil {
		return "", false, errs.ErrInternal
	}
	// AI 结论与可信判定（供审核链路由与经理审批参考）
	outcome := sysmodel.AIGateReview // 未核验/存疑 → 存疑桶（转人工方向）
	switch {
	case s.aiCli == nil || !s.aiCli.Enabled():
		// AI 未启用：不预检（ai_verdict 留 NULL，ConfirmList 排后）
	case m.LabelMissing:
		// 标签缺失登记无可读日期，不同步核验：维持异步预检（只验证照片有效性）
		go s.aiPreCheck(m.ID)
	case syncFailed || !syncChecked:
		// 同步核验调用失败：异步预检兜底（写结论供经理参考，不改变路由）
		logger.L.Warn("维保登记同步 AI 核验失败，转异步预检兜底", zap.String("rec_id", m.ID))
		go s.aiPreCheck(m.ID)
	default:
		reason := syncReason
		trusted, suspectReplace, trustNote := JudgeLabelTrust(syncVerdict, syncReading, m.MaintenanceDate, e.ManufactureDate)
		if trusted {
			outcome = sysmodel.AIGatePass
			s.db.Model(&m).Updates(map[string]any{
				"ai_verdict": model.AIVerdictPass,
				"ai_reason":  truncateStr2(appendReason(reason, "AI 核对通过"), 500),
			})
			break
		}
		// 存疑/读不出兜底：同步写入结论（疑似更换标注在 ai_reason）
		if suspectReplace {
			reason = appendReason(reason, trustNote)
		} else if syncVerdict == model.AIVerdictPass {
			reason = appendReason(reason, "未读出可信维修日期，待人工确认")
		}
		s.db.Model(&m).Updates(map[string]any{
			"ai_verdict": model.AIVerdictReview,
			"ai_reason":  truncateStr2(reason, 500),
		})
	}
	// 审核链路由（maint_review，与打卡链同一引擎）：空流程 = 登记即生效回写台账；
	// AI 环节 = 闸门（三分支可配）；人工环节 = pending 待确认（通知环节名单，空名单回落角色兜底）
	return s.routeMaintenance(&e, &m, outcome, 0)
}

// routeMaintenance 维保审核链路由（communitysvc.WalkFlow）：
// Finish → 自动确认 + 回写台账（空流程 confirm_mode=auto，AI 闸门 confirm_mode=ai）；
// Reject → 驳回并通知登记人；pending → 落定环节（confirm_step）+ 通知环节名单（兜底回落角色通知）。
// startIdx > 0 用于人工环节推进到 AI 环节的续走（Confirm 链路）。
// 返回（记录ID, 是否已生效, 错误）。
func (s *MaintenanceService) routeMaintenance(e *model.Equipment, m *model.EquipmentMaintenance, outcome string, startIdx int) (string, bool, *errs.Error) {
	flow := communitysvc.ResolveFlow(s.db, e.CommunityID, sysmodel.FlowMaintReview)
	// 流程快照固化：登记时点的链落记录，审核全程按快照推进（改流程只影响新单；空流程无冻结语义）
	if len(flow) > 0 {
		m.FlowSnapshot = flow
		s.db.Model(m).Update("flow_snapshot", flow)
	}
	walk := communitysvc.WalkFlow(flow, startIdx, outcome, false)
	switch {
	case walk.Finish:
		if be := s.confirmAuto(m, len(flow) == 0); be != nil {
			return "", false, be
		}
		return m.ID, true, nil
	case walk.Reject:
		reason := "AI 核对不通过"
		if m.AIReason != nil && *m.AIReason != "" {
			reason = *m.AIReason
		}
		if err := s.db.Model(m).Updates(map[string]any{
			"confirm_status": model.ConfirmRejected, "reject_reason": truncateStr2(reason, 255),
		}).Error; err != nil {
			return "", false, errs.ErrInternal
		}
		s.notifyMaintRejected(e, m, reason)
		return m.ID, false, nil
	default: // pending
		if walk.Step > 0 {
			s.db.Model(m).Update("confirm_step", walk.Step)
		}
		if walk.NotifyIdx >= 0 && !walk.Fallback {
			s.notifyMaintStep(e, m, flow, walk.NotifyIdx)
		} else {
			// 兜底：角色通知（项目经理/租户管理员，甲方口径现状）
			s.notifyPendingConfirm(e, m)
		}
		return m.ID, false, nil
	}
}

// confirmAuto 自动确认（AI 闸门通过 / 空流程直通）：confirmed + 台账回写（幂等跳过由 ApplyLedgerWriteback 保证）。
func (s *MaintenanceService) confirmAuto(m *model.EquipmentMaintenance, emptyFlow bool) *errs.Error {
	mode := model.ConfirmModeAI
	if emptyFlow {
		mode = model.ConfirmModeAuto
	}
	return s.confirmByAI(m, mode)
}

// notifyMaintStep 通知环节名单（空名单回落角色兜底，与 notifyPendingConfirm 同口径）。
func (s *MaintenanceService) notifyMaintStep(e *model.Equipment, m *model.EquipmentMaintenance, flow types.FlowStepArray, stepIdx int) {
	recipients := communitysvc.SlotUserIDs(s.db, e.CommunityID, flow[stepIdx].Slot)
	if len(recipients) == 0 || e.TenantID == nil {
		s.notifyPendingConfirm(e, m)
		return
	}
	content := fmt.Sprintf("设备「%s（%s）」已拍新标签提交维保登记（经办：%s），待您执行「%s」。", e.Name, e.Code, m.OperatorName, flow[stepIdx].Name)
	if err := s.notifier.SendBatch(recipients, e.TenantID, MsgTypeMaintPending, "维保待确认", content, &m.ID); err != nil {
		logger.L.Warn("维保待确认通知发送失败", zap.String("maintenance_id", m.ID), zap.Error(err))
	}
}

// notifyMaintRejected 驳回通知登记人（Reject 与 AI 闸门自动打回共用）。
func (s *MaintenanceService) notifyMaintRejected(e *model.Equipment, m *model.EquipmentMaintenance, reason string) {
	content := fmt.Sprintf("设备「%s（%s）」的维保登记被驳回：%s。请重新登记。", e.Name, e.Code, reason)
	if err := s.notifier.Send(m.CreatedBy, MsgTypeMaintReject, "维保登记被驳回", content, &m.ID); err != nil {
		logger.L.Warn("维保驳回通知发送失败", zap.String("maintenance_id", m.ID), zap.Error(err))
	}
}

// notifyPendingConfirm 维保待确认通知：接收人为该设备租户内 project_admin/tenant_admin 角色的启用账号
// （与 expire_job 接收人解析同口径）；发送失败仅记日志不影响登记。
func (s *MaintenanceService) notifyPendingConfirm(e *model.Equipment, m *model.EquipmentMaintenance) {
	if s.notifier == nil || e.TenantID == nil {
		return
	}
	recipients := UserIDsByRoleCodes(s.db, *e.TenantID, []string{sysmodel.ProjectAdminCode, sysmodel.TenantAdminCode})
	if len(recipients) == 0 {
		return
	}
	content := fmt.Sprintf("设备「%s（%s）」已拍新标签提交维保登记（经办：%s），请到维保确认页核实或安排整改。", e.Name, e.Code, m.OperatorName)
	if err := s.notifier.SendBatch(recipients, e.TenantID, MsgTypeMaintPending, "维保待确认", content, &m.ID); err != nil {
		logger.L.Warn("维保待确认通知发送失败", zap.String("maintenance_id", m.ID), zap.Error(err))
	}
}

// appendReason 追加理由（分号连接，忽略空段）。
func appendReason(base, add string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return add
	}
	return base + "；" + add
}

// confirmByAI 自动确认：同事务 confirmed（confirmed_by 置空）+ 台账回写。
// mode：ConfirmModeAI（AI 闸门通过）/ ConfirmModeAuto（空审核链默认直通）。
// 幂等：台账 last_maintenance_date 已被更新的记录推进 → ApplyLedgerWriteback 跳过回写，流水保留并注明。
func (s *MaintenanceService) confirmByAI(m *model.EquipmentMaintenance, mode string) *errs.Error {
	rules, _ := NewEquipmentService(s.db).typeRules()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var e model.Equipment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&e, "id = ?", m.EquipmentID).Error; err != nil {
			return errs.ErrInternal
		}
		now := time.Now()
		updates := map[string]any{
			"confirm_status": model.ConfirmConfirmed,
			"confirmed_by":   nil, // 自动确认无人工确认人
			"confirmed_at":   now,
			"confirm_mode":   mode,
		}
		if mode == model.ConfirmModeAI {
			updates["ai_verdict"] = model.AIVerdictPass
		}
		if err := tx.Model(&model.EquipmentMaintenance{}).Where("id = ?", m.ID).Updates(updates).Error; err != nil {
			return errs.ErrInternal
		}
		skipped, werr := ApplyLedgerWriteback(tx, m, &e, rules[e.Type])
		if werr != nil {
			return errs.ErrInternal
		}
		if skipped {
			// 台账已被更新的记录推进：流水保留，注明跳过原因
			note := "台账最近维保日期已不早于本次登记，跳过回写（流水保留）"
			if m.AIReason != nil {
				note = appendReason(*m.AIReason, note)
			}
			if err := tx.Model(&model.EquipmentMaintenance{}).Where("id = ?", m.ID).
				Update("ai_reason", truncateStr2(note, 500)).Error; err != nil {
				return errs.ErrInternal
			}
		}
		m.ConfirmStatus = model.ConfirmConfirmed
		return nil
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return be
		}
		return errs.ErrInternal
	}
	return nil
}

// ledgerFixDates 从 payload 列解析台账补录随单的补录日期。
func ledgerFixDates(m *model.EquipmentMaintenance) (manufacture, lastMaint *time.Time) {
	if len(m.Payload) == 0 {
		return nil, nil
	}
	if v, ok := m.Payload[ledgerKeyManufacture].(string); ok && v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			manufacture = &t
		}
	}
	if v, ok := m.Payload[ledgerKeyLastMaint].(string); ok && v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			lastMaint = &t
		}
	}
	return manufacture, lastMaint
}

// ConfirmResult 批量确认结果。
type ConfirmResult struct {
	Confirmed int      `json:"confirmed"`
	Advanced  int      `json:"advanced"` // 多环节链：通过当前环节、推进下一环节（仍 pending）
	Skipped   int      `json:"skipped"`  // 非 pending（已确认/已驳回）幂等跳过
	NotFound  []string `json:"not_found"`
}

// Confirm 批量确认（按维保审核链 maint_review 推进）：
// 当前环节是人工环节 → RouteHumanPass：末环节 confirmed + 回写台账；非末环节 confirm_step 推进并通知下一环节名单；
// 停在 AI 环节的（兜底人工）直接 confirmed。跨租户记录整条拒绝（不暴露存在性）；已 confirmed 跳过（幂等）。
// 台账回写：last_maintenance_date、ledger_fix 时同时回写 manufacture_date、按类型规则重算 next_due_date 与 scrap_date。
func (s *MaintenanceService) Confirm(c *gin.Context, req *dto.ConfirmReq) (*ConfirmResult, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	result := &ConfirmResult{NotFound: []string{}}
	rules, _ := NewEquipmentService(s.db).typeRules()
	flowCache := map[string]types.FlowStepArray{} // community_id → 审核链（事务内缓存）
	var advances []struct {
		m    *model.EquipmentMaintenance
		flow types.FlowStepArray
		next int
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range req.IDs {
			var m model.EquipmentMaintenance
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", id).Error; err != nil {
				result.NotFound = append(result.NotFound, id)
				continue
			}
			// 跨租户记录：整条批次拒绝（40302，不暴露存在性）
			if be := middleware.CheckTenantRow(c, tx, m.TenantID); be != nil {
				return be
			}
			if m.ConfirmStatus != model.ConfirmPending {
				result.Skipped++
				continue
			}
			var e model.Equipment
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&e, "id = ?", m.EquipmentID).Error; err != nil {
				result.NotFound = append(result.NotFound, id)
				continue
			}
			// 快照优先：记录按登记时固化的流程推进；无快照（存量/空流程）才按小区缓存的现配
			flow := m.FlowSnapshot
			if len(flow) == 0 {
				var ok bool
				flow, ok = flowCache[e.CommunityID]
				if !ok {
					flow = communitysvc.ResolveFlow(s.db, e.CommunityID, sysmodel.FlowMaintReview)
					flowCache[e.CommunityID] = flow
				}
			}
			idx := int(m.ConfirmStep)
			// 人工环节且非末位：推进下一环节（仍 pending）
			if idx < len(flow) && flow[idx].Kind != sysmodel.FlowStepKindAI {
				if d := communitysvc.RouteHumanPass(flow, idx); !d.Finish {
					if err := tx.Model(&m).Updates(map[string]any{
						"confirm_step": d.NextIdx, "confirmed_by": identity.UserID, "confirmed_at": time.Now(),
					}).Error; err != nil {
						return errs.ErrInternal
					}
					result.Advanced++
					mCopy := m
					advances = append(advances, struct {
						m    *model.EquipmentMaintenance
						flow types.FlowStepArray
						next int
					}{&mCopy, flow, d.NextIdx})
					continue
				}
			}
			// 末环节/AI 环节停放/空流程兜底：confirmed + 回写台账
			now := time.Now()
			if err := tx.Model(&m).Updates(map[string]any{
				"confirm_status": model.ConfirmConfirmed,
				"confirmed_by":   identity.UserID,
				"confirmed_at":   now,
				"confirm_mode":   model.ConfirmModeManual,
			}).Error; err != nil {
				return errs.ErrInternal
			}
			// 回写台账（confirmed 流水是台账唯一写入口；幂等跳过由 ApplyLedgerWriteback 保证）
			if _, err := ApplyLedgerWriteback(tx, &m, &e, rules[e.Type]); err != nil {
				return errs.ErrInternal
			}
			result.Confirmed++
		}
		return nil
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return nil, be
		}
		return nil, errs.ErrInternal
	}
	// 推进通知（事务外）：下一环节是 AI 闸门 → 触发闸门路由；人工环节 → 通知名单
	for _, adv := range advances {
		if adv.flow[adv.next].Kind == sysmodel.FlowStepKindAI {
			s.runMaintGate(adv.m, adv.flow, adv.next)
			continue
		}
		var e model.Equipment
		if s.db.Select("code", "name", "community_id", "tenant_id").First(&e, "id = ?", adv.m.EquipmentID).Error == nil {
			s.notifyMaintStep(&e, adv.m, adv.flow, adv.next)
		}
	}
	return result, nil
}

// runMaintGate 维保 AI 闸门（人工环节推进到 AI 环节时触发）：按已有结论走链落定。
// 不再调大模型（登记/修改时已同步核验）；无结论按存疑路由。
func (s *MaintenanceService) runMaintGate(m *model.EquipmentMaintenance, flow types.FlowStepArray, idx int) {
	var cur model.EquipmentMaintenance
	if err := s.db.First(&cur, "id = ?", m.ID).Error; err != nil || cur.ConfirmStatus != model.ConfirmPending {
		return
	}
	outcome := sysmodel.AIGateReview
	if cur.AIVerdict != nil && *cur.AIVerdict == model.AIVerdictPass {
		outcome = sysmodel.AIGatePass
	}
	walk := communitysvc.WalkFlow(flow, idx, outcome, false)
	switch {
	case walk.Finish:
		if be := s.confirmAuto(&cur, false); be != nil {
			logger.L.Warn("维保闸门自动确认失败", zap.String("maintenance_id", cur.ID), zap.String("err", be.Msg))
		}
	case walk.Reject:
		reason := "AI 核对不通过"
		if cur.AIReason != nil && *cur.AIReason != "" {
			reason = *cur.AIReason
		}
		s.db.Model(&cur).Updates(map[string]any{
			"confirm_status": model.ConfirmRejected, "reject_reason": truncateStr2(reason, 255),
		})
		var e model.Equipment
		if s.db.Select("code", "name").First(&e, "id = ?", cur.EquipmentID).Error == nil {
			s.notifyMaintRejected(&e, &cur, reason)
		}
	default:
		if walk.Step != idx {
			s.db.Model(&cur).Update("confirm_step", walk.Step)
		}
		if walk.NotifyIdx >= 0 && !walk.Fallback {
			var e model.Equipment
			if s.db.Select("code", "name", "community_id", "tenant_id").First(&e, "id = ?", cur.EquipmentID).Error == nil {
				s.notifyMaintStep(&e, &cur, flow, walk.NotifyIdx)
			}
		} else {
			var e model.Equipment
			if s.db.Select("code", "name", "community_id", "tenant_id").First(&e, "id = ?", cur.EquipmentID).Error == nil {
				s.notifyPendingConfirm(&e, &cur)
			}
		}
	}
}

// ApplyLedgerWriteback 台账回写（confirmed 流水是台账唯一写入口；Confirm 与 AI 自动确认/打卡融合链路共用）：
// last_maintenance_date、ledger_fix 随单补录 manufacture_date、按类型规则重算 next_due_date/scrap_date、
// label_missing 标记随确认链流转、清除防打扰提醒标记（确认后恢复全绿，下一周期重新提醒）。
// 幂等（覆盖修改不撤流水）：非补录单且台账 last_maintenance_date 已不早于本次维保日期 → 跳过回写，
// 流水保留，返回 skipped=true；ledger_fix 是纠错单（可能回拨日期），不参与跳过。
// 调用方须已对 equipment 行加锁（FOR UPDATE）。
func ApplyLedgerWriteback(tx *gorm.DB, m *model.EquipmentMaintenance, e *model.Equipment, rule TypeRule) (skipped bool, err error) {
	lastMaint := &m.MaintenanceDate
	updates := map[string]any{}
	if m.MaintenanceType == model.MaintenanceLedger {
		manufacture, fixLast := ledgerFixDates(m)
		if fixLast != nil {
			lastMaint = fixLast
		}
		if manufacture != nil {
			updates["manufacture_date"] = *manufacture
			e.ManufactureDate = manufacture
		}
	} else if e.LastMaintenanceDate != nil && !truncateDay(*e.LastMaintenanceDate).Before(truncateDay(*lastMaint)) {
		return true, nil
	}
	updates["last_maintenance_date"] = *lastMaint
	if nd := CalcNextDueDate(e.ManufactureDate, lastMaint, rule.FirstMonths, rule.CycleMonths); nd != nil {
		updates["next_due_date"] = *nd
	}
	updates["scrap_date"] = CalcScrapDate(e.ManufactureDate, rule.ScrapMonths)
	// 标签缺失标记随确认链流转：标签缺失单确认后打标（退出自动判定）；
	// 正常登记/补录确认后清除（覆盖设备换过标签的场景）
	updates["label_missing"] = m.LabelMissing
	updates["last_notified_at"] = nil
	if err := tx.Model(e).Updates(updates).Error; err != nil {
		return false, err
	}
	return false, nil
}

// Reject 驳回登记：pending→rejected + 理由，通知登记人重新登记（台账不变）。
func (s *MaintenanceService) Reject(c *gin.Context, req *dto.RejectReq) *errs.Error {
	var m model.EquipmentMaintenance
	if err := s.db.First(&m, "id = ?", req.ID).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckTenantRow(c, s.db, m.TenantID); be != nil {
		return be
	}
	if m.ConfirmStatus != model.ConfirmPending {
		return errs.ErrConflict.WithMsg("仅待确认记录可驳回")
	}
	if err := s.db.Model(&m).Updates(map[string]any{
		"confirm_status": model.ConfirmRejected,
		"reject_reason":  strings.TrimSpace(req.Reason),
	}).Error; err != nil {
		return errs.ErrInternal
	}
	// 通知登记人（附设备编号/名称与驳回理由）
	var e model.Equipment
	if s.db.Select("code", "name").First(&e, "id = ?", m.EquipmentID).Error == nil {
		s.notifyMaintRejected(&e, &m, strings.TrimSpace(req.Reason))
	}
	return nil
}

// ConfirmList 待确认分页：ai_verdict='review' 置顶（NULL 视为 pass 排后），
// 带设备编号/名称/点位名/登记人姓名/照片 url。
func (s *MaintenanceService) ConfirmList(c *gin.Context, q *dto.ConfirmListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.EquipmentMaintenance{}).Where("confirm_status = ?", model.ConfirmPending)
	if tid, be := middleware.TenantScopeOrDefault(c, s.db); be != nil {
		return nil, be
	} else if tid != "" {
		db = db.Where("tenant_id = ?", tid)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.EquipmentMaintenance
	offset, limit := q.Normalize()
	if err := db.Order("CASE WHEN ai_verdict = 'review' THEN 0 ELSE 1 END, created_at ASC").
		Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &response.Page{List: s.toMaintenanceItems(rows), Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// History 某设备维保历史（含确认状态与照片凭证）。
func (s *MaintenanceService) History(c *gin.Context, equipmentID string, q *response.PageQuery) (*response.Page, *errs.Error) {
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", equipmentID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := checkRegisterAccess(s.db, c, &e); be != nil {
		return nil, be
	}
	db := s.db.Model(&model.EquipmentMaintenance{}).Where("equipment_id = ?", equipmentID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.EquipmentMaintenance
	offset, limit := q.Normalize()
	if err := db.Order("maintenance_date DESC, created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &response.Page{List: s.toMaintenanceItems(rows), Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// toMaintenanceItems 维保流水装配：设备编号/名称/点位名/登记人姓名/照片 url。
func (s *MaintenanceService) toMaintenanceItems(rows []model.EquipmentMaintenance) []gin.H {
	eqByID := map[string]model.Equipment{}
	{
		ids := make([]string, 0, len(rows))
		for i := range rows {
			ids = append(ids, rows[i].EquipmentID)
		}
		var es []model.Equipment
		s.db.Select("id", "code", "name", "point_id").Where("id IN ?", ids).Find(&es)
		for i := range es {
			eqByID[es[i].ID] = es[i]
		}
	}
	pointNames := map[string]string{}
	{
		ids := []string{}
		for _, e := range eqByID {
			if e.PointID != nil && *e.PointID != "" {
				ids = append(ids, *e.PointID)
			}
		}
		if len(ids) > 0 {
			var ps []insmodel.InspectionPoint
			s.db.Select("id", "name").Where("id IN ?", ids).Find(&ps)
			for _, p := range ps {
				pointNames[p.ID] = p.Name
			}
		}
	}
	userNames := map[string]string{}
	{
		ids := []string{}
		for i := range rows {
			ids = append(ids, rows[i].CreatedBy)
			if rows[i].ConfirmedBy != nil {
				ids = append(ids, *rows[i].ConfirmedBy)
			}
		}
		var us []sysmodel.SysUser
		s.db.Select("id", "name").Where("id IN ?", ids).Find(&us)
		for _, u := range us {
			userNames[u.ID] = u.Name
		}
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		m := &rows[i]
		e := eqByID[m.EquipmentID]
		photos := make([]gin.H, 0, len(m.FileIDs))
		for fid, f := range uploadfile.ByIDs(s.db, m.FileIDs) {
			photos = append(photos, gin.H{"file_id": fid, "url": f.URL})
		}
		item := gin.H{
			"id": m.ID, "equipment_id": m.EquipmentID,
			"equipment_code": e.Code, "equipment_name": e.Name,
			"maintenance_type": m.MaintenanceType,
			"maintenance_date": dateStr(&m.MaintenanceDate),
			"vendor":           m.Vendor, "operator_name": m.OperatorName, "note": m.Note,
			"photos":         photos,
			"confirm_status": m.ConfirmStatus, "reject_reason": m.RejectReason,
			"label_missing": m.LabelMissing,
			"ai_verdict": m.AIVerdict, "ai_reason": m.AIReason,
			"created_by": m.CreatedBy, "created_by_name": userNames[m.CreatedBy],
			"created_at": timefmt.T(m.CreatedAt),
		}
		if e.PointID != nil {
			item["point_id"] = *e.PointID
			item["point_name"] = pointNames[*e.PointID]
		}
		if m.ConfirmedBy != nil {
			item["confirmed_by"] = *m.ConfirmedBy
			item["confirmed_by_name"] = userNames[*m.ConfirmedBy]
			item["confirmed_at"] = timefmt.TP(m.ConfirmedAt)
		}
		// 台账补录：补录日期显式透出（确认页核对用）
		if m.MaintenanceType == model.MaintenanceLedger {
			manufacture, lastMaint := ledgerFixDates(m)
			item["fix_manufacture_date"] = dateStr(manufacture)
			item["fix_last_maintenance_date"] = dateStr(lastMaint)
		}
		list = append(list, item)
	}
	return list
}

// Mine 我提交的维保登记分页（全部状态，最新在前；巡检员「我的提交」用——
// 提交后设备因有 pending 流水会从待维保列表消失，这里给登记人一个可见、可修改的入口）。
func (s *MaintenanceService) Mine(c *gin.Context, q *response.PageQuery) (*response.Page, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	db := s.db.Model(&model.EquipmentMaintenance{}).Where("created_by = ?", identity.UserID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.EquipmentMaintenance
	offset, limit := q.Normalize()
	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return &response.Page{List: s.toMaintenanceItems(rows), Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Update 待确认登记修改（限本人 + pending；已确认/已驳回不可改——已生效的回写台账，驳回的请重新登记）。
// 照片逐一校验归属本人；非标签缺失单且 AI 可用时重新同步核验：明显不合格（质量/内容判不像）
// 43107 拦截不落库（与 Register 同口径）；可信且 equipment.ai_auto_confirm 开则自动 confirmed 回写台账，
// 否则保持 pending 并刷新 ai_verdict/ai_reason 供经理确认参考（不重复通知经理，避免骚扰）。
// 返回 confirmed=true 表示本次修改后已自动生效。
func (s *MaintenanceService) Update(c *gin.Context, id string, req *dto.MaintenanceUpdateReq) (bool, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return false, errs.ErrUnauthorized
	}
	var m model.EquipmentMaintenance
	if err := s.db.First(&m, "id = ?", id).Error; err != nil {
		return false, errs.ErrNotFound
	}
	if m.CreatedBy != identity.UserID {
		return false, errs.ErrDataScope.WithMsg("只能修改本人提交的维保登记")
	}
	if m.ConfirmStatus != model.ConfirmPending {
		return false, errs.ErrConflict.WithMsg("该登记已被经理处理，不可修改；如需变更请重新拍照登记")
	}
	if len(req.FileIDs) > 9 {
		return false, errs.ErrParam.WithMsg("照片最多 9 张")
	}
	for _, fid := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, fid)
		if err != nil || f.UserID != identity.UserID {
			return false, errs.ErrPhotoNotUploaded
		}
	}
	var e model.Equipment
	if err := s.db.First(&e, "id = ?", m.EquipmentID).Error; err != nil {
		return false, errs.ErrNotFound
	}
	updates := map[string]any{
		"file_ids": types.IDArray(req.FileIDs),
		"note":     strings.TrimSpace(req.Note),
	}
	m.FileIDs = types.IDArray(req.FileIDs)
	m.Note = strings.TrimSpace(req.Note)
	if strings.TrimSpace(req.MaintenanceDate) != "" {
		d, be := parseDate(req.MaintenanceDate)
		if be != nil {
			return false, be
		}
		updates["maintenance_date"] = *d
		m.MaintenanceDate = *d
	}
	// 照片变更重新同步 AI 核验（标签缺失登记无日期可读，保持原结论）
	aiVerdict := ""
	if s.aiCli != nil && s.aiCli.Enabled() && !m.LabelMissing {
		timeout := time.Duration(cfgInt(s.db, "ai.sync_timeout_seconds", 15)) * time.Second
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		verdict, reason, reading, blocked, aiErr := CheckMaintenanceLabel(ctx, s.aiCli, s.db, e.Name, e.Type, m.FileIDs, false)
		cancel()
		switch {
		case aiErr != nil:
			// 调用失败：清掉旧结论，经理审批时不被过期结论误导
			updates["ai_verdict"] = nil
			updates["ai_reason"] = nil
		case blocked:
			return false, errs.ErrPhotoQuality.WithMsg("照片未通过系统核验（" + reason + "），请重新拍摄")
		default:
			reason = truncateStr2(reason, 500)
			trusted, suspectReplace, trustNote := JudgeLabelTrust(verdict, reading, m.MaintenanceDate, e.ManufactureDate)
			if trusted {
				aiVerdict = model.AIVerdictPass
				updates["ai_verdict"] = aiVerdict
				updates["ai_reason"] = truncateStr2(appendReason(reason, "AI 核对通过，待经理确认"), 500)
			} else {
				if suspectReplace {
					reason = appendReason(reason, trustNote)
				} else if verdict == model.AIVerdictPass {
					reason = appendReason(reason, "未读出可信维修日期，待人工确认")
				}
				aiVerdict = model.AIVerdictReview
				updates["ai_verdict"] = aiVerdict
				updates["ai_reason"] = truncateStr2(reason, 500)
			}
		}
	}
	if err := s.db.Model(&model.EquipmentMaintenance{}).Where("id = ?", m.ID).Updates(updates).Error; err != nil {
		return false, errs.ErrInternal
	}
	// 审核链路由（与 Register 同引擎）：AI 环节按结论重走路由；人工环节保持 pending 不重复通知（避免骚扰）
	if aiVerdict == model.AIVerdictPass || aiVerdict == model.AIVerdictReview {
		flow := communitysvc.ResolveFlow(s.db, e.CommunityID, sysmodel.FlowMaintReview)
		walk := communitysvc.WalkFlow(flow, int(m.ConfirmStep), gateOutcomeOf(aiVerdict), false)
		switch {
		case walk.Finish:
			if be := s.confirmAuto(&m, len(flow) == 0); be != nil {
				return false, be
			}
			return true, nil
		case walk.Reject:
			reason := "AI 核对不通过"
			if m.AIReason != nil && *m.AIReason != "" {
				reason = *m.AIReason
			}
			s.db.Model(&m).Updates(map[string]any{
				"confirm_status": model.ConfirmRejected, "reject_reason": truncateStr2(reason, 255),
			})
			s.notifyMaintRejected(&e, &m, reason)
		case walk.Step > 0:
			s.db.Model(&m).Update("confirm_step", walk.Step)
		}
	}
	return false, nil
}

// gateOutcomeOf 维保 AI 结论 → 闸门票仓（pass=可信；其余=存疑转人工方向）。
func gateOutcomeOf(aiVerdict string) string {
	if aiVerdict == model.AIVerdictPass {
		return sysmodel.AIGatePass
	}
	return sysmodel.AIGateReview
}

// DueDevices mp 端待维保设备列表：本租户临期+逾期+应报废在用设备（带点位/小区名，逾期/报废在前）。
// 标签缺失设备不出现（已退出自动判定，走经理处置通道）。
func (s *MaintenanceService) DueDevices(c *gin.Context) ([]gin.H, *errs.Error) {
	tid, be := middleware.TenantScopeOrDefault(c, s.db)
	if be != nil {
		return nil, be
	}
	globalWarn := cfgInt(s.db, "equipment.expire_warn_days", 30)
	today := truncateDay(time.Now())
	db := s.db.Model(&model.Equipment{}).
		Where("status = ? AND label_missing = ?", model.StatusInService, false).
		Where("(next_due_date IS NOT NULL AND next_due_date <= ?::date + COALESCE(warn_days, ?)) OR (scrap_date IS NOT NULL AND scrap_date <= ?)",
			today.Format("2006-01-02"), globalWarn, today.Format("2006-01-02")).
		Where("NOT EXISTS (SELECT 1 FROM equipment_maintenance m WHERE m.equipment_id = equipment.id AND m.confirm_status = ?)", model.ConfirmPending)
	if tid != "" {
		db = db.Where("tenant_id = ?", tid)
	}
	// 只列启用催办的类型（电梯等外包合同驱动型不催办）；报废设备不受类型催办开关限制（违规使用风险）
	rules, _ := NewEquipmentService(s.db).typeRules()
	remindTypes := make([]string, 0, len(rules))
	for t, r := range rules {
		if r.Remind {
			remindTypes = append(remindTypes, t)
		}
	}
	if len(remindTypes) == 0 {
		return []gin.H{}, nil
	}
	var rows []model.Equipment
	if err := db.Where("type IN ? OR scrap_date <= ?", remindTypes, today.Format("2006-01-02")).
		Order("next_due_date ASC NULLS FIRST").Limit(200).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := NewEquipmentService(s.db).toItems(rows)
	for i := range rows {
		if rows[i].NextDueDate != nil {
			items[i]["overdue_days"] = int(today.Sub(truncateDay(*rows[i].NextDueDate)).Hours() / 24)
		}
	}
	return items, nil
}
