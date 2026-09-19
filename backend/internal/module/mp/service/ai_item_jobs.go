// 逐项 AI 识别队列：巡检员按检查项提交 1 张照片（一项一图硬约束），异步调大模型识别（质量+内容一次调用），
// App 轮询 job 状态取回逐项结论，确认后随打卡提交（ai_confirmed=true）。
// 队列：Redis list ai:item:queue（LPUSH 入队，N 个 worker BLPOP 消费，N=ai.worker_count 默认 8，兼容旧键 ai.worker_concurrency）；
// 大模型限流（429）指数退避重试 2s/4s/8s 最多 3 次，仍失败按现有失败语义转 failed；
// 结果：Redis hash ai:item:job:{id}（status/verdict/reason/reading/quality_pass/quality_issue，TTL 2 小时）。
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	eqsvc "anxuncloud/internal/module/equipment/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	aiItemQueueKey  = "ai:item:queue"      // 逐项识别任务队列（list）
	aiItemWorkKey   = "ai:item:processing" // worker 处理中任务（list）
	aiItemJobPfx    = "ai:item:job:"       // 逐项识别结果 hash 前缀
	aiItemJobTTL    = 2 * time.Hour        // 结果保留时长（过期按"任务已过期"处理）
	aiItemMaxQuery  = 20                   // 单次批量查询上限
	aiItemRetryMax  = 3                    // 限流重试次数上限（指数退避 2s/4s/8s）
	aiItemRetryBase = 2 * time.Second      // 限流重试退避基数
)

// aiItemJobPayload 队列任务载荷（point/item 上下文入队时快照，worker 不再查库）。
// TaskID/PointID/CommunityID 用于 worker 识别完成后回写 checkin_item_draft 过程草稿。
type aiItemJobPayload struct {
	JobID       string         `json:"job_id"`
	UserID      string         `json:"user_id"`
	TaskID      string         `json:"task_id"`
	PointID     string         `json:"point_id"`
	CommunityID string         `json:"community_id"`
	TenantID    *string        `json:"tenant_id,omitempty"`
	PointName   string         `json:"point_name"`
	PointType   string         `json:"point_type"`
	Name        string         `json:"name"`
	Requirement string         `json:"requirement,omitempty"`
	AIHint      string         `json:"ai_hint,omitempty"`
	JudgeType   string         `json:"judge_type,omitempty"`
	JudgeConfig map[string]any `json:"judge_config,omitempty"`
	Tags        []string       `json:"tags,omitempty"` // 观察点 tag（模型逐点核对，异常点回 abnormal_tags）
	FileIDs     []string       `json:"file_ids"`
}

// SubmitAIItemJob 提交逐项 AI 识别任务（POST /checkin/ai-item-jobs）。
// 判定参数（requirement/judge_type/judge_config/ai_hint）从点位模板项自取，防客户端伪造；
// manual 手动确认项不调 AI，直接拒绝。
func (s *CheckinService) SubmitAIItemJob(ctx context.Context, inspectorID string, req *dto.AIItemJobReq) (gin.H, *errs.Error) {
	if !s.aiCli.Enabled() {
		return nil, errs.ErrAIDisabled
	}
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	tplIDs := pointTemplateIDs(s.db, point.ID)
	if len(tplIDs) == 0 {
		return nil, errs.ErrParam.WithMsg("该点位未绑定检查项模板")
	}
	var tplItem insmodel.CheckTemplateItem
	tplErr := s.db.Where("template_id IN ? AND name = ?", tplIDs, req.Name).First(&tplItem).Error
	if tplErr != nil {
		// 标签抽查合成项（equipment_date_spot，绑定即启用不在模板内）：按任务上下文重算触发，命中才放行
		if !strings.HasPrefix(req.Name, eqsvc.SpotItemPrefix) {
			return nil, errs.ErrParam.WithMsg("检查项「" + req.Name + "」不属于该点位模板")
		}
		_, spotMap, be := s.equipmentSynthetics(&task, &point)
		if be != nil {
			return nil, be
		}
		if _, ok := spotMap[req.Name]; !ok {
			return nil, errs.ErrParam.WithMsg("该抽查项当前未触发")
		}
		tplItem = insmodel.CheckTemplateItem{
			Name: req.Name, JudgeType: ai.JudgeLabel,
			Requirement: strPtr("读取消防/物业设备标签或钢印上的日期。reading 严格输出：M{生产年月}|W{维修年月}，读不到写 无，如 M2020-05|W2025-03 或 M2020-05|W无 或 M无|W无；除 reading 外不要输出日期"),
		}
	} else if ai.NormalizeJudgeType(tplItem.JudgeType) == ai.JudgeManual {
		return nil, errs.ErrParam.WithMsg("手动确认项无需 AI 识别")
	}
	// file_ids 仅接受 upload_file.id。
	fileIDs := make([]string, 0, len(req.FileIDs))
	for _, ref := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.UserID != inspectorID {
			return nil, errs.ErrPhotoNotUploaded
		}
		fileIDs = append(fileIDs, f.ID)
	}
	payload := aiItemJobPayload{
		JobID: uuid.NewString(), UserID: inspectorID,
		TaskID: task.ID, PointID: req.PointID, CommunityID: task.CommunityID, TenantID: task.TenantID,
		PointName: point.Name, PointType: point.Type,
		Name: tplItem.Name, Requirement: strutil.StrVal(tplItem.Requirement), AIHint: strutil.StrVal(tplItem.AIHint),
		JudgeType: tplItem.JudgeType, JudgeConfig: tplItem.JudgeConfig, Tags: tplItem.Tags,
		FileIDs: fileIDs,
	}
	// 过程草稿实时落库：提交即记 pending（重新识别时重置旧结论），worker 识别完回写 done/failed。
	// 草稿仅作过程记录，不影响任务进度；点位正式提交成功后才删除。
	draft := insmodel.CheckinItemDraft{
		TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
		InspectorID: inspectorID, CommunityID: task.CommunityID,
		ItemName: tplItem.Name, DraftKind: insmodel.DraftKindAI,
		JobID: payload.JobID, FileIDs: fileIDs, AIStatus: insmodel.ItemDraftPending,
		ShootLng: req.ShootLng, ShootLat: req.ShootLat, ShootAt: parseShootAt(req.ShootAt),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}, {Name: "point_id"}, {Name: "item_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"inspector_id", "community_id", "tenant_id", "draft_kind", "job_id", "file_ids",
			"exception_type", "ai_status", "ai_verdict", "ai_reason", "ai_reading", "abnormal_tags", "quality_pass", "quality_issue",
			"manual_pass", "manual_note",
			"shoot_lng", "shoot_lat", "shoot_at", "updated_at",
		}),
	}).Create(&draft).Error; err != nil {
		return nil, errs.ErrInternal
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, errs.ErrInternal
	}
	key := aiItemJobPfx + payload.JobID
	// 先入结果 hash（pending）再入队，保证提交后即可轮询到 pending 状态
	if err := s.rdb.HSet(ctx, key, "status", "pending", "user_id", inspectorID).Err(); err != nil {
		return nil, errs.ErrInternal
	}
	s.rdb.Expire(ctx, key, aiItemJobTTL)
	if err := s.rdb.LPush(ctx, aiItemQueueKey, raw).Err(); err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"job_id": payload.JobID}, nil
}

// AIItemJobs 批量查询逐项识别结果（GET /checkin/ai-item-jobs?ids=a,b,c，≤20 个）。
// job 不存在/已过期/非本人 → status=failed + reason"任务已过期，请重新提交识别"（防枚举，不区分原因）。
func (s *CheckinService) AIItemJobs(ctx context.Context, inspectorID, idsRaw string) (gin.H, *errs.Error) {
	ids := make([]string, 0, aiItemMaxQuery)
	for _, id := range strings.Split(idsRaw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, errs.ErrParam.WithMsg("ids 不能为空")
	}
	if len(ids) > aiItemMaxQuery {
		return nil, errs.ErrParam.WithMsg("单次最多查询 20 个识别任务")
	}
	jobs := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		m, err := s.rdb.HGetAll(ctx, aiItemJobPfx+id).Result()
		if err != nil || len(m) == 0 || m["user_id"] != inspectorID {
			jobs = append(jobs, gin.H{"job_id": id, "status": "failed", "reason": "任务已过期，请重新提交识别"})
			continue
		}
		job := gin.H{
			"job_id": id, "status": m["status"],
			"verdict": m["verdict"], "reason": m["reason"], "reading": m["reading"],
			"quality_issue": m["quality_issue"],
		}
		if v, ok := m["quality_pass"]; ok && v != "" {
			job["quality_pass"] = v == "true"
		}
		// 异常 tag（JSON 数组串，识别完成时写入）
		if v := m["abnormal_tags"]; v != "" {
			var abn []string
			if json.Unmarshal([]byte(v), &abn) == nil {
				job["abnormal_tags"] = abn
			}
		}
		jobs = append(jobs, job)
	}
	return gin.H{"jobs": jobs}, nil
}

// StartAIItemWorkers 启动逐项识别队列消费 worker（N=ai.worker_count 默认 8，兼容旧键 ai.worker_concurrency；服务启动时读取一次）。
func (s *CheckinService) StartAIItemWorkers() {
	// 上次进程异常退出时，处理中列表里的任务尚未确认完成，先放回待处理队列。
	for {
		raw, err := s.rdb.RPopLPush(context.Background(), aiItemWorkKey, aiItemQueueKey).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			logger.L.Warn("恢复逐项识别任务失败", zap.Error(err))
			break
		}
		if raw == "" {
			break
		}
	}
	n := s.cfgInt("ai.worker_count", s.cfgInt("ai.worker_concurrency", 8))
	for i := 0; i < n; i++ {
		go s.aiItemWorker(i)
	}
	logger.L.Info("逐项 AI 识别队列已启动", zap.Int("workers", n))
}

// aiItemWorker BLPOP 消费循环（无任务时 5s 阻塞唤醒重试，随进程退出结束）。
func (s *CheckinService) aiItemWorker(idx int) {
	defer func() {
		if r := recover(); r != nil {
			logger.L.Error("逐项 AI 识别 worker panic", zap.Int("worker", idx), zap.Any("panic", r))
		}
	}()
	ctx := context.Background()
	for {
		res, err := s.rdb.BRPopLPush(ctx, aiItemQueueKey, aiItemWorkKey, 5*time.Second).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			logger.L.Warn("逐项 AI 识别队列读取失败", zap.Int("worker", idx), zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		raw := res
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("逐项识别任务 panic，将重回队列", zap.Int("worker", idx), zap.Any("panic", r))
					_ = s.rdb.LPush(ctx, aiItemQueueKey, raw).Err()
					_ = s.rdb.LRem(ctx, aiItemWorkKey, 1, raw).Err()
				}
			}()
			s.processAIItemJob(ctx, raw)
			_ = s.rdb.LRem(ctx, aiItemWorkKey, 1, raw).Err()
		}()
	}
}

// processAIItemJob 执行单个识别任务：复用 ReviewCheckin（质量+内容一次调用，单项 ItemPhotos），
// 结果写 Redis hash（TTL 2h）；单项结论按名称匹配，找不到回落整体 Verdict。
func (s *CheckinService) processAIItemJob(ctx context.Context, raw string) {
	var p aiItemJobPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		logger.L.Warn("逐项 AI 识别任务载荷解析失败", zap.Error(err))
		return
	}
	key := aiItemJobPfx + p.JobID
	// 草稿回写：按 (task, point, item) 定位过程草稿行（与 Redis 结果并行，DB 做持久记录）
	writeDraft := func(fields map[string]any) {
		fields["updated_at"] = time.Now()
		if err := s.db.Model(&insmodel.CheckinItemDraft{}).
			Where("task_id = ? AND point_id = ? AND item_name = ?", p.TaskID, p.PointID, p.Name).
			Updates(fields).Error; err != nil {
			logger.L.Warn("逐项识别草稿回写失败", zap.String("job_id", p.JobID), zap.Error(err))
		}
	}
	fail := func(msg string) {
		s.rdb.HSet(ctx, key, "status", "failed", "reason", strutil.Truncate(msg, 200))
		s.rdb.Expire(ctx, key, aiItemJobTTL)
		writeDraft(map[string]any{"ai_status": insmodel.ItemDraftFailed, "ai_reason": strutil.Truncate(msg, 200)})
	}
	refs := make([]ai.PhotoRef, 0, len(p.FileIDs))
	for _, ref := range p.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil {
			fail("识别照片不存在或已失效")
			return
		}
		refs = append(refs, ai.PhotoRef{URL: f.URL})
	}
	input := ai.ReviewInput{
		PointName: p.PointName, PointType: p.PointType, CheckItems: []string{p.Name},
		ItemPhotos: []ai.ItemPhoto{{
			Name: p.Name, Requirement: p.Requirement, AIHint: p.AIHint,
			JudgeType: p.JudgeType, JudgeConfig: p.JudgeConfig, Tags: p.Tags, Photos: refs,
		}},
	}
	res, err := s.reviewItemWithRetry(ctx, p.JobID, input)
	if err != nil {
		logger.L.Warn("逐项 AI 识别调用失败", zap.String("job_id", p.JobID), zap.Error(err))
		fail("AI 识别失败：" + err.Error())
		return
	}
	// 单项结果映射：ReviewResult.Items 找同名项，找不到用整体 Verdict；质量结论一并记录
	verdict, reason, reading := res.Verdict, res.Reason, ""
	var abnTags []string
	for _, iv := range res.Items {
		if iv.Name == p.Name {
			verdict, reason, reading = iv.Verdict, iv.Reason, iv.Reading
			abnTags = iv.AbnormalTags
			break
		}
	}
	// 异常 tag 过滤到该项模板 tags（模型可能杜撰原名）
	if len(abnTags) > 0 && len(p.Tags) > 0 {
		allow := map[string]bool{}
		for _, t := range p.Tags {
			allow[t] = true
		}
		filtered := abnTags[:0]
		for _, t := range abnTags {
			if allow[t] {
				filtered = append(filtered, t)
			}
		}
		abnTags = filtered
	} else {
		abnTags = nil
	}
	abnJSON, _ := json.Marshal(abnTags)
	if len(abnTags) > 0 && verdict == ai.VerdictPass {
		verdict = ai.VerdictReview // 有异常 tag 的项结论至少存疑（与打卡提交口径一致）
	}
	s.rdb.HSet(ctx, key,
		"status", "done",
		"verdict", verdict,
		"reason", strutil.Truncate(reason, 500),
		"reading", strutil.Truncate(strings.TrimSpace(reading), 64),
		"abnormal_tags", string(abnJSON),
		"quality_pass", strconv.FormatBool(res.Quality.Pass),
		"quality_issue", strutil.Truncate(res.Quality.Issue, 255),
	)
	s.rdb.Expire(ctx, key, aiItemJobTTL)
	var readingPtr *string
	if rd := strutil.Truncate(strings.TrimSpace(reading), 64); rd != "" {
		readingPtr = &rd
	}
	writeDraft(map[string]any{
		"ai_status": insmodel.ItemDraftDone, "ai_verdict": verdict,
		"ai_reason": strutil.Truncate(reason, 500), "ai_reading": readingPtr,
		"abnormal_tags": types.StringArray(abnTags),
		"quality_pass":  res.Quality.Pass, "quality_issue": strutil.Truncate(res.Quality.Issue, 255),
	})
}

// reviewItemWithRetry 调大模型识别：限流（429）时指数退避重试（2s/4s/8s，最多 3 次），
// 其余错误直接返回。每次尝试独立计时 ai.timeout_seconds（默认 180s，Client 内部控制）。
func (s *CheckinService) reviewItemWithRetry(ctx context.Context, jobID string, input ai.ReviewInput) (*ai.ReviewResult, error) {
	backoff := aiItemRetryBase
	for attempt := 0; ; attempt++ {
		res, err := s.aiCli.ReviewCheckin(ctx, input)
		if err == nil || !ai.IsRateLimited(err) || attempt >= aiItemRetryMax {
			return res, err
		}
		logger.L.Warn("逐项 AI 识别限流，退避后重试",
			zap.String("job_id", jobID), zap.Int("attempt", attempt+1), zap.Duration("backoff", backoff))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// ItemDrafts 查询逐项识别/手动项过程草稿（GET /checkin/item-drafts?task_id[&point_id]）。
// point_id 为空返回整个任务的全部草稿（向导进入时一次拉取重建进度）；仅本人任务可见。
// 草稿是巡检进度的唯一事实来源：App 本地不存快照，断点恢复完全以服务端为准。
// 草稿在点位正式提交成功后删除——查不到草稿且点位未打卡即表示尚未开始。
func (s *CheckinService) ItemDrafts(ctx context.Context, inspectorID, taskID, pointID string) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	q := s.db.Where("task_id = ? AND inspector_id = ?", taskID, inspectorID)
	if pointID != "" {
		q = q.Where("point_id = ?", pointID)
	}
	var drafts []insmodel.CheckinItemDraft
	q.Order("created_at").Find(&drafts)
	items := make([]gin.H, 0, len(drafts))
	for _, d := range drafts {
		photos := make([]string, 0, len(d.FileIDs))
		for _, ref := range d.FileIDs {
			if f, err := uploadfile.ByID(s.db, ref); err == nil {
				photos = append(photos, f.URL)
			}
		}
		items = append(items, gin.H{
			"point_id": d.PointID, "item_name": d.ItemName, "draft_kind": d.DraftKind, "job_id": d.JobID,
			"file_ids": d.FileIDs, "photos": photos, "exception_type": d.ExceptionType,
			"ai_status": d.AIStatus, "ai_verdict": d.AIVerdict, "ai_reason": d.AIReason,
			"ai_reading": d.AIReading, "abnormal_tags": d.AbnormalTags, "quality_pass": d.QualityPass, "quality_issue": d.QualityIssue,
			"manual_pass": d.ManualPass, "manual_note": d.ManualNote,
			"updated_at": timefmt.T(d.UpdatedAt),
		})
	}
	out := gin.H{"items": items}
	// 传了 point_id 时附带该点位的凭证核验草稿（§14.2 断点恢复：重进向导不再要求重新签到），无则 null
	if pointID != "" {
		out["credential"] = s.pointCredDraftView(taskID, pointID, inspectorID)
	}
	return out, nil
}

// pointCredDraftView 该点位本人凭证核验草稿视图（无草稿返回 nil）。
func (s *CheckinService) pointCredDraftView(taskID, pointID, inspectorID string) any {
	var d insmodel.CheckinPointCredDraft
	if err := s.db.Where("task_id = ? AND point_id = ? AND inspector_id = ?", taskID, pointID, inspectorID).
		First(&d).Error; err != nil {
		return nil
	}
	return gin.H{
		"checkin_type": d.CheckinType, "cred_no": d.CredNo,
		"fence_distance": d.FenceDistance, "verified_at": timefmt.T(d.VerifiedAt),
	}
}

// SavePointCredDraft 点位凭证核验落草稿（POST /checkin/point-cred，§14.2）：
// 扫码/NFC/围栏核验通过即 upsert（verified_at=now()）；校验口径同逐项草稿（任务归属/点位在任务内/任务未完成）。
// 草稿仅恢复 UI 状态，正式提交的凭证复核口径不变（checkMode 仍逐项比对）。
func (s *CheckinService) SavePointCredDraft(ctx context.Context, inspectorID string, req *dto.PointCredDraftReq) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	draft := insmodel.CheckinPointCredDraft{
		TenantID: task.TenantID, CommunityID: task.CommunityID,
		TaskID: task.ID, PointID: req.PointID, InspectorID: inspectorID,
		CheckinType: req.CheckinType, CredNo: strutil.Truncate(strings.TrimSpace(req.CredNo), 128),
		FenceDistance: req.FenceDistance, VerifiedAt: time.Now(),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}, {Name: "point_id"}, {Name: "inspector_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"tenant_id", "community_id", "checkin_type", "cred_no", "fence_distance", "verified_at", "updated_at",
		}),
	}).Create(&draft).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"saved": true}, nil
}

// DeleteItemDraft 撤销某一项的过程草稿（DELETE /checkin/item-drafts?task_id&point_id&item_name）。
// 用于逃生撤销（设备不存在/无法拍摄选错后回到待拍）：删除草稿行；进行中的 AI job 完成时
// 按 (task, point, item) Updates 定位不到行自然不落库，不会复活草稿。
func (s *CheckinService) DeleteItemDraft(ctx context.Context, inspectorID, taskID, pointID, itemName string) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	if pointID == "" || itemName == "" {
		return nil, errs.ErrParam
	}
	res := s.db.Where("task_id = ? AND point_id = ? AND item_name = ? AND inspector_id = ?",
		taskID, pointID, itemName, inspectorID).Delete(&insmodel.CheckinItemDraft{})
	if res.Error != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"deleted": res.RowsAffected}, nil
}

// SaveManualDraft 手动确认项选择落草稿（POST /checkin/item-drafts/manual）。
// 校验任务归属与「该项确为该点位模板的手动项」；ai_verdict 保持 NULL（最终提交不会误当 AI 结论）。
func (s *CheckinService) SaveManualDraft(ctx context.Context, inspectorID string, req *dto.ManualItemDraftReq) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	tplIDs := pointTemplateIDs(s.db, point.ID)
	if len(tplIDs) == 0 {
		return nil, errs.ErrParam.WithMsg("该点位未绑定检查项模板")
	}
	var tplItem insmodel.CheckTemplateItem
	if err := s.db.Where("template_id IN ? AND name = ?", tplIDs, req.Name).First(&tplItem).Error; err != nil {
		return nil, errs.ErrParam.WithMsg("检查项「" + req.Name + "」不属于该点位模板")
	}
	// 手动档向导的拍照项：照片归属校验 + 异常 tag 归一（⊆ 模板项 tags）
	fileIDs := make([]string, 0, len(req.FileIDs))
	for _, ref := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.UserID != inspectorID {
			return nil, errs.ErrPhotoNotUploaded
		}
		fileIDs = append(fileIDs, f.ID)
	}
	abn := make(types.StringArray, 0, len(req.AbnormalTags))
	allow := map[string]bool{}
	for _, t := range tplItem.Tags {
		allow[t] = true
	}
	for _, t := range req.AbnormalTags {
		if allow[t] {
			abn = append(abn, t)
		}
	}
	draft := insmodel.CheckinItemDraft{
		TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
		InspectorID: inspectorID, CommunityID: task.CommunityID,
		ItemName: tplItem.Name, DraftKind: insmodel.DraftKindManual,
		AIStatus:   insmodel.ItemDraftDone, // manual/escape 行无 AI 流程，ai_status 仅作"已定稿"标记（消费端按 draft_kind 分发）
		ManualPass: &req.Pass, ManualNote: strutil.Truncate(strings.TrimSpace(req.Note), 512),
		FileIDs: types.StringArray(fileIDs), AbnormalTags: abn,
		ShootLng: req.ShootLng, ShootLat: req.ShootLat, ShootAt: parseShootAt(req.ShootAt),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}, {Name: "point_id"}, {Name: "item_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"inspector_id", "community_id", "tenant_id", "draft_kind", "ai_status", "manual_pass", "manual_note", "file_ids", "abnormal_tags",
			"job_id", "exception_type", "ai_verdict", "ai_reason", "ai_reading", "quality_pass", "quality_issue",
			"shoot_lng", "shoot_lat", "shoot_at", "updated_at",
		}),
	}).Create(&draft).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"saved": true}, nil
}

// SavePhotoItemAbnormalDraft 拍照项异常逃生入口：设备不存在/无法拍摄/相机故障时落逃生草稿（draft_kind=escape）。
// 佐证分流：device_missing 须恰好 1 张佐证照片；unable_to_capture/camera_broken 免佐证（file_ids 可为空，带 1 张也收）。
func (s *CheckinService) SavePhotoItemAbnormalDraft(ctx context.Context, inspectorID string, req *dto.PhotoItemAbnormalDraftReq) (gin.H, *errs.Error) {
	if !validItemExceptionType(req.ExceptionType) || req.ExceptionType == "label_missing" {
		// label_missing 是标签抽查合成项的逃生类型，随正式提交直连 resolveSpotItem，不落模板项逃生草稿
		return nil, errs.ErrParam.WithMsg("异常类型无效")
	}
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	tplIDs := pointTemplateIDs(s.db, point.ID)
	if len(tplIDs) == 0 {
		return nil, errs.ErrParam.WithMsg("该点位未绑定检查项模板")
	}
	var tplItem insmodel.CheckTemplateItem
	if err := s.db.Where("template_id IN ? AND name = ?", tplIDs, req.Name).First(&tplItem).Error; err != nil {
		return nil, errs.ErrParam.WithMsg("检查项「" + req.Name + "」不属于该点位模板")
	}
	if ai.NormalizeJudgeType(tplItem.JudgeType) == ai.JudgeManual {
		return nil, errs.ErrParam.WithMsg("手动确认项请直接选择正常/异常")
	}
	// 佐证分流：device_missing 须恰好 1 张佐证照片；unable_to_capture/camera_broken 免佐证（0 或 1 张均可）
	fileIDs := make([]string, 0, len(req.FileIDs))
	if req.ExceptionType == "device_missing" && len(req.FileIDs) != 1 {
		return nil, errs.ErrParam.WithMsg("设备不存在须上传 1 张佐证照片")
	}
	for _, ref := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.UserID != inspectorID {
			return nil, errs.ErrPhotoNotUploaded
		}
		fileIDs = append(fileIDs, f.ID)
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		switch req.ExceptionType {
		case "device_missing":
			note = "设备确实不存在，已上报异常"
		case "camera_broken":
			note = "相机故障无法拍摄，已上报异常"
		default:
			note = "现场无法拍摄，已上报异常"
		}
	}
	draft := insmodel.CheckinItemDraft{
		TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
		InspectorID: inspectorID, CommunityID: task.CommunityID,
		ItemName: tplItem.Name, DraftKind: insmodel.DraftKindEscape, FileIDs: fileIDs,
		AIStatus:      insmodel.ItemDraftDone, // escape 行无 AI 流程，ai_status 仅作"已定稿"标记（消费端按 draft_kind 分发）
		ExceptionType: req.ExceptionType,
		AIReason:      strPtr(note),
		ShootLng:      req.ShootLng, ShootLat: req.ShootLat, ShootAt: parseShootAt(req.ShootAt),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "task_id"}, {Name: "point_id"}, {Name: "item_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"inspector_id", "community_id", "tenant_id", "draft_kind", "job_id", "file_ids",
			"exception_type", "ai_status", "ai_verdict", "ai_reason", "ai_reading", "abnormal_tags", "quality_pass", "quality_issue",
			"manual_pass", "manual_note",
			"shoot_lng", "shoot_lat", "shoot_at", "updated_at",
		}),
	}).Create(&draft).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"saved": true}, nil
}

func strPtr(s string) *string { return &s }

// parseShootAt 拍照时间解析（YYYY-MM-DD HH:mm:ss；空串/解析失败返回 nil，落库存 NULL）。
func parseShootAt(s string) *time.Time {
	t, err := timefmt.Parse(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &t
}

// pointTemplateIDs 点位关联模板 ID 列表（point_template，sort 升序；无关联返回空）。
func pointTemplateIDs(db *gorm.DB, pointID string) []string {
	var ids []string
	db.Model(&insmodel.PointTemplate{}).Where("point_id = ?", pointID).Order("sort ASC").Pluck("template_id", &ids)
	return ids
}

func validItemExceptionType(exceptionType string) bool {
	// label_missing 仅标签抽查合成项可用（模板项逃生校验在 SavePhotoItemAbnormalDraft 拦截）
	return exceptionType == "device_missing" || exceptionType == "unable_to_capture" ||
		exceptionType == "camera_broken" || exceptionType == "label_missing" || exceptionType == "ai_failed"
}
