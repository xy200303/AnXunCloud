// 逐项 AI 识别：River 执行，PG 保存任务状态，App 轮询后人工确认再提交。
package service

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	aimodel "anxuncloud/internal/module/ai/model"
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
	"gorm.io/gorm/clause"
)

const (
	aiItemMaxQuery  = 20              // 单次批量查询上限
	aiItemRetryMax  = 3               // 限流重试次数上限（指数退避 2s/4s/8s）
	aiItemRetryBase = 2 * time.Second // 限流重试退避基数
)

// aiItemJobPayload 队列任务载荷（point/item 上下文入队时快照，worker 不再查库）。
// TaskID/PointID/CommunityID 用于 worker 识别完成后回写 checkin_item_draft 过程草稿。
type aiItemJobPayload struct {
	JobID        string   `json:"job_id"`
	RequestID    string   `json:"request_id"`
	RetakeSeq    int      `json:"retake_seq"`
	PhotoSetHash string   `json:"photo_set_hash"`
	UserID       string   `json:"user_id"`
	TaskID       string   `json:"task_id"`
	PointID      string   `json:"point_id"`
	CommunityID  string   `json:"community_id"`
	TenantID     *string  `json:"tenant_id,omitempty"`
	PointName    string   `json:"point_name"`
	PointType    string   `json:"point_type"`
	TemplateID   *string  `json:"template_id"`
	ItemCode     string   `json:"item_code"`
	EquipmentID  *string  `json:"equipment_id"`
	Name         string   `json:"name"`
	Standard     string   `json:"standard,omitempty"`
	AIHint       string   `json:"ai_hint,omitempty"`
	AIPromptKind string   `json:"ai_prompt_kind"`
	ValueType    string   `json:"value_type"`
	Unit         string   `json:"unit,omitempty"`
	ValueMin     *float64 `json:"value_min,omitempty"`
	ValueMax     *float64 `json:"value_max,omitempty"`
	EnumOptions  []string `json:"enum_options,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	FileIDs      []string `json:"file_ids"`
}

// SubmitAIItemJob 提交逐项 AI 识别任务（POST /checkin/ai-item-jobs）。
// 提示类型、值定义和识别要点从任务授权模板自取，ai_enabled=false 直接拒绝。
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
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	if be := RequirePointExecutable(s.db, &point); be != nil {
		return nil, be
	}
	tplItem, be := s.templateItemForDraft(&task, &point, req.CheckItemRef)
	if be != nil {
		return nil, be
	}
	if !tplItem.AIEnabled {
		return nil, errs.ErrParam.WithMsg("该检查项未启用 AI，请人工填写")
	}
	fileIDs := make([]string, 0, len(req.FileIDs))
	for _, ref := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.UserID != inspectorID {
			return nil, errs.ErrPhotoNotUploaded
		}
		fileIDs = append(fileIDs, f.ID)
	}
	jobID := uuid.NewString()
	payload := aiItemJobPayload{
		JobID: jobID, RequestID: uuid.NewString(), PhotoSetHash: draftPhotoHash(fileIDs), UserID: inspectorID,
		TaskID: task.ID, PointID: req.PointID, CommunityID: task.CommunityID, TenantID: task.TenantID,
		PointName: point.Name, PointType: point.Type, ItemCode: tplItem.Code, EquipmentID: req.EquipmentID,
		Name: tplItem.Name, Standard: tplItem.Standard, AIHint: strutil.StrVal(tplItem.AIHint),
		AIPromptKind: tplItem.AIPromptKind, ValueType: tplItem.ValueType, Unit: tplItem.Unit,
		ValueMin: tplItem.ValueMin, ValueMax: tplItem.ValueMax, Tags: tplItem.Tags, FileIDs: fileIDs,
	}
	if tplItem.BuiltinKind == "" {
		payload.TemplateID = &req.TemplateID
	}
	for _, option := range tplItem.EnumOptions {
		payload.EnumOptions = append(payload.EnumOptions, option.Label)
	}
	var previous insmodel.CheckinItemDraft
	identity := draftIdentityWhere(s.db, payload.TemplateID, payload.ItemCode, payload.EquipmentID)
	if err := identity.Where("task_id = ? AND point_id = ?", task.ID, point.ID).First(&previous).Error; err == nil {
		payload.RetakeSeq = previous.RetakeSeq + 1
	}
	draft := insmodel.CheckinItemDraft{
		TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
		InspectorID: inspectorID, CommunityID: task.CommunityID,
		TemplateID: payload.TemplateID, ItemCode: payload.ItemCode, EquipmentID: payload.EquipmentID,
		RequestID: payload.RequestID, RetakeSeq: payload.RetakeSeq, PhotoSetHash: payload.PhotoSetHash,
		DraftKind: insmodel.DraftKindAI, JobID: payload.JobID, FileIDs: fileIDs, AIStatus: insmodel.ItemDraftPending,
		ShootLng: req.ShootLng, ShootLat: req.ShootLat, ShootAt: parseShootAt(req.ShootAt),
	}
	if err := s.enqueueAIItemJob(ctx, &draft, payload); err != nil {
		logger.L.Warn("AI 任务入队失败", zap.String("job_id", payload.JobID), zap.Error(err))
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
		var row aimodel.AIItemJob
		if err := s.db.Where("id = ? AND user_id = ?", id, inspectorID).First(&row).Error; err != nil {
			jobs = append(jobs, gin.H{"job_id": id, "status": "failed", "reason": "任务不存在或无权查看"})
			continue
		}
		jobs = append(jobs, gin.H{
			"job_id": id, "status": row.Status, "verdict": row.Verdict, "reason": row.Reason,
			"reading": row.Reading, "abnormal_tags": row.AbnormalTags, "quality_pass": row.QualityPass,
			"quality_issue": row.QualityIssue,
		})
	}
	return gin.H{"jobs": jobs}, nil
}

func (s *CheckinService) processAIItemJob(ctx context.Context, p aiItemJobPayload) error {
	writeDraft := func(fields map[string]any) error {
		fields["updated_at"] = time.Now()
		return draftIdentityWhere(s.db.Model(&insmodel.CheckinItemDraft{}), p.TemplateID, p.ItemCode, p.EquipmentID).
			Where("task_id = ? AND point_id = ? AND job_id = ? AND request_id = ? AND retake_seq = ? AND photo_set_hash = ?",
				p.TaskID, p.PointID, p.JobID, p.RequestID, p.RetakeSeq, p.PhotoSetHash).Updates(fields).Error
	}
	fail := func(msg string) error {
		if err := s.db.Model(&jobProjection{}).Where("id = ? AND tenant_id = ?", p.JobID, *p.TenantID).
			Updates(map[string]any{"status": "failed", "reason": strutil.Truncate(msg, 200), "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return writeDraft(map[string]any{"ai_status": insmodel.ItemDraftFailed, "ai_reason": strutil.Truncate(msg, 200)})
	}
	refs := make([]ai.PhotoRef, 0, len(p.FileIDs))
	for _, ref := range p.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.TenantID == nil || p.TenantID == nil || *f.TenantID != *p.TenantID || f.UserID != p.UserID {
			return fail("识别照片不存在、失效或不属于当前租户和用户")
		}
		refs = append(refs, ai.PhotoRef{URL: f.URL})
	}
	instanceKey := insmodel.ItemInstanceKey(strutil.StrVal(p.TemplateID), p.ItemCode, p.EquipmentID)
	input := ai.ReviewInput{
		PointName: p.PointName, PointType: p.PointType, CheckItems: []string{p.Name},
		ItemPhotos: []ai.ItemPhoto{{
			InstanceKey: instanceKey, Name: p.Name, Standard: p.Standard, AIHint: p.AIHint,
			AIPromptKind: p.AIPromptKind, ValueType: p.ValueType, Unit: p.Unit,
			ValueMin: p.ValueMin, ValueMax: p.ValueMax, EnumOptions: p.EnumOptions, Tags: p.Tags, Photos: refs,
		}},
	}
	res, err := s.reviewItemWithRetry(ctx, p.JobID, input)
	if err != nil {
		logger.L.Warn("逐项 AI 识别调用失败", zap.String("job_id", p.JobID), zap.Error(err))
		return fail("AI 识别失败：" + err.Error())
	}
	verdict, reason, reading := res.Verdict, res.Reason, ""
	var abnTags []string
	for _, iv := range res.Items {
		if iv.InstanceKey == instanceKey {
			verdict, reason, reading = iv.Verdict, iv.Reason, iv.Reading
			abnTags = iv.AbnormalTags
			break
		}
	}
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
	if len(abnTags) > 0 && verdict == ai.VerdictPass {
		verdict = ai.VerdictReview
	}
	if err := s.db.Model(&jobProjection{}).Where("id = ? AND tenant_id = ?", p.JobID, *p.TenantID).Updates(map[string]any{
		"status": "done", "verdict": verdict, "reason": strutil.Truncate(reason, 500),
		"reading": strutil.Truncate(strings.TrimSpace(reading), 64), "abnormal_tags": types.StringArray(abnTags),
		"quality_pass": res.Quality.Pass, "quality_issue": strutil.Truncate(res.Quality.Issue, 255), "updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	var readingPtr *string
	if rd := strutil.Truncate(strings.TrimSpace(reading), 64); rd != "" {
		readingPtr = &rd
	}
	return writeDraft(map[string]any{
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
			"point_id": d.PointID, "template_id": d.TemplateID, "item_code": d.ItemCode, "equipment_id": d.EquipmentID,
			"draft_kind": d.DraftKind, "job_id": d.JobID, "request_id": d.RequestID, "retake_seq": d.RetakeSeq,
			"value_num": d.ValueNum, "value_date": d.ValueDate, "value_text": d.ValueText,
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

func (s *CheckinService) DeleteItemDraft(ctx context.Context, inspectorID, taskID, pointID, templateID, itemCode, equipmentID string) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	if pointID == "" || itemCode == "" {
		return nil, errs.ErrParam
	}
	var equipment *string
	if equipmentID != "" {
		equipment = &equipmentID
	}
	res := draftIdentityWhere(s.db, &templateID, itemCode, equipment).
		Where("task_id = ? AND point_id = ? AND inspector_id = ?", taskID, pointID, inspectorID).
		Delete(&insmodel.CheckinItemDraft{})
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
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	tplItem, be := s.templateItemForDraft(&task, &point, req.CheckItemRef)
	if be != nil {
		return nil, be
	}
	if tplItem.BuiltinKind != "" {
		return nil, errs.ErrParam.WithMsg("系统内置项不能保存为人工草稿")
	}
	var valueDate *time.Time
	if req.ValueDate != "" {
		parsed, err := time.ParseInLocation("2006-01-02", req.ValueDate, time.Local)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("日期格式应为 YYYY-MM-DD")
		}
		valueDate = &parsed
	}
	result, valueErr := tplItem.ResultForValue(req.Result, req.ValueNum, valueDate, strutil.StrVal(req.ValueText))
	if valueErr != nil {
		return nil, errs.ErrParam.WithMsg(valueErr.Error())
	}
	if tplItem.NoteRequired && strings.TrimSpace(req.Note) == "" {
		return nil, errs.ErrParam.WithMsg("该检查项必须填写备注")
	}
	fileIDs := make([]string, 0, len(req.FileIDs))
	for _, ref := range req.FileIDs {
		f, err := uploadfile.ByID(s.db, ref)
		if err != nil || f.UserID != inspectorID {
			return nil, errs.ErrPhotoNotUploaded
		}
		fileIDs = append(fileIDs, f.ID)
	}
	abn, tagErr := resolveAbnormalTags(req.AbnormalTags, tplItem.Tags)
	if tagErr != "" {
		return nil, errs.ErrParam.WithMsg(tagErr)
	}
	pass := result == insmodel.ItemResultNormal
	templateID := tplItem.TemplateID
	draft := insmodel.CheckinItemDraft{
		TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
		InspectorID: inspectorID, CommunityID: task.CommunityID,
		TemplateID: &templateID, ItemCode: tplItem.Code, EquipmentID: req.EquipmentID,
		ValueNum: req.ValueNum, ValueDate: valueDate, ValueText: req.ValueText,
		RequestID: uuid.NewString(), PhotoSetHash: draftPhotoHash(fileIDs), DraftKind: insmodel.DraftKindManual,
		AIStatus: insmodel.ItemDraftDone, ManualPass: &pass, ManualNote: strutil.Truncate(strings.TrimSpace(req.Note), 512),
		FileIDs: types.StringArray(fileIDs), AbnormalTags: abn,
		ShootLng: req.ShootLng, ShootLat: req.ShootLat, ShootAt: parseShootAt(req.ShootAt),
	}
	var previous insmodel.CheckinItemDraft
	if err := draftIdentityWhere(s.db, &templateID, tplItem.Code, req.EquipmentID).
		Where("task_id = ? AND point_id = ?", task.ID, point.ID).First(&previous).Error; err == nil {
		draft.RetakeSeq = previous.RetakeSeq + 1
	}
	if err := upsertItemDraft(s.db, &draft); err != nil {
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
	tplItem, be := s.templateItemForDraft(&task, &point, req.CheckItemRef)
	if be != nil {
		return nil, be
	}
	if tplItem.BuiltinKind != "" {
		return nil, errs.ErrParam.WithMsg("系统内置项请使用对应异常入口")
	}
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
	templateID := tplItem.TemplateID
	draft := insmodel.CheckinItemDraft{
		TenantID: task.TenantID, TaskID: task.ID, PointID: req.PointID,
		InspectorID: inspectorID, CommunityID: task.CommunityID,
		TemplateID: &templateID, ItemCode: tplItem.Code, EquipmentID: req.EquipmentID,
		RequestID: uuid.NewString(), PhotoSetHash: draftPhotoHash(fileIDs),
		DraftKind: insmodel.DraftKindEscape, FileIDs: fileIDs,
		AIStatus: insmodel.ItemDraftDone, ExceptionType: req.ExceptionType, AIReason: strPtr(note),
		ShootLng: req.ShootLng, ShootLat: req.ShootLat, ShootAt: parseShootAt(req.ShootAt),
	}
	var previous insmodel.CheckinItemDraft
	if err := draftIdentityWhere(s.db, &templateID, tplItem.Code, req.EquipmentID).
		Where("task_id = ? AND point_id = ?", task.ID, point.ID).First(&previous).Error; err == nil {
		draft.RetakeSeq = previous.RetakeSeq + 1
	}
	if err := upsertItemDraft(s.db, &draft); err != nil {
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

func validItemExceptionType(exceptionType string) bool {
	// label_missing 仅标签抽查合成项可用（模板项逃生校验在 SavePhotoItemAbnormalDraft 拦截）
	return exceptionType == "device_missing" || exceptionType == "unable_to_capture" ||
		exceptionType == "camera_broken" || exceptionType == "label_missing" || exceptionType == "ai_failed"
}
