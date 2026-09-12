package service

// 打卡审核 AI 闸门（通用审批引擎的打卡侧执行器）。
// 流程语义（approval_flow.steps，kind='ai' 环节）：
//   - 空流程 = 默认通过；AI 环节按判定结果三分支路由（无异常/有异常/存疑），
//     每个去向可配 finish（直接生效）/next（下一环节）/reject（直接打回）/goto:N（跳到第 N 环节）；
//   - 强制人工（上报待处理/台账判异常/强制提交/AI 不可用）按「存疑」桶路由；
//   - 走链逻辑 = communitysvc.WalkFlow（与维保链共用）；落定走条件更新
//     （audit_status=pending AND audit_step=当前环节），人工已介入则闸门静默退出。

import (
	"context"
	"fmt"
	"time"

	communitysvc "anxuncloud/internal/module/community/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/types"

	"go.uber.org/zap"
)

// gateBucketOf 记录级 AI 结论 → 闸门票仓（pass/abnormal/review）。
func gateBucketOf(verdict string) string {
	switch verdict {
	case insmodel.AIVerdictPass:
		return sysmodel.AIGatePass
	case insmodel.AIVerdictAbnormal:
		return sysmodel.AIGateAbnormal
	default:
		return sysmodel.AIGateReview
	}
}

// RunAIGate AI 闸门入口（review_service 推进到 AI 环节时经注入回调调用；异步 goroutine 内运行）。
func (s *CheckinService) RunAIGate(recID string) {
	defer func() {
		if r := recover(); r != nil {
			logger.L.Error("打卡 AI 闸门 panic", zap.String("rec_id", recID), zap.Any("panic", r))
		}
	}()
	s.runAIGate(recID)
}

// runAIGate 闸门执行：取/补 AI 结论 → 走链 → 条件更新落定（人工已介入则静默退出）。
func (s *CheckinService) runAIGate(recID string) {
	var rec insmodel.CheckinRecord
	if err := s.db.First(&rec, "id = ?", recID).Error; err != nil {
		return
	}
	if rec.AuditStatus != insmodel.AuditPending {
		return // 人工已处理
	}
	flow := communitysvc.FlowOrResolve(s.db, rec.FlowSnapshot, rec.CommunityID, sysmodel.FlowCheckinReview)
	idx := int(rec.AuditStep)
	if idx >= len(flow) || flow[idx].Kind != sysmodel.FlowStepKindAI {
		return // 不在 AI 环节（流程配置已变更等）
	}
	var point insmodel.InspectionPoint
	s.db.Select("id", "name", "type").First(&point, "id = ?", rec.PointID)

	outcome := ""
	forced := false
	verdict := rec.AIVerdict
	if s.hasReportPendingItem(recID) {
		outcome = sysmodel.AIGateReview // 上报待处理强制人工
		forced = true
	} else if verdict != "" {
		outcome = gateBucketOf(verdict)
	} else {
		// 无结论：AI 可用则现判（异步上下文，不阻塞打卡请求），不可用按存疑转人工
		verdict, outcome = s.gateJudge(&rec, &point)
	}
	walk := communitysvc.WalkFlow(flow, idx, outcome, forced)
	s.settleGate(&rec, &point, idx, walk, rec.AIReason)
}

// gateJudge 闸门关内现判：调大模型并回写逐项/记录级结论（条件更新防人工竞态）。
// 返回记录级 verdict 与票仓；调用失败/AI 未启用 → review 桶。
func (s *CheckinService) gateJudge(rec *insmodel.CheckinRecord, point *insmodel.InspectionPoint) (string, string) {
	if s.aiCli == nil || !s.aiCli.Enabled() {
		s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ?", rec.ID, insmodel.AuditPending).
			Updates(map[string]any{"ai_verdict": insmodel.AIVerdictError, "ai_reason": "AI 未启用，转人工审核"})
		rec.AIReason = "AI 未启用，转人工审核"
		return insmodel.AIVerdictError, sysmodel.AIGateReview
	}
	var items []insmodel.CheckinRecordItem
	s.db.Where("record_id = ?", rec.ID).Order("sort ASC").Find(&items)
	timeout := time.Duration(s.cfgInt("ai.sync_timeout_seconds", 15)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	res, err := s.aiCli.ReviewCheckin(ctx, s.buildReviewInput(point, items, rec.Remark))
	cancel()
	if err != nil {
		logger.L.Warn("闸门 AI 判定失败", zap.String("rec_id", rec.ID), zap.Error(err))
		s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ?", rec.ID, insmodel.AuditPending).
			Updates(map[string]any{"ai_verdict": insmodel.AIVerdictError, "ai_reason": truncateStr(err.Error(), 200)})
		rec.AIReason = truncateStr(err.Error(), 200)
		return insmodel.AIVerdictError, sysmodel.AIGateReview
	}
	writeItemVerdicts(s.db, rec.ID, res.Items)
	// 记录级结论：整体 review 或存在存疑项 → review；有明确异常项 → abnormal；否则 pass
	verdict := res.Verdict
	hasAbnormal := false
	for _, iv := range res.Items {
		if iv.Verdict == insmodel.AIVerdictAbnormal {
			hasAbnormal = true
		}
		if iv.Verdict == insmodel.AIVerdictReview {
			verdict = insmodel.AIVerdictReview
		}
	}
	if verdict == insmodel.AIVerdictPass && hasAbnormal {
		verdict = insmodel.AIVerdictAbnormal
	}
	s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ?", rec.ID, insmodel.AuditPending).
		Updates(map[string]any{
			"ai_verdict": verdict, "ai_reason": truncateStr(res.Reason, 500),
			"ai_quality_pass": res.Quality.Pass, "ai_quality_issue": truncateStr(res.Quality.Issue, 255),
		})
	rec.AIReason = truncateStr(res.Reason, 500)
	return verdict, gateBucketOf(verdict)
}

// settleGate 闸门落定（条件更新：pending 且仍在原环节才生效，人工已介入则静默放弃）。
func (s *CheckinService) settleGate(rec *insmodel.CheckinRecord, point *insmodel.InspectionPoint, idx int, walk communitysvc.WalkResult, routeReason string) {
	switch {
	case walk.Finish:
		s.db.Model(&insmodel.CheckinRecord{}).
			Where("id = ? AND audit_status = ? AND audit_step = ?", rec.ID, insmodel.AuditPending, idx).
			Updates(map[string]any{"audit_status": insmodel.AuditAutoPass, "audit_step": walk.Step})
	case walk.Reject:
		reason := "AI 审核不通过"
		if routeReason != "" {
			reason = routeReason
		}
		res := s.db.Model(&insmodel.CheckinRecord{}).
			Where("id = ? AND audit_status = ? AND audit_step = ?", rec.ID, insmodel.AuditPending, idx).
			Updates(map[string]any{"audit_status": insmodel.AuditRejected, "audit_remark": reason, "audit_at": time.Now()})
		if res.Error == nil && res.RowsAffected > 0 {
			_ = s.notifier.Send(rec.InspectorID, "checkin_audit",
				"打卡记录被打回",
				fmt.Sprintf("你在点位「%s」的打卡记录经 AI 审核不通过：%s。请核实后按要求补巡。", point.Name, reason),
				&rec.ID)
		}
	default: // pending：推进到人工环节（或原地兜底）
		if walk.Step != idx {
			res := s.db.Model(&insmodel.CheckinRecord{}).
				Where("id = ? AND audit_status = ? AND audit_step = ?", rec.ID, insmodel.AuditPending, idx).
				Updates(map[string]any{"audit_step": walk.Step})
			if res.Error != nil || res.RowsAffected == 0 {
				return // 人工已介入
			}
		}
		if walk.Fallback || walk.NotifyIdx >= 0 {
			s.notifyStepReviewers(rec.ID, point.Name, walk.Step, walk.Fallback, routeReason)
		}
	}
}

// notifyStepReviewers 通知环节审核人：flow[stepIdx] 人工环节 → 该环节名单（汇报线槽位带巡查类型维度路由）；
// AI 环节/下标越界/名单为空 → 汇报线通用槽位兜底。reason 为空表示常规待审。
func (s *CheckinService) notifyStepReviewers(recID, pointName string, stepIdx int, forceFallback bool, reason string) {
	var rec struct {
		CommunityID  string
		TaskID       string
		FlowSnapshot types.FlowStepArray
	}
	if err := s.db.Model(&insmodel.CheckinRecord{}).Select("community_id", "task_id", "flow_snapshot").Where("id = ?", recID).First(&rec).Error; err != nil {
		return
	}
	var task struct {
		PatrolType string
	}
	if err := s.db.Model(&insmodel.InspectionTask{}).Select("patrol_type").Where("id = ?", rec.TaskID).First(&task).Error; err != nil {
		return
	}
	flow := communitysvc.FlowOrResolve(s.db, rec.FlowSnapshot, rec.CommunityID, sysmodel.FlowCheckinReview)
	slot := ""
	if !forceFallback && stepIdx < len(flow) && flow[stepIdx].Kind != sysmodel.FlowStepKindAI {
		slot = communitysvc.FlowStepSlot(s.db, rec.CommunityID, task.PatrolType, flow[stepIdx].Slot)
	}
	if slot == "" {
		slot = communitysvc.FlowStepSlot(s.db, rec.CommunityID, task.PatrolType, sysmodel.SlotPatrolReportLine)
	}
	userIDs := communitysvc.SlotUserIDs(s.db, rec.CommunityID, slot)
	if len(userIDs) == 0 && slot != sysmodel.SlotPatrolReportLine {
		// 维度槽位未配置名单：回落通用汇报线
		userIDs = communitysvc.SlotUserIDs(s.db, rec.CommunityID, sysmodel.SlotPatrolReportLine)
	}
	title := "打卡记录待审核：" + pointName
	content := fmt.Sprintf("点位「%s」的打卡记录待您审核。", pointName)
	if reason != "" {
		title = "打卡记录转人工：" + pointName
		content = fmt.Sprintf("点位「%s」的打卡记录需人工复核。理由：%s", pointName, reason)
	}
	for _, uid := range userIDs {
		_ = s.notifier.Send(uid, "checkin_audit", title, content, &recID)
	}
}
