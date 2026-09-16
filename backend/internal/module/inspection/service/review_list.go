package service

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

// reviewBatchCtx 审核列表批量预载上下文（消除 reviewItem 逐行 N+1：
// point/community/user 各 1 次 IN 查询，逐项与照片各 1 次 IN 查询，审批链按小区缓存）。
type reviewBatchCtx struct {
	pointNames map[string]string
	commNames  map[string]string
	userNames  map[string]string
	itemsByRec map[string][]model.CheckinRecordItem
	filesByID  map[string]sysmodel.UploadFile
	flows      map[string]types.FlowStepArray
	// 槽位名单缓存（project|slot → 用户名单），避免逐记录重复查询
	slotUsers map[string]types.IDArray
	// 当前请求身份（can_audit 判定用）
	identity *middleware.Identity
}

// loadReviewBatch 按本页记录集合批量预载。
func (s *ReviewService) loadReviewBatch(rows []model.CheckinRecord) *reviewBatchCtx {
	ctx := &reviewBatchCtx{
		pointNames: map[string]string{},
		commNames:  map[string]string{},
		userNames:  map[string]string{},
		itemsByRec: map[string][]model.CheckinRecordItem{},
		filesByID:  map[string]sysmodel.UploadFile{},
		flows:     map[string]types.FlowStepArray{},
		slotUsers: map[string]types.IDArray{},
	}
	pointIDs, commIDs, userIDs, recIDs := []string{}, []string{}, []string{}, []string{}
	seenP, seenC, seenU := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i := range rows {
		r := &rows[i]
		recIDs = append(recIDs, r.ID)
		if !seenP[r.PointID] {
			seenP[r.PointID] = true
			pointIDs = append(pointIDs, r.PointID)
		}
		if !seenC[r.CommunityID] {
			seenC[r.CommunityID] = true
			commIDs = append(commIDs, r.CommunityID)
		}
		if !seenU[r.InspectorID] {
			seenU[r.InspectorID] = true
			userIDs = append(userIDs, r.InspectorID)
		}
	}
	if len(pointIDs) > 0 {
		var pts []model.InspectionPoint
		s.db.Select("id", "name").Where("id IN ?", pointIDs).Find(&pts)
		for i := range pts {
			ctx.pointNames[pts[i].ID] = pts[i].Name
		}
	}
	if len(commIDs) > 0 {
		var comms []sysmodel.Community
		s.db.Select("id", "name").Where("id IN ?", commIDs).Find(&comms)
		for i := range comms {
			ctx.commNames[comms[i].ID] = comms[i].Name
			ctx.flows[comms[i].ID] = communitysvc.ResolveFlow(s.db, comms[i].ID, sysmodel.FlowCheckinReview)
		}
	}
	if len(userIDs) > 0 {
		var users []sysmodel.SysUser
		s.db.Select("id", "name").Where("id IN ?", userIDs).Find(&users)
		for i := range users {
			ctx.userNames[users[i].ID] = users[i].Name
		}
	}
	if len(recIDs) > 0 {
		var items []model.CheckinRecordItem
		s.db.Where("record_id IN ?", recIDs).Order("sort ASC").Find(&items)
		refs := make([]string, 0, len(items))
		for i := range items {
			itemsByRecPut(ctx, items[i].RecordID, items[i])
			refs = append(refs, items[i].Photos...)
			refs = append(refs, items[i].ResolutionFileIDs...)
		}
		ctx.filesByID = uploadfile.ByIDs(s.db, refs)
	}
	return ctx
}

func itemsByRecPut(ctx *reviewBatchCtx, recID string, it model.CheckinRecordItem) {
	ctx.itemsByRec[recID] = append(ctx.itemsByRec[recID], it)
}

// reviewItemBatch 与 reviewItem 同结构的行视图（全部走预载 map，无额外查询）。
func (s *ReviewService) reviewItemBatch(r *model.CheckinRecord, ctx *reviewBatchCtx) gin.H {
	// 扁平照片（同 RecordFlatPhotos：逐项 sort 顺序）
	flat := make([]gin.H, 0, 8)
	for _, it := range ctx.itemsByRec[r.ID] {
		for _, ref := range it.Photos {
			f := ctx.filesByID[ref]
			entry := gin.H{"item": it.Name, "file_id": f.ID, "url": f.URL}
			if f.WatermarkedURL != "" {
				entry["watermarked_url"] = f.WatermarkedURL
			}
			if f.ExifTime != nil {
				entry["exif_time"] = timefmt.T(*f.ExifTime)
			}
			flat = append(flat, entry)
		}
	}
	// 逐项视图（同 checkItemViews）
	views := make([]gin.H, 0, len(ctx.itemsByRec[r.ID]))
	for _, ci := range ctx.itemsByRec[r.ID] {
		urls := make([]string, 0, len(ci.Photos))
		for _, ref := range ci.Photos {
			f := ctx.filesByID[ref]
			if f.WatermarkedURL != "" {
				urls = append(urls, f.WatermarkedURL)
			} else {
				urls = append(urls, f.URL)
			}
		}
		// 处置照片（disposition=on_site_resolved 时非空；优先水印图）
		resURLs := make([]string, 0, len(ci.ResolutionFileIDs))
		for _, ref := range ci.ResolutionFileIDs {
			f := ctx.filesByID[ref]
			if f.WatermarkedURL != "" {
				resURLs = append(resURLs, f.WatermarkedURL)
			} else {
				resURLs = append(resURLs, f.URL)
			}
		}
		views = append(views, gin.H{
			"name": ci.Name, "pass": ci.Pass, "note": ci.Note,
			"photos": ci.Photos, "photo_urls": urls,
			"requirement": ci.Requirement, "ai_hint": ci.AIHint,
			"judge_type": ci.JudgeType, "judge_config": ci.JudgeConfig,
			"ai_verdict": ci.AIVerdict, "ai_reason": ci.AIReason, "ai_reading": ci.AIReading,
			"disposition": ci.Disposition, "resolution_note": ci.ResolutionNote,
			"resolution_file_ids": ci.ResolutionFileIDs, "resolution_photo_urls": resURLs,
		})
	}
	// 审批链环节（同 flowStepViews，flow 来自预载）
	flowSteps := make([]gin.H, 0, 2)
	for i, step := range ctx.flows[r.CommunityID] {
		flowSteps = append(flowSteps, gin.H{
			"name": step.Name, "slot": step.Slot,
			"done": i < int(r.AuditStep), "current": i == int(r.AuditStep) && r.AuditStatus == model.AuditPending,
		})
	}
	// 待审核记录透出当前环节名与「当前用户可否审核」（与 Pass 同口径的名单制授权判定）：
	// 列表按小区+状态过滤，不过滤授权，前端据 can_audit 展示「待授权人处理」而非让用户点了再报错
	canAudit := false
	stepName := ""
	if r.AuditStatus == model.AuditPending {
		stepName, canAudit = s.auditActionable(r, ctx)
	}
	return gin.H{
		"id": r.ID, "task_id": r.TaskID, "point_id": r.PointID,
		"point_name":     ctx.pointNames[r.PointID],
		"community_id":   r.CommunityID,
		"community_name": ctx.commNames[r.CommunityID],
		"inspector_id":   r.InspectorID, "inspector_name": ctx.userNames[r.InspectorID],
		"checkin_time": timefmt.T(r.CheckinTime), "checkin_type": r.CheckinType,
		"distance_to_point": distanceOrNil(r), "result": r.Result, "remark": r.Remark,
		"is_suspect": r.IsSuspect, "suspect_reason": r.SuspectReason,
		"photos": flat, "check_items": views,
		"audit_status": r.AuditStatus, "audit_step": r.AuditStep, "flow_steps": flowSteps,
		"audit_by": r.AuditBy,
		"audit_at": timefmt.TP(r.AuditAt), "audit_remark": r.AuditRemark,
		"ai_verdict": r.AIVerdict, "ai_reason": r.AIReason,
		"ai_quality_pass": r.AIQualityPass, "ai_quality_issue": r.AIQualityIssue,
		"force_submit":    r.ForceSubmit,
		"current_step_name": stepName, "can_audit": canAudit,
	}
}

// auditActionable 待审核记录的当前环节名 + 当前用户是否可审核（与 Pass 同口径：环节槽位名单制授权；
// 快照优先于现行流程，环节越界走汇报线兜底）。
func (s *ReviewService) auditActionable(r *model.CheckinRecord, ctx *reviewBatchCtx) (string, bool) {
	flow := ctx.flows[r.CommunityID]
	if len(r.FlowSnapshot) > 0 {
		flow = communitysvc.FlowOrResolve(s.db, r.FlowSnapshot, r.CommunityID, sysmodel.FlowCheckinReview)
	}
	stepIdx := int(r.AuditStep)
	stepName := "主管审核"
	slot := sysmodel.SlotPatrolReportLine
	if stepIdx < len(flow) {
		step := flow[stepIdx]
		stepName = step.Name
		if step.Kind != sysmodel.FlowStepKindAI {
			slot = step.Slot
		}
	}
	return stepName, s.slotAuthorizedCached(ctx, r.CommunityID, slot)
}

// slotAuthorizedCached 名单制授权判定（同 communitysvc.SlotAuthorized，名单按 project|slot 走批次缓存）。
func (s *ReviewService) slotAuthorizedCached(ctx *reviewBatchCtx, projectID, slot string) bool {
	idt := ctx.identity
	if idt == nil {
		return false
	}
	if idt.SuperAdmin {
		return true
	}
	for _, code := range idt.RoleCodes {
		if code == sysmodel.TenantAdminCode {
			return true
		}
	}
	key := projectID + "|" + slot
	users, ok := ctx.slotUsers[key]
	if !ok {
		users = communitysvc.SlotUserIDs(s.db, projectID, slot)
		ctx.slotUsers[key] = users
	}
	for _, uid := range users {
		if uid == idt.UserID {
			return true
		}
	}
	return false
}
