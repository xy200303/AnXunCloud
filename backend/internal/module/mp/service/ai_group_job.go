// 整组 AI 识别：photo_mode=group 点位拍 1 张整组照片，一次大模型调用识别该点位全部模板检查项
// （质量+内容一次调用，复用 ReviewCheckin 的 GroupPhotos 整组模式），App 轮询 job 状态取回逐项结论与设备数量。
// 结果：Redis hash ai:group:job:{id}（status/result(JSON)/reason/quality_*，TTL 2 小时）；
// 识别完成后逐项回写 checkin_item_draft 过程草稿（与逐项 job 同语义，断点恢复/进度展示共用）。
// 结果映射口径：pass→normal、abnormal→abnormal、review/缺失→unrecognized（读不出，默认正常不拦截）；
// count 为识别到的设备/设施数量，供 App 端与点位绑定设备数比对。
package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

const (
	aiGroupJobPfx = "ai:group:job:" // 整组识别结果 hash 前缀
	aiGroupJobTTL = 2 * time.Hour   // 结果保留时长（过期按"任务已过期"处理）

	// 整组识别逐项结论（对 App 的结果口径）
	groupResultNormal       = "normal"
	groupResultAbnormal     = "abnormal"
	groupResultUnrecognized = "unrecognized" // 读不出（默认正常不拦截）
)

// aiGroupResultItem 整组识别逐项结论。
type aiGroupResultItem struct {
	Name   string `json:"name"`
	Result string `json:"result"` // normal/abnormal/unrecognized
	Reason string `json:"reason"`
	Value  string `json:"value,omitempty"` // 识别到的日期/数值（可空）
	// AbnormalTags 异常观察点 tag（已过滤到模板 tags；可空）
	AbnormalTags []string `json:"abnormal_tags,omitempty"`
}

// aiGroupResult 整组识别结果（存 Redis hash result 字段，JSON）。
type aiGroupResult struct {
	Count int                 `json:"count"` // 识别到的设备/设施数量（供与点位绑定设备数比对）
	Items []aiGroupResultItem `json:"items"`
}

// aiGroupJobPayload 整组识别任务载荷（point/项上下文入队时快照，执行时不再查模板库）。
type aiGroupJobPayload struct {
	JobID       string         `json:"job_id"`
	UserID      string         `json:"user_id"`
	TaskID      string         `json:"task_id"`
	PointID     string         `json:"point_id"`
	CommunityID string         `json:"community_id"`
	TenantID    *string        `json:"tenant_id,omitempty"`
	PointName   string         `json:"point_name"`
	PointType   string         `json:"point_type"`
	Items       []ai.ItemPhoto `json:"items"` // 仅判定参数元数据（name/requirement/ai_hint/judge_*），Photos 不用
	FileIDs     []string       `json:"file_ids"`
}

// SubmitAIGroupJob 提交整组 AI 识别任务（POST /checkin/ai-group-jobs）。
// 仅 photo_mode=group（全部关联模板均为整组模式）的点位可用；判定参数从点位模板项自取，防客户端伪造；
// 手动确认项（judge_type=manual）不调 AI，不进整组识别清单。
func (s *CheckinService) SubmitAIGroupJob(ctx context.Context, inspectorID string, req *dto.AIGroupJobReq) (gin.H, *errs.Error) {
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
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	tplSet := insmodel.LoadPointTemplateSets(s.db, []string{point.ID})[point.ID]
	if tplSet == nil || len(tplSet.TemplateIDs) == 0 {
		return nil, errs.ErrParam.WithMsg("该点位未绑定检查项模板")
	}
	if tplSet.PhotoMode != insmodel.PhotoModeGroup {
		return nil, errs.ErrParam.WithMsg("该点位含逐项拍照模板，请走逐项识别流程")
	}
	items := make([]ai.ItemPhoto, 0, len(tplSet.Items))
	for _, it := range tplSet.Items {
		if ai.NormalizeJudgeType(it.JudgeType) == ai.JudgeManual {
			continue // 手动确认项不调 AI
		}
		items = append(items, ai.ItemPhoto{
			Name: it.Name, Requirement: strutil.StrVal(it.Requirement), AIHint: strutil.StrVal(it.AIHint),
			JudgeType: it.JudgeType, JudgeConfig: it.JudgeConfig, Tags: it.Tags,
		})
	}
	if len(items) == 0 {
		return nil, errs.ErrParam.WithMsg("该点位无可 AI 识别的检查项")
	}
	// file_ids 仅接受 upload_file.id（整组照片恰好 1 张，binding 已约束）。
	fileIDs := make([]string, 0, len(req.FileIDs))
	for _, ref := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.UserID != inspectorID {
			return nil, errs.ErrPhotoNotUploaded
		}
		fileIDs = append(fileIDs, f.ID)
	}
	payload := aiGroupJobPayload{
		JobID: uuid.NewString(), UserID: inspectorID,
		TaskID: task.ID, PointID: req.PointID, CommunityID: task.CommunityID, TenantID: task.TenantID,
		PointName: point.Name, PointType: point.Type,
		Items: items, FileIDs: fileIDs,
	}
	// 过程草稿实时落库：提交即记 pending（重新识别时重置旧结论），识别完回写 done/failed。
	for _, it := range items {
		draft := insmodel.CheckinItemDraft{
			TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
			InspectorID: inspectorID, CommunityID: task.CommunityID,
			ItemName: it.Name, JobID: payload.JobID, FileIDs: fileIDs, AIStatus: insmodel.ItemDraftPending,
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}, {Name: "point_id"}, {Name: "item_name"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"inspector_id", "community_id", "tenant_id", "job_id", "file_ids",
				"exception_type", "ai_status", "ai_verdict", "ai_reason", "ai_reading", "abnormal_tags", "quality_pass", "quality_issue", "updated_at",
			}),
		}).Create(&draft).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	key := aiGroupJobPfx + payload.JobID
	// 先入结果 hash（pending）再异步执行，保证提交后即可轮询到 pending 状态
	if err := s.rdb.HSet(ctx, key, "status", "pending", "user_id", inspectorID).Err(); err != nil {
		return nil, errs.ErrInternal
	}
	s.rdb.Expire(ctx, key, aiGroupJobTTL)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L.Error("整组 AI 识别任务 panic", zap.String("job_id", payload.JobID), zap.Any("panic", r))
			}
		}()
		s.processAIGroupJob(payload)
	}()
	return gin.H{"job_id": payload.JobID}, nil
}

// AIGroupJob 查询整组识别结果（GET /checkin/ai-group-jobs/:id）。
// job 不存在/已过期/非本人 → status=failed + reason"任务已过期，请重新提交识别"（防枚举，不区分原因）。
func (s *CheckinService) AIGroupJob(ctx context.Context, inspectorID, id string) (gin.H, *errs.Error) {
	m, err := s.rdb.HGetAll(ctx, aiGroupJobPfx+id).Result()
	if err != nil || len(m) == 0 || m["user_id"] != inspectorID {
		return gin.H{"job_id": id, "status": "failed", "reason": "任务已过期，请重新提交识别"}, nil
	}
	job := gin.H{
		"job_id": id, "status": m["status"],
		"reason": m["reason"], "quality_issue": m["quality_issue"],
	}
	if v, ok := m["quality_pass"]; ok && v != "" {
		job["quality_pass"] = v == "true"
	}
	if raw := m["result"]; raw != "" {
		job["result"] = json.RawMessage(raw)
	}
	return job, nil
}

// processAIGroupJob 执行整组识别任务：一张整组照片 + 全部检查项判定参数一次调用（GroupPhotos 整组模式），
// 逐项结论映射为 normal/abnormal/unrecognized 后存 Redis hash（result JSON），并逐项回写过程草稿。
func (s *CheckinService) processAIGroupJob(p aiGroupJobPayload) {
	ctx := context.Background()
	key := aiGroupJobPfx + p.JobID
	s.rdb.HSet(ctx, key, "status", "running")
	s.rdb.Expire(ctx, key, aiGroupJobTTL)
	// 草稿回写：按 (task, point, item) 定位过程草稿行（与 Redis 结果并行，DB 做持久记录）
	writeDraft := func(itemName string, fields map[string]any) {
		fields["updated_at"] = time.Now()
		if err := s.db.Model(&insmodel.CheckinItemDraft{}).
			Where("task_id = ? AND point_id = ? AND item_name = ?", p.TaskID, p.PointID, itemName).
			Updates(fields).Error; err != nil {
			logger.L.Warn("整组识别草稿回写失败", zap.String("job_id", p.JobID), zap.String("item", itemName), zap.Error(err))
		}
	}
	fail := func(msg string) {
		s.rdb.HSet(ctx, key, "status", "failed", "reason", strutil.Truncate(msg, 200))
		s.rdb.Expire(ctx, key, aiGroupJobTTL)
		for _, it := range p.Items {
			writeDraft(it.Name, map[string]any{"ai_status": insmodel.ItemDraftFailed, "ai_reason": strutil.Truncate(msg, 200)})
		}
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
	names := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		names = append(names, it.Name)
	}
	input := ai.ReviewInput{
		PointName: p.PointName, PointType: p.PointType, CheckItems: names,
		ItemPhotos: p.Items, GroupPhotos: refs,
	}
	res, err := s.aiCli.ReviewCheckin(ctx, input)
	if err != nil {
		logger.L.Warn("整组 AI 识别调用失败", zap.String("job_id", p.JobID), zap.Error(err))
		fail("AI 识别失败：" + err.Error())
		return
	}
	// 逐项结果映射：按名称匹配；pass→normal、abnormal→abnormal、review/未返回→unrecognized（默认正常不拦截）
	byName := map[string]ai.ItemVerdict{}
	for _, iv := range res.Items {
		byName[iv.Name] = iv
	}
	count := 0
	if res.Count != nil {
		count = *res.Count
	}
	result := aiGroupResult{Count: count, Items: make([]aiGroupResultItem, 0, len(p.Items))}
	for _, it := range p.Items {
		iv, ok := byName[it.Name]
		out := aiGroupResultItem{Name: it.Name, Result: groupResultUnrecognized, Reason: "AI 未识别出该项，默认正常"}
		verdict, reading := ai.VerdictReview, ""
		if ok {
			out.Reason = strutil.Truncate(iv.Reason, 500)
			switch iv.Verdict {
			case ai.VerdictPass:
				out.Result = groupResultNormal
				verdict = ai.VerdictPass
			case ai.VerdictAbnormal:
				out.Result = groupResultAbnormal
				verdict = ai.VerdictAbnormal
			default:
				out.Result = groupResultUnrecognized
			}
			reading = strings.TrimSpace(iv.Reading)
			out.Value = reading
			// 异常 tag 过滤到该项模板 tags（模型可能杜撰原名）
			if len(iv.AbnormalTags) > 0 && len(it.Tags) > 0 {
				allow := map[string]bool{}
				for _, t := range it.Tags {
					allow[t] = true
				}
				for _, t := range iv.AbnormalTags {
					if allow[t] {
						out.AbnormalTags = append(out.AbnormalTags, t)
					}
				}
			}
			if len(out.AbnormalTags) > 0 && out.Result == groupResultNormal {
				out.Result = groupResultUnrecognized // 有异常 tag 的项至少存疑（与打卡提交口径一致）
				verdict = ai.VerdictReview
			}
		}
		result.Items = append(result.Items, out)
		var readingPtr *string
		if rd := strutil.Truncate(reading, 64); rd != "" {
			readingPtr = &rd
		}
		writeDraft(it.Name, map[string]any{
			"ai_status": insmodel.ItemDraftDone, "ai_verdict": verdict,
			"ai_reason": strutil.Truncate(out.Reason, 500), "ai_reading": readingPtr,
			"abnormal_tags": types.StringArray(out.AbnormalTags),
			"quality_pass":  res.Quality.Pass, "quality_issue": strutil.Truncate(res.Quality.Issue, 255),
		})
	}
	raw, err := json.Marshal(result)
	if err != nil {
		fail("结果序列化失败")
		return
	}
	s.rdb.HSet(ctx, key,
		"status", "done",
		"result", string(raw),
		"quality_pass", strconv.FormatBool(res.Quality.Pass),
		"quality_issue", strutil.Truncate(res.Quality.Issue, 255),
	)
	s.rdb.Expire(ctx, key, aiGroupJobTTL)
}
