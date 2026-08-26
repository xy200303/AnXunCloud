// 逐项 AI 识别队列：巡检员按检查项提交 1~3 张照片，异步调大模型识别（质量+内容一次调用），
// App 轮询 job 状态取回逐项结论，确认后随打卡提交（ai_confirmed=true）。
// 队列：Redis list ai:item:queue（LPUSH 入队，N 个 worker BLPOP 消费，N=ai.worker_concurrency 默认 4）；
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

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"

	"go.uber.org/zap"
)

const (
	aiItemQueueKey = "ai:item:queue" // 逐项识别任务队列（list）
	aiItemJobPfx   = "ai:item:job:"  // 逐项识别结果 hash 前缀
	aiItemJobTTL   = 2 * time.Hour   // 结果保留时长（过期按"任务已过期"处理）
	aiItemMaxQuery = 20              // 单次批量查询上限
)

// aiItemJobPayload 队列任务载荷（point/item 上下文入队时快照，worker 不再查库）。
type aiItemJobPayload struct {
	JobID       string         `json:"job_id"`
	UserID      string         `json:"user_id"`
	PointName   string         `json:"point_name"`
	PointType   string         `json:"point_type"`
	Name        string         `json:"name"`
	Requirement string         `json:"requirement,omitempty"`
	AIHint      string         `json:"ai_hint,omitempty"`
	JudgeType   string         `json:"judge_type,omitempty"`
	JudgeConfig map[string]any `json:"judge_config,omitempty"`
	FileKeys    []string       `json:"file_keys"`
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
	if !insmodel.TaskPointIDs(&task, &plan).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	if point.TemplateID == nil || *point.TemplateID == "" {
		return nil, errs.ErrParam.WithMsg("该点位未绑定检查项模板")
	}
	var tplItem insmodel.CheckTemplateItem
	if err := s.db.Where("template_id = ? AND name = ?", *point.TemplateID, req.Name).First(&tplItem).Error; err != nil {
		return nil, errs.ErrParam.WithMsg("检查项「" + req.Name + "」不属于该点位模板")
	}
	if ai.NormalizeJudgeType(tplItem.JudgeType) == ai.JudgeManual {
		return nil, errs.ErrParam.WithMsg("手动确认项无需 AI 识别")
	}
	// file_keys 归属校验（同 resolvePhotos 的上传确认逻辑）
	for _, key := range req.FileKeys {
		var count int64
		s.db.Model(&sysmodel.UploadFile{}).Where("file_key = ? AND user_id = ?", key, inspectorID).Count(&count)
		if count == 0 {
			return nil, errs.ErrPhotoNotUploaded
		}
	}
	payload := aiItemJobPayload{
		JobID: uuid.NewString(), UserID: inspectorID,
		PointName: point.Name, PointType: point.Type,
		Name: tplItem.Name, Requirement: strVal(tplItem.Requirement), AIHint: strVal(tplItem.AIHint),
		JudgeType: tplItem.JudgeType, JudgeConfig: tplItem.JudgeConfig,
		FileKeys: req.FileKeys,
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
		jobs = append(jobs, job)
	}
	return gin.H{"jobs": jobs}, nil
}

// StartAIItemWorkers 启动逐项识别队列消费 worker（N=ai.worker_concurrency，默认 4；服务启动时读取一次）。
func (s *CheckinService) StartAIItemWorkers() {
	n := s.cfgInt("ai.worker_concurrency", 4)
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
		res, err := s.rdb.BLPop(ctx, 5*time.Second, aiItemQueueKey).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			logger.L.Warn("逐项 AI 识别队列读取失败", zap.Int("worker", idx), zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		s.processAIItemJob(ctx, res[1])
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
	fail := func(msg string) {
		s.rdb.HSet(ctx, key, "status", "failed", "reason", truncateStr(msg, 200))
		s.rdb.Expire(ctx, key, aiItemJobTTL)
	}
	refs := make([]ai.PhotoRef, 0, len(p.FileKeys))
	for _, k := range p.FileKeys {
		refs = append(refs, ai.PhotoRef{URL: s.store.URL(k)})
	}
	input := ai.ReviewInput{
		PointName: p.PointName, PointType: p.PointType, CheckItems: []string{p.Name},
		ItemPhotos: []ai.ItemPhoto{{
			Name: p.Name, Requirement: p.Requirement, AIHint: p.AIHint,
			JudgeType: p.JudgeType, JudgeConfig: p.JudgeConfig, Photos: refs,
		}},
	}
	res, err := s.aiCli.ReviewCheckin(ctx, input)
	if err != nil {
		logger.L.Warn("逐项 AI 识别调用失败", zap.String("job_id", p.JobID), zap.Error(err))
		fail("AI 识别失败：" + err.Error())
		return
	}
	// 单项结果映射：ReviewResult.Items 找同名项，找不到用整体 Verdict；质量结论一并记录
	verdict, reason, reading := res.Verdict, res.Reason, ""
	for _, iv := range res.Items {
		if iv.Name == p.Name {
			verdict, reason, reading = iv.Verdict, iv.Reason, iv.Reading
			break
		}
	}
	s.rdb.HSet(ctx, key,
		"status", "done",
		"verdict", verdict,
		"reason", truncateStr(reason, 500),
		"reading", truncateStr(strings.TrimSpace(reading), 64),
		"quality_pass", strconv.FormatBool(res.Quality.Pass),
		"quality_issue", truncateStr(res.Quality.Issue, 255),
	)
	s.rdb.Expire(ctx, key, aiItemJobTTL)
}
