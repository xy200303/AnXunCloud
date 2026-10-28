package service

import (
	"fmt"
	"math"
	"strings"
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
	workordersvc "anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/configread"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

// spotcheckLimit 单次抽查抽取上限（防呆）。
const spotcheckLimit = 500

// spotcheckAILimit AI 通道单次抽取上限（逐条同步调用大模型，防失控）。
const spotcheckAILimit = 50

// ReviewService 打卡记录审核与抽查服务。
type ReviewService struct {
	db    *gorm.DB
	aiCli *ai.Client
	// checkinAIGate 打卡 AI 闸门回调（mp 模块装配注入；人工环节推进到 AI 环节时触发），为 nil 跳过（记录停 AI 环节等人工兜底）。
	checkinAIGate func(recID string)
	orderSink     interface {
		CreateFromInspection(tx *gorm.DB, req workordersvc.CreateFromInspectionReq) error
	}
	store    *storage.Storage // 可空；逐项照片 file_id 转 URL 用
	notifier *notify.Notifier
}

// BindCheckinAIGate 注入打卡 AI 闸门回调（router 装配：mp.CheckinService.RunAIGate）。
func (s *ReviewService) BindCheckinAIGate(gate func(recID string)) {
	s.checkinAIGate = gate
}

func (s *ReviewService) BindWorkOrderSink(sink interface {
	CreateFromInspection(tx *gorm.DB, req workordersvc.CreateFromInspectionReq) error
}) {
	s.orderSink = sink
}

func NewReviewService(db *gorm.DB, notifier *notify.Notifier) *ReviewService {
	// 配置改由 sys_config 直读（审核/抽查为低频操作，无需走 config:all 缓存），存储抽象按环境配置自装配
	getCfg := configread.Getter(db)
	var opts []ai.Option
	var store *storage.Storage
	if cfg, err := config.Load(); err == nil {
		store = storage.New(cfg.Upload, cfg.OSS, cfg.COS, cfg.App.BaseURL)
		opts = append(opts, ai.WithStorage(store))
	}
	// 租户级挂点预留（P3 设计方案 §9.2）：大模型配置的租户级覆盖后续改为
	// ConfigService.Resolve(tenantID, ...) 取值，本期统一用平台默认，行为不变。
	return &ReviewService{db: db, aiCli: ai.NewClient(getCfg, opts...), store: store, notifier: notifier}
}

// List 审核记录分页列表（数据权限按小区过滤）。
func (s *ReviewService) List(c *gin.Context, q *dto.ReviewListQuery) (*response.Page, *errs.Error) {
	db, be := s.scopeQuery(c, q.AuditStatus, q.CommunityID, q.InspectorID, q.StartTime, q.EndTime)
	if be != nil {
		return nil, be
	}
	if q.ID != "" {
		db = db.Where("id = ?", q.ID)
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
	batch := s.loadReviewBatch(rows)
	batch.identity = middleware.CurrentIdentity(c)
	for i := range rows {
		list = append(list, s.reviewItemBatch(&rows[i], batch))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// scopeQuery 审核范围基础查询（含数据权限；固定过滤 superseded_by 非空的已覆盖旧记录）。
func (s *ReviewService) scopeQuery(c *gin.Context, auditStatus, communityID, inspectorID, startTime, endTime string) (*gorm.DB, *errs.Error) {
	db := s.db.Model(&model.CheckinRecord{}).Where("superseded_by IS NULL")
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
	db = middleware.ApplyCommunityFilter(db, c, "checkin_record.community_id")
	if identity := middleware.CurrentIdentity(c); identity != nil && identity.ScopeSelf {
		db = db.Where("checkin_record.inspector_id = ? OR checkin_record.id IN (?)", identity.UserID, communitysvc.CandidateCheckinRecordIDs(s.db, identity))
	}
	return db, nil
}

func (s *ReviewService) Pass(c *gin.Context, id string) *errs.Error {
	r, be := s.loadPending(c, id)
	if be != nil {
		return be
	}
	flow := communitysvc.FlowOrResolve(s.db, r.FlowSnapshot, r.CommunityID, sysmodel.FlowCheckinReview)
	stepIdx := int(r.AuditStep)
	if stepIdx >= len(flow) {
		return errs.ErrConflict.WithMsg("审批环节无效，请修复审批链")
	}
	step := flow[stepIdx]
	if step.Kind == sysmodel.FlowStepKindAI || !communitysvc.RoleCandidateAuthorized(s.db, []string{r.CommunityID}, step.RoleCode, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不是该审批节点的候选人")
	}
	by := middleware.CurrentUserID(c)
	if communitysvc.SameNaturalPerson(s.db, by, r.InspectorID) {
		return errs.ErrNoPerm.WithMsg("不能审核本人提交的记录")
	}
	walk := communitysvc.WalkFlowWithVoters(s.db, r.CommunityID, flow, stepIdx+1, "", false)
	if walk.Blocked {
		communitysvc.NotifyBlockedFlowSteps(s.db, s.notifier, r.TenantID, "打卡审核", walk.BlockedSteps, &r.ID)
		return errs.ErrConflict.WithMsg("后续审批节点缺少候选人，已挂起")
	}
	now := time.Now()
	if walk.Finish {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&model.CheckinRecord{}).Where("id = ? AND audit_status = ? AND audit_step = ?", r.ID, model.AuditPending, stepIdx).
				Updates(map[string]any{"audit_status": model.AuditPass, "audit_step": walk.Step,
					"audit_by": by, "audit_at": now, "audit_remark": ""})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errs.ErrConflict.WithMsg("审核状态已被其他人更新")
			}
			return s.createApprovedWorkOrders(tx, r, by)
		})
		if err != nil {
			if be, ok := err.(*errs.Error); ok {
				return be
			}
			return errs.ErrInternal
		}
		return nil
	}
	updates := map[string]any{"audit_step": walk.Step, "audit_by": by, "audit_at": now}
	res := s.db.Model(&model.CheckinRecord{}).Where("id = ? AND audit_status = ? AND audit_step = ?", r.ID, model.AuditPending, stepIdx).Updates(updates)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected != 1 {
		return errs.ErrConflict.WithMsg("审核状态已被其他人更新")
	}
	if walk.NeedGate || (walk.Step < len(flow) && flow[walk.Step].Kind == sysmodel.FlowStepKindAI) {
		if s.checkinAIGate != nil {
			go s.checkinAIGate(r.ID)
		}
		return nil
	}
	next := flow[walk.Step]
	_ = s.notifier.SendBatch(communitysvc.RoleCandidateUserIDsForCommunity(s.db, r.CommunityID, next.RoleCode), r.TenantID, "checkin_audit",
		"打卡记录待"+next.Name,
		fmt.Sprintf("点位「%s」的打卡记录已通过「%s」，待您执行「%s」。", pointName(s.db, r.PointID), step.Name, next.Name), &r.ID)
	return nil
}

func (s *ReviewService) requireReportLine(c *gin.Context, r *model.CheckinRecord) *errs.Error {
	if !communitysvc.RoleCandidateAuthorized(s.db, []string{r.CommunityID}, sysmodel.QualitySupervisorCode, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不是打卡复核节点的候选人")
	}
	return nil
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
		if !communitysvc.RoleCandidateAuthorized(s.db, []string{p.CommunityID}, sysmodel.QualitySupervisorCode, middleware.CurrentIdentity(c)) {
			return errs.ErrNotInSlot.WithMsg("当前用户不是打卡复核节点的候选人")
		}
	}
	return nil
}

func (s *ReviewService) BatchPass(c *gin.Context, ids []string) (gin.H, *errs.Error) {
	passed, advanced := 0, 0
	skippedDetail := make([]gin.H, 0)
	for _, id := range ids {
		before, be := s.loadPending(c, id)
		if be != nil {
			skippedDetail = append(skippedDetail, gin.H{"id": id, "reason": be.Msg})
			continue
		}
		previousStep := before.AuditStep
		if be := s.Pass(c, id); be != nil {
			skippedDetail = append(skippedDetail, gin.H{"id": id, "reason": be.Msg})
			continue
		}
		var after model.CheckinRecord
		if err := s.db.Select("audit_status", "audit_step").First(&after, "id = ?", id).Error; err != nil {
			return nil, errs.ErrInternal
		}
		if after.AuditStatus == model.AuditPass {
			passed++
		} else if after.AuditStep != previousStep {
			advanced++
		} else {
			skippedDetail = append(skippedDetail, gin.H{"id": id, "reason": "审批已挂起"})
		}
	}
	return gin.H{"passed": passed, "advanced": advanced, "skipped": len(skippedDetail), "skipped_detail": skippedDetail}, nil
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
	flow := communitysvc.FlowOrResolve(s.db, r.FlowSnapshot, r.CommunityID, sysmodel.FlowCheckinReview)
	stepIdx := int(r.AuditStep)
	if stepIdx >= len(flow) {
		return errs.ErrConflict.WithMsg("审批环节无效，请修复审批链")
	}
	step := flow[stepIdx]
	by := middleware.CurrentUserID(c)
	if step.Kind == sysmodel.FlowStepKindAI || !communitysvc.RoleCandidateAuthorized(s.db, []string{r.CommunityID}, step.RoleCode, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不是该审批节点的候选人")
	}
	if communitysvc.SameNaturalPerson(s.db, by, r.InspectorID) {
		return errs.ErrNoPerm.WithMsg("不能审核本人提交的记录")
	}
	now := time.Now()
	updates := map[string]any{
		"audit_status": model.AuditRejected, "audit_by": by, "audit_at": now, "audit_remark": reason,
	}
	res := s.db.Model(&model.CheckinRecord{}).Where("id = ? AND audit_status = ? AND audit_step = ?", r.ID, model.AuditPending, stepIdx).Updates(updates)
	if res.Error != nil {
		return errs.ErrInternal
	}
	if res.RowsAffected != 1 {
		return errs.ErrConflict.WithMsg("审核状态已被其他人更新")
	}
	// 站内通知巡检员 + App 推送
	ptName := pointName(s.db, r.PointID)
	_ = s.notifier.Send(r.InspectorID, "checkin_audit",
		"打卡记录被驳回",
		fmt.Sprintf("你在点位「%s」的打卡记录审核未通过，原因：%s。请核实后按要求补巡。", ptName, reason),
		&r.ID)
	return nil
}

// loadPending 加载待审核记录并做小区数据权限校验。
func (s *ReviewService) loadPending(c *gin.Context, id string) (*model.CheckinRecord, *errs.Error) {
	var r model.CheckinRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, r.CommunityID); be != nil {
		flow := communitysvc.FlowOrResolve(s.db, r.FlowSnapshot, r.CommunityID, sysmodel.FlowCheckinReview)
		if int(r.AuditStep) >= len(flow) || !communitysvc.RoleCandidateAuthorized(s.db, []string{r.CommunityID}, flow[r.AuditStep].RoleCode, middleware.CurrentIdentity(c)) {
			return nil, be
		}
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
			updates["ai_reason"] = strutil.Truncate(err.Error(), 200)
			failed++
		default:
			updates["ai_reason"] = strutil.Truncate(res.Reason, 500)
			updates["ai_quality_pass"] = res.Quality.Pass
			updates["ai_quality_issue"] = strutil.Truncate(res.Quality.Issue, 255)
			WriteItemVerdicts(s.db, r.ID, res.Items)
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

// reviewInputOf 由记录身份读取当前/已退役模板定义，记录本身只提供事实和照片。
func (s *ReviewService) reviewInputOf(r *model.CheckinRecord) ai.ReviewInput {
	var point model.InspectionPoint
	s.db.Select("name", "type").First(&point, "id = ?", r.PointID)
	var recItems []model.CheckinRecordItem
	s.db.Where("record_id = ?", r.ID).Order("sort ASC").Find(&recItems)
	LoadItemDefinitions(s.db, recItems)
	names := make([]string, 0, len(recItems))
	var itemPhotos []ai.ItemPhoto
	for _, it := range recItems {
		if it.Definition == nil || !it.Definition.AIEnabled {
			continue
		}
		names = append(names, it.Name)
		irefs := make([]ai.PhotoRef, 0, len(it.Photos))
		for _, ref := range it.Photos {
			if f, err := uploadfile.ByID(s.db, ref); err == nil {
				irefs = append(irefs, ai.PhotoRef{URL: f.URL})
			}
		}
		itemPhotos = append(itemPhotos, ItemPhoto(it, irefs))
	}
	return ai.ReviewInput{PointName: point.Name, PointType: point.Type, CheckItems: names, Remark: r.Remark, ItemPhotos: itemPhotos}
}

func WriteItemVerdicts(db *gorm.DB, recID string, items []ai.ItemVerdict) {
	var rows []model.CheckinRecordItem
	db.Where("record_id = ?", recID).Find(&rows)
	LoadItemDefinitions(db, rows)
	byKey := make(map[string]model.CheckinRecordItem, len(rows))
	for _, row := range rows {
		byKey[row.InstanceKey()] = row
	}
	for _, iv := range items {
		row, ok := byKey[iv.InstanceKey]
		if !ok || row.Definition == nil {
			logger.L.Warn("AI 返回未知检查项实例，已忽略", zap.String("rec_id", recID), zap.String("instance_key", iv.InstanceKey))
			continue
		}
		v, reason := iv.Verdict, strutil.Truncate(iv.Reason, 500)
		updates := map[string]any{"ai_verdict": v, "ai_reason": reason}
		if rd := strutil.Truncate(strings.TrimSpace(iv.Reading), 64); rd != "" {
			updates["ai_reading"] = rd
		}
		if len(iv.AbnormalTags) > 0 {
			allow := map[string]bool{}
			for _, tag := range row.Definition.Tags {
				allow[tag] = true
			}
			abn := make(types.StringArray, 0, len(iv.AbnormalTags))
			for _, tag := range iv.AbnormalTags {
				if allow[tag] {
					abn = append(abn, tag)
				}
			}
			updates["abnormal_tags"] = abn
		}
		query := db.Model(&model.CheckinRecordItem{}).Where("record_id = ? AND item_code = ?", recID, row.ItemCode)
		if row.TemplateID == nil {
			query = query.Where("template_id IS NULL")
		} else {
			query = query.Where("template_id = ?", *row.TemplateID)
		}
		if row.EquipmentID == nil {
			query = query.Where("equipment_id IS NULL")
		} else {
			query = query.Where("equipment_id = ?", *row.EquipmentID)
		}
		if err := query.Updates(updates).Error; err != nil {
			logger.L.Warn("逐项 AI 结论回写失败", zap.String("rec_id", recID), zap.String("instance_key", iv.InstanceKey), zap.Error(err))
		}
	}
}
