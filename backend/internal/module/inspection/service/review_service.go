package service

import (
	"fmt"
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"anxuncloud/internal/config"
	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// spotcheckLimit 单次抽查抽取上限（防呆）。
const spotcheckLimit = 500

// spotcheckAILimit AI 通道单次抽取上限（逐条同步调用大模型，防失控）。
const spotcheckAILimit = 50

// ReviewService 打卡记录审核与抽查服务。
type ReviewService struct {
	db    *gorm.DB
	aiCli *ai.Client
	store *storage.Storage // 可空；逐项照片 file_key 转 URL 用
}

func NewReviewService(db *gorm.DB) *ReviewService {
	// router 装配签名固定为 NewReviewService(db)：配置改由 sys_config 直读
	// （审核/抽查为低频操作，无需走 config:all 缓存），存储抽象按环境配置自装配
	getCfg := func(key string) (string, bool) {
		var cfg sysmodel.SysConfig
		if err := db.Select("value").Where("key = ?", key).First(&cfg).Error; err != nil {
			return "", false
		}
		return cfg.Value, true
	}
	var opts []ai.Option
	var store *storage.Storage
	if cfg, err := config.Load(); err == nil {
		store = storage.New(cfg.Upload, cfg.OSS, cfg.COS, cfg.App.BaseURL)
		opts = append(opts, ai.WithStorage(store))
	}
	// 租户级挂点预留（P3 设计方案 §9.2）：大模型配置的租户级覆盖后续改为
	// ConfigService.Resolve(tenantID, ...) 取值，本期统一用平台默认，行为不变。
	return &ReviewService{db: db, aiCli: ai.NewClient(getCfg, opts...), store: store}
}

// List 审核记录分页列表（数据权限按小区过滤）。
func (s *ReviewService) List(c *gin.Context, q *dto.ReviewListQuery) (*response.Page, *errs.Error) {
	db, be := s.scopeQuery(c, q.AuditStatus, q.CommunityID, q.InspectorID, q.StartTime, q.EndTime)
	if be != nil {
		return nil, be
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.CheckinRecord
	offset, limit := q.Normalize()
	if err := db.Order("checkin_time DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, s.reviewItem(&rows[i]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// scopeQuery 审核范围基础查询（含数据权限）。
func (s *ReviewService) scopeQuery(c *gin.Context, auditStatus, communityID, inspectorID, startTime, endTime string) (*gorm.DB, *errs.Error) {
	db := s.db.Model(&model.CheckinRecord{})
	if auditStatus != "" {
		db = db.Where("audit_status = ?", auditStatus)
	}
	if communityID != "" {
		db = db.Where("community_id = ?", communityID)
	}
	if inspectorID != "" {
		db = db.Where("inspector_id = ?", inspectorID)
	}
	var be *errs.Error
	if db, be = timeRangeOn(db, "checkin_time", startTime, endTime); be != nil {
		return nil, be
	}
	return middleware.ApplyCommunityFilter(db, c, "checkin_record.community_id"), nil
}

func (s *ReviewService) reviewItem(r *model.CheckinRecord) gin.H {
	return gin.H{
		"id": r.ID, "task_id": r.TaskID, "point_id": r.PointID,
		"point_name":     pointName(s.db, r.PointID),
		"community_id":   r.CommunityID,
		"community_name": commName(s.db, r.CommunityID),
		"inspector_id": r.InspectorID, "inspector_name": userName(s.db, r.InspectorID),
		"checkin_time": timefmt.T(r.CheckinTime), "checkin_type": r.CheckinType,
		"distance_to_point": distanceOrNil(r), "result": r.Result, "remark": r.Remark,
		"is_suspect": r.IsSuspect, "suspect_reason": r.SuspectReason,
		"photos": r.Photos, "check_items": s.checkItemViews(r.ID),
		"audit_status": r.AuditStatus, "audit_step": r.AuditStep, "flow_steps": s.flowStepViews(r),
		"audit_by": r.AuditBy,
		"audit_at": timefmt.TP(r.AuditAt), "audit_remark": r.AuditRemark,
		"ai_verdict": r.AIVerdict, "ai_reason": r.AIReason,
	}
}

// flowStepViews 审批链环节视图（供前端展示进度：环节名 + 是否已通过 + 是否当前环节）。
func (s *ReviewService) flowStepViews(r *model.CheckinRecord) []gin.H {
	flow := communitysvc.ResolveFlow(s.db, r.CommunityID, sysmodel.FlowCheckinReview)
	out := make([]gin.H, 0, len(flow))
	for i, step := range flow {
		out = append(out, gin.H{
			"name": step.Name, "slot": step.Slot,
			"done": i < int(r.AuditStep), "current": i == int(r.AuditStep) && r.AuditStatus == model.AuditPending,
		})
	}
	return out
}

// Pass 审核通过当前环节（审批链按链执行，扩展方案 §3）：
// 操作者须在当前环节槽位名单内；非末环节通过后推进到下一环节并通知下一环节名单，末环节通过才置 pass。
func (s *ReviewService) Pass(c *gin.Context, id string) *errs.Error {
	r, be := s.loadPending(c, id)
	if be != nil {
		return be
	}
	flow := communitysvc.ResolveFlow(s.db, r.CommunityID, sysmodel.FlowCheckinReview)
	stepIdx := int(r.AuditStep)
	if stepIdx >= len(flow) {
		return errs.ErrConflict.WithMsg("该记录已完成全部审核环节")
	}
	step := flow[stepIdx]
	patrolType := s.patrolTypeOf(r.TaskID)
	slot := communitysvc.FlowStepSlot(s.db, r.CommunityID, patrolType, step.Slot)
	if !communitysvc.SlotAuthorized(s.db, r.CommunityID, slot, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不在「" + step.Name + "」环节授权名单内")
	}
	now := time.Now()
	by := middleware.CurrentUserID(c)
	if stepIdx+1 >= len(flow) { // 末环节 → 审核通过
		updates := map[string]any{
			"audit_status": model.AuditPass, "audit_step": stepIdx + 1,
			"audit_by": by, "audit_at": now, "audit_remark": "",
		}
		if err := s.db.Model(&model.CheckinRecord{}).Where("id = ?", r.ID).Updates(updates).Error; err != nil {
			return errs.ErrInternal
		}
		return nil
	}
	// 非末环节 → 进度 +1，保持 pending，定向通知下一环节名单
	updates := map[string]any{"audit_step": stepIdx + 1, "audit_by": by, "audit_at": now}
	if err := s.db.Model(&model.CheckinRecord{}).Where("id = ?", r.ID).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	next := flow[stepIdx+1]
	nextSlot := communitysvc.FlowStepSlot(s.db, r.CommunityID, patrolType, next.Slot)
	ptName := pointName(s.db, r.PointID)
	for _, uid := range communitysvc.SlotUserIDs(s.db, r.CommunityID, nextSlot) {
		s.db.Create(&sysmodel.SysMessage{
			UserID: uid, Type: "checkin_audit",
			Title:   "打卡记录待" + next.Name,
			Content: fmt.Sprintf("点位「%s」的打卡记录已通过「%s」，待您执行「%s」。", ptName, step.Name, next.Name),
			BizID:   &r.ID,
		})
	}
	return nil
}

// requireReportLine 巡查汇报线名单校验（抽查/催办归口汇报线成员；超管/租户管理员默认放行）。
// 按打卡所属任务的巡查类型路由到对应业务线汇报线槽位（维度槽位 → 通用槽位回落，见扩展方案 §2）。
func (s *ReviewService) requireReportLine(c *gin.Context, r *model.CheckinRecord) *errs.Error {
	slot := communitysvc.ResolveReportLineSlot(s.db, r.CommunityID, s.patrolTypeOf(r.TaskID))
	if !communitysvc.SlotAuthorized(s.db, r.CommunityID, slot, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不在本项目该巡查业务线的审核名单内")
	}
	return nil
}

// patrolTypeOf 任务巡查类型（记录必属任务；取不到按空串 → 通用汇报线槽位）。
func (s *ReviewService) patrolTypeOf(taskID string) string {
	var t model.InspectionTask
	if err := s.db.Select("patrol_type").First(&t, "id = ?", taskID).Error; err != nil {
		return ""
	}
	return t.PatrolType
}

// requireReportLineForRecords 抽查场景：记录涉及的「小区 × 巡查类型」组合均须通过汇报线校验。
func (s *ReviewService) requireReportLineForRecords(c *gin.Context, ids []string) *errs.Error {
	type pair struct {
		CommunityID string
		PatrolType  string
	}
	var pairs []pair
	s.db.Model(&model.CheckinRecord{}).
		Select("checkin_record.community_id, inspection_task.patrol_type").
		Joins("JOIN inspection_task ON inspection_task.id = checkin_record.task_id").
		Where("checkin_record.id IN ?", ids).
		Distinct().Scan(&pairs)
	for _, p := range pairs {
		slot := communitysvc.ResolveReportLineSlot(s.db, p.CommunityID, p.PatrolType)
		if !communitysvc.SlotAuthorized(s.db, p.CommunityID, slot, middleware.CurrentIdentity(c)) {
			return errs.ErrNotInSlot.WithMsg("当前用户不在本项目该巡查业务线的审核名单内")
		}
	}
	return nil
}

// BatchPass 批量审核通过（按链各推进一步：处于末环节的记录置 pass，其余进度 +1）。
// 小区数据权限通过 ApplyCommunityFilter 收敛；返回 passed/advanced/skipped 计数。
func (s *ReviewService) BatchPass(c *gin.Context, ids []string) (gin.H, *errs.Error) {
	var recs []model.CheckinRecord
	db := s.db.Where("id IN ? AND audit_status = ?", ids, model.AuditPending)
	db = middleware.ApplyCommunityFilter(db, c, "checkin_record.community_id")
	if err := db.Find(&recs).Error; err != nil {
		return nil, errs.ErrInternal
	}
	// 先全量校验授权（任一记录当前环节未授权则整批拒绝，避免半批推进）
	flows := map[string]types.FlowStepArray{} // community_id → 审批链
	for i := range recs {
		flow, ok := flows[recs[i].CommunityID]
		if !ok {
			flow = communitysvc.ResolveFlow(s.db, recs[i].CommunityID, sysmodel.FlowCheckinReview)
			flows[recs[i].CommunityID] = flow
		}
		stepIdx := int(recs[i].AuditStep)
		if stepIdx >= len(flow) {
			continue
		}
		slot := communitysvc.FlowStepSlot(s.db, recs[i].CommunityID, s.patrolTypeOf(recs[i].TaskID), flow[stepIdx].Slot)
		if !communitysvc.SlotAuthorized(s.db, recs[i].CommunityID, slot, middleware.CurrentIdentity(c)) {
			return nil, errs.ErrNotInSlot.WithMsg("当前用户不在「" + flow[stepIdx].Name + "」环节授权名单内")
		}
	}
	now := time.Now()
	by := middleware.CurrentUserID(c)
	passed, advanced := 0, 0
	for i := range recs {
		flow := flows[recs[i].CommunityID]
		stepIdx := int(recs[i].AuditStep)
		if stepIdx >= len(flow) {
			continue
		}
		updates := map[string]any{"audit_step": stepIdx + 1, "audit_by": by, "audit_at": now}
		if stepIdx+1 >= len(flow) {
			updates["audit_status"] = model.AuditPass
			updates["audit_remark"] = ""
			passed++
		} else {
			advanced++
		}
		// 并发下不覆盖人工已处理的记录
		res := s.db.Model(&model.CheckinRecord{}).
			Where("id = ? AND audit_status = ? AND audit_step = ?", recs[i].ID, model.AuditPending, stepIdx).
			Updates(updates)
		if res.Error != nil {
			return nil, errs.ErrInternal
		}
	}
	return gin.H{"passed": passed, "advanced": advanced, "skipped": len(ids) - passed - advanced}, nil
}

// Reopen 撤销审核（pass/rejected → pending 且审批进度回 0，重新走完整链）：审核误操作的后悔药。
// 操作者须在首环节（汇报线）名单内；清空原审核人与意见（操作日志已留痕）。
func (s *ReviewService) Reopen(c *gin.Context, id string) *errs.Error {
	var r model.CheckinRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, r.CommunityID); be != nil {
		return be
	}
	if be := s.requireReportLine(c, &r); be != nil {
		return be
	}
	res := s.db.Model(&model.CheckinRecord{}).
		Where("id = ? AND audit_status IN ?", r.ID, []string{model.AuditPass, model.AuditRejected}).
		Updates(map[string]any{
			"audit_status": model.AuditPending,
			"audit_step":   0,
			"audit_by":     nil,
			"audit_at":     nil,
			"audit_remark": "",
		})
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected == 0 {
		return errs.ErrConflict.WithMsg("仅已审核（pass/rejected）记录可撤销审核")
	}
	return nil
}

// Reject 审核打回（仅 pending 可审；操作者须在当前环节槽位名单内），并站内通知巡检员。
func (s *ReviewService) Reject(c *gin.Context, id, reason string) *errs.Error {
	r, be := s.loadPending(c, id)
	if be != nil {
		return be
	}
	flow := communitysvc.ResolveFlow(s.db, r.CommunityID, sysmodel.FlowCheckinReview)
	stepIdx := int(r.AuditStep)
	if stepIdx >= len(flow) {
		return errs.ErrConflict.WithMsg("该记录已完成全部审核环节")
	}
	step := flow[stepIdx]
	slot := communitysvc.FlowStepSlot(s.db, r.CommunityID, s.patrolTypeOf(r.TaskID), step.Slot)
	if !communitysvc.SlotAuthorized(s.db, r.CommunityID, slot, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不在「" + step.Name + "」环节授权名单内")
	}
	now := time.Now()
	by := middleware.CurrentUserID(c)
	updates := map[string]any{
		"audit_status": model.AuditRejected, "audit_by": by, "audit_at": now, "audit_remark": reason,
	}
	if err := s.db.Model(&model.CheckinRecord{}).Where("id = ?", r.ID).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	// 站内通知巡检员（微信订阅推送预留）
	ptName := pointName(s.db, r.PointID)
	msg := sysmodel.SysMessage{
		UserID:  r.InspectorID,
		Type:    "checkin_audit",
		Title:   "打卡记录被打回",
		Content: fmt.Sprintf("你在点位「%s」的打卡记录审核未通过，原因：%s。请核实后按要求补巡。", ptName, reason),
		BizID:   &r.ID,
	}
	s.db.Create(&msg)
	return nil
}

// loadPending 加载待审核记录并做小区数据权限校验。
func (s *ReviewService) loadPending(c *gin.Context, id string) (*model.CheckinRecord, *errs.Error) {
	var r model.CheckinRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, r.CommunityID); be != nil {
		return nil, be
	}
	if r.AuditStatus != model.AuditPending {
		return nil, errs.ErrConflict.WithMsg("仅待审核（pending）记录可执行审核操作")
	}
	return &r, nil
}

// Spotcheck 抽查：manual 通道从 auto_pass/pass 记录中抽取并置为 pending；ai 通道逐条大模型审核。
func (s *ReviewService) Spotcheck(c *gin.Context, req *dto.SpotcheckReq) (gin.H, *errs.Error) {
	if req.Mode == "random" && req.Ratio == 0 {
		return nil, errs.ErrParam.WithMsg("mode=random 时 ratio 必填（1-100）")
	}
	if req.Handler == "ai" {
		return s.spotcheckAI(c, req)
	}
	ids, be := s.pickSpotcheckIDs(c, req, spotcheckLimit)
	if be != nil {
		return nil, be
	}
	if len(ids) == 0 {
		return gin.H{"picked": 0}, nil
	}
	if be := s.requireReportLineForRecords(c, ids); be != nil {
		return nil, be
	}
	res := s.db.Model(&model.CheckinRecord{}).
		Where("id IN ? AND audit_status IN ?", ids, []string{model.AuditAutoPass, model.AuditPass}).
		Update("audit_status", model.AuditPending)
	if res.Error != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"picked": int(res.RowsAffected)}, nil
}

// pickSpotcheckIDs 按范围与方式抽取记录 ID（random 按比例、full 取最近，上限 limit）。
func (s *ReviewService) pickSpotcheckIDs(c *gin.Context, req *dto.SpotcheckReq, limit int) ([]string, *errs.Error) {
	base, be := s.scopeQuery(c, "", req.CommunityID, req.InspectorID, req.StartTime, req.EndTime)
	if be != nil {
		return nil, be
	}
	base = base.Where("audit_status IN ?", []string{model.AuditAutoPass, model.AuditPass})
	var ids []string
	q := base.Session(&gorm.Session{})
	if req.Mode == "random" {
		q = q.Order("random()")
	} else {
		q = q.Order("checkin_time DESC")
	}
	if err := q.Limit(limit).Pluck("id", &ids).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if req.Mode == "random" && len(ids) > 0 {
		n := int(math.Ceil(float64(len(ids)) * float64(req.Ratio) / 100))
		if n < 1 {
			n = 1
		}
		if n < len(ids) {
			ids = ids[:n]
		}
	}
	return ids, nil
}

// spotcheckAI AI 抽查通道：范围内抽取记录（上限 50 条）逐条同步调用大模型，
// pass 标 ai_verdict=pass（audit_status 不变）；review 标 review + 转 pending；失败标 error。
func (s *ReviewService) spotcheckAI(c *gin.Context, req *dto.SpotcheckReq) (gin.H, *errs.Error) {
	if !s.aiCli.Enabled() {
		return nil, errs.ErrParam.WithMsg("大模型审核未启用，请先在系统配置 ai 分组开启")
	}
	ids, be := s.pickSpotcheckIDs(c, req, spotcheckAILimit)
	if be != nil {
		return nil, be
	}
	if be := s.requireReportLineForRecords(c, ids); be != nil {
		return nil, be
	}
	picked, toReview, passed, failed := 0, 0, 0, 0
	for i, id := range ids {
		var r model.CheckinRecord
		if err := s.db.First(&r, "id = ?", id).Error; err != nil {
			continue
		}
		picked++
		res, err := s.aiCli.ReviewCheckin(c.Request.Context(), s.reviewInputOf(&r))
		updates := map[string]any{}
		switch {
		case err != nil:
			updates["ai_verdict"] = model.AIVerdictError
			updates["ai_reason"] = truncateRunes(err.Error(), 200)
			failed++
		default:
			updates["ai_reason"] = truncateRunes(res.Reason, 500)
			writeItemVerdicts(s.db, r.ID, res.Items)
			if res.Verdict == model.AIVerdictPass {
				updates["ai_verdict"] = model.AIVerdictPass
				passed++
			} else {
				updates["ai_verdict"] = model.AIVerdictReview
				updates["audit_status"] = model.AuditPending
				toReview++
			}
		}
		// 并发下不覆盖人工已审核的记录
		if uerr := s.db.Model(&model.CheckinRecord{}).
			Where("id = ? AND audit_status IN ?", r.ID, []string{model.AuditAutoPass, model.AuditPass}).
			Updates(updates).Error; uerr != nil {
			logger.L.Warn("AI 抽查回写失败", zap.String("rec_id", r.ID), zap.Error(uerr))
		}
		logger.L.Info("AI 抽查进度",
			zap.Int("done", i+1), zap.Int("total", len(ids)),
			zap.String("rec_id", r.ID), zap.String("verdict", updates["ai_verdict"].(string)))
	}
	return gin.H{"picked": picked, "to_review": toReview, "passed": passed, "failed": failed}, nil
}

// checkItemViews 逐项结果视图（v18 起读 checkin_record_item 快照表）：附带该项照片可访问 URL（photos 存 file_key）。
func (s *ReviewService) checkItemViews(recordID string) []gin.H {
	var items []model.CheckinRecordItem
	s.db.Where("record_id = ?", recordID).Order("sort ASC").Find(&items)
	out := make([]gin.H, 0, len(items))
	for _, ci := range items {
		urls := make([]string, 0, len(ci.Photos))
		if s.store != nil {
			for _, key := range ci.Photos {
				urls = append(urls, s.store.URL(key))
			}
		}
		out = append(out, gin.H{
			"name": ci.Name, "pass": ci.Pass, "note": ci.Note,
			"photos": ci.Photos, "photo_urls": urls,
			"requirement": ci.Requirement, "ai_hint": ci.AIHint,
			"ai_verdict": ci.AIVerdict, "ai_reason": ci.AIReason,
		})
	}
	return out
}

// reviewInputOf 由打卡记录组装大模型审核上下文。
func (s *ReviewService) reviewInputOf(r *model.CheckinRecord) ai.ReviewInput {
	var point model.InspectionPoint
	s.db.Select("name", "type").First(&point, "id = ?", r.PointID)
	var recItems []model.CheckinRecordItem
	s.db.Where("record_id = ?", r.ID).Order("sort ASC").Find(&recItems)
	names := make([]string, 0, len(recItems))
	for _, it := range recItems {
		names = append(names, it.Name)
	}
	refs := make([]ai.PhotoRef, 0, len(r.Photos))
	for _, p := range r.Photos {
		refs = append(refs, ai.PhotoRef{URL: p.URL})
	}
	var itemPhotos []ai.ItemPhoto
	if s.store != nil {
		for _, it := range recItems {
			if len(it.Photos) == 0 {
				continue
			}
			irefs := make([]ai.PhotoRef, 0, len(it.Photos))
			for _, key := range it.Photos {
				irefs = append(irefs, ai.PhotoRef{URL: s.store.URL(key)})
			}
			itemPhotos = append(itemPhotos, ai.ItemPhoto{
				Name: it.Name, Requirement: strVal(it.Requirement), AIHint: strVal(it.AIHint), Photos: irefs,
			})
		}
	}
	return ai.ReviewInput{
		PointName: point.Name, PointType: point.Type, CheckItems: names, Remark: r.Remark,
		Photos: refs, ItemPhotos: itemPhotos,
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// strVal 可空文本快照取值（nil → 空串）。
func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// writeItemVerdicts 逐项 AI 结论落库（按 record_id+name 匹配快照行；模型未返回逐项结论时为空不做事）。
func writeItemVerdicts(db *gorm.DB, recID string, items []ai.ItemVerdict) {
	for _, iv := range items {
		v, r := iv.Verdict, truncateRunes(iv.Reason, 500)
		if err := db.Model(&model.CheckinRecordItem{}).
			Where("record_id = ? AND name = ?", recID, iv.Name).
			Updates(map[string]any{"ai_verdict": v, "ai_reason": r}).Error; err != nil {
			logger.L.Warn("逐项 AI 结论回写失败", zap.String("rec_id", recID), zap.String("item", iv.Name), zap.Error(err))
		}
	}
}
