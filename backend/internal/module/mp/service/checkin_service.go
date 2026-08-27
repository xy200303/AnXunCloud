// Package service 小程序端打卡核心逻辑（6 步校验、幂等、疑似作弊判定）。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	communitysvc "anxuncloud/internal/module/community/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/geo"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/watermark"

	"go.uber.org/zap"
)

// CheckinService 打卡服务。
type CheckinService struct {
	db       *gorm.DB
	rdb      *redis.Client
	store    *storage.Storage
	getCfg   func(key string) (string, bool)
	aiCli    *ai.Client
	fontOn   bool
	notifier *notify.Notifier
}

func NewCheckinService(db *gorm.DB, rdb *redis.Client, store *storage.Storage, getCfg func(string) (string, bool), notifier *notify.Notifier) *CheckinService {
	return &CheckinService{
		db: db, rdb: rdb, store: store, getCfg: getCfg, notifier: notifier,
		// 租户级挂点预留（P3 设计方案 §9.2）：大模型配置的租户级覆盖（tenant_config 预留 ai.* key）
		// 后续改为 ConfigService.Resolve(tenantID, ...) 取值，本期统一用平台默认，行为不变。
		aiCli: ai.NewClient(getCfg, ai.WithStorage(store)),
	}
}

// cfgInt 读取整数型系统参数。
func (s *CheckinService) cfgInt(key string, def int) int {
	if s.getCfg != nil {
		if v, ok := s.getCfg(key); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return def
}

func (s *CheckinService) cfgFloat(key string, def float64) float64 {
	if s.getCfg != nil {
		if v, ok := s.getCfg(key); ok {
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil && f > 0 {
				return f
			}
		}
	}
	return def
}

// cfgBool 读取布尔型系统参数。
func (s *CheckinService) cfgBool(key string, def bool) bool {
	if s.getCfg != nil {
		if v, ok := s.getCfg(key); ok {
			return v == "true"
		}
	}
	return def
}

// AIEnabled 逐项 AI 识别能力是否可用（ai.enabled+api_key 且 sync_enabled 开启）；TaskDetail 透出用。
func (s *CheckinService) AIEnabled() bool {
	return s.aiCli.Enabled() && s.cfgBool("ai.sync_enabled", false)
}

// AIResultEditable 打卡结果是否允许覆盖修改（ai.result_editable，默认 true）；TaskDetail 透出用。
func (s *CheckinService) AIResultEditable() bool {
	return s.cfgBool("ai.result_editable", true)
}

// Submit 在线打卡提交，返回打卡结果视图。
func (s *CheckinService) Submit(c *gin.Context, inspectorID string, req *dto.CheckinReq) (gin.H, *errs.Error) {
	rec, syncRes, be := s.doCheckin(c.Request.Context(), inspectorID, req, false)
	if be != nil {
		return nil, be
	}
	return s.resultView(rec, syncRes), nil
}

// OfflineSync 离线批量补传（逐条处理，单条失败不影响其他；重复提交幂等拦截）。
func (s *CheckinService) OfflineSync(c *gin.Context, inspectorID string, req *dto.OfflineSyncReq) (gin.H, *errs.Error) {
	limit := s.cfgInt("mp.offline_sync_limit", 50)
	if len(req.Items) > limit {
		return nil, errs.ErrParam.WithMsg(fmt.Sprintf("单次补传最多 %d 条", limit))
	}
	success := []gin.H{}
	failed := []gin.H{}
	for i := range req.Items {
		item := &req.Items[i]
		rec, _, be := s.doCheckin(c.Request.Context(), inspectorID, item, true)
		if be != nil {
			failed = append(failed, gin.H{"point_id": item.PointID, "code": be.Code, "message": be.Msg})
			continue
		}
		success = append(success, gin.H{"point_id": item.PointID, "checkin_id": rec.ID, "checkin_time": timefmt.T(rec.CheckinTime)})
	}
	return gin.H{"success": success, "failed": failed}, nil
}

// doCheckin 打卡主流程（offline 标记离线补传）。
// syncRes 为同步 AI 判定结果（仅在线+开关开启+非强制提交时非空），供响应视图直接渲染。
func (s *CheckinService) doCheckin(ctx context.Context, inspectorID string, req *dto.CheckinReq, offline bool) (*insmodel.CheckinRecord, *ai.ReviewResult, *errs.Error) {
	// 防并发重复提交（弱网双击/重试）
	lockKey := fmt.Sprintf("lock:checkin:%s:%s", req.TaskID, req.PointID)
	ok, err := s.rdb.SetNX(ctx, lockKey, "1", 5*time.Second).Result()
	if err != nil {
		return nil, nil, errs.ErrInternal
	}
	if !ok {
		return nil, nil, errs.ErrDuplicateCheckin
	}
	rec, syncRes, be := s.doCheckinLocked(ctx, inspectorID, req, offline)
	// 无论成败均放锁：幂等由 DB 保证（重复点位校验 + 唯一索引 + 客户端 UUIDv7 幂等），
	// 锁仅用于收窄并发窗口；成功后必须放锁，否则客户端携带同一 UUID 的幂等重试会被误拦
	s.rdb.Del(ctx, lockKey)
	return rec, syncRes, be
}

// doCheckinLocked 打卡主流程（已持有防重锁）。
func (s *CheckinService) doCheckinLocked(ctx context.Context, inspectorID string, req *dto.CheckinReq, offline bool) (*insmodel.CheckinRecord, *ai.ReviewResult, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, nil, errs.ErrTaskNotOwned
	}
	// 客户端 UUIDv7 幂等：必须先确认任务归属，再返回已有记录，避免利用已知 ID 读取他人打卡。
	if req.ID != "" {
		if be := validateClientID(req.ID); be != nil {
			return nil, nil, be
		}
		var exist insmodel.CheckinRecord
		if err := s.db.Where("id = ?", req.ID).First(&exist).Error; err == nil {
			if exist.TaskID != task.ID || exist.PointID != req.PointID || exist.InspectorID != inspectorID || exist.CommunityID != task.CommunityID {
				return nil, nil, errs.ErrTaskNotOwned
			}
			return &exist, nil, nil
		}
	}
	if task.Status == insmodel.TaskDone {
		return nil, nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	// 点位须属于该任务路线（任务点位快照优先，空回落计划名单——兼容存量任务）
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task, &plan).Contains(req.PointID) {
		return nil, nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	// 客户端时间解析
	clientTime, err := timefmt.Parse(req.ClientTime)
	if err != nil {
		return nil, nil, errs.ErrParam.WithMsg("client_time 格式应为 YYYY-MM-DD HH:mm:ss")
	}
	// ③ 打卡方式校验（码值 / 围栏距离）
	distance := geo.Haversine(req.Longitude, req.Latitude, point.Longitude, point.Latitude)
	if be := s.checkMode(req, &point, distance); be != nil {
		return nil, nil, be
	}
	// ④ 检查项模板与逐项照片：模板每项都必须有提交结果（按 name 匹配）并生成逐项快照行；
	// 必拍约束（模板 required 项/不合格项 ≥1 张）与 file_key 上传确认（43104 / 43106）在逐项内完成；
	// 返回 upload_file 索引（EXIF 判定/水印/AI 输入共用）。照片唯一归属逐项，无记录级照片。
	checkItems, uploadFiles, be := s.resolveCheckItems(req, &point, inspectorID)
	if be != nil {
		return nil, nil, be
	}
	// 异常时描述必填
	if req.Result == insmodel.ResultAbnormal && strings.TrimSpace(req.Remark) == "" {
		return nil, nil, errs.ErrParam.WithMsg("异常打卡必须填写异常描述")
	}
	// ⑤ 疑似作弊判定（距离倍数 / EXIF 拍摄时间偏差，不阻断提交）
	isSuspect, suspectReason := s.suspectCheck(&point, distance, uploadFiles)
	// 落库类型：离线补传统一记 offline
	checkinType := req.CheckinType
	if offline {
		checkinType = "offline"
	}
	now := time.Now()
	rec := insmodel.CheckinRecord{
		TenantID: task.TenantID, // 冗余列随任务快照（=所属小区租户）
		TaskID:   req.TaskID, PointID: req.PointID, InspectorID: inspectorID,
		CommunityID: task.CommunityID, CheckinTime: now, ClientTime: &clientTime,
		Longitude: &req.Longitude, Latitude: &req.Latitude, DistanceToPoint: &distance,
		CheckinType: checkinType, Result: req.Result, Remark: req.Remark,
		IsSuspect: isSuspect, SuspectReason: suspectReason,
		AuditStatus: insmodel.AuditAutoPass,
	}
	if req.ID != "" {
		rec.ID = req.ID // 客户端 UUIDv7（BeforeCreate 不覆盖已有值）
	}

	// 同步 AI 判定（质量+内容两层一次调用）：开关开启且非强制提交、非离线补传、非逐项识别确认提交时，在事务落库前执行。
	// Force=true（重拍次数用尽）跳过同步判定直接落库，转人工复核。
	// AIConfirmed=true（逐项 AI 识别确认）：采纳逐项带回的 AI 结论，不再调大模型（不触发 43107）。
	useSyncAI := s.aiCli.Enabled() && s.cfgBool("ai.sync_enabled", false) && !req.Force && !offline && !req.AIConfirmed
	if req.Force {
		rec.ForceSubmit = true
		rec.AuditStatus = insmodel.AuditPending
		rec.AIReason = "强制提交"
	}
	var syncRes *ai.ReviewResult
	if useSyncAI {
		timeout := s.cfgInt("ai.sync_timeout_seconds", 15)
		actx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		res, aiErr := s.aiCli.ReviewCheckin(actx, s.buildReviewInput(&point, checkItems, req.Remark))
		cancel()
		switch {
		case aiErr != nil:
			// 调用失败/超时：放行落库，ai_verdict=error 转人工复核；不再起异步 goroutine
			logger.L.Warn("同步 AI 判定失败，放行转人工", zap.String("point_id", req.PointID), zap.Error(aiErr))
			rec.AIVerdict = insmodel.AIVerdictError
			rec.AIReason = truncateStr(aiErr.Error(), 200)
			rec.AuditStatus = insmodel.AuditPending
		case !res.Quality.Pass:
			// 质量不达标：拒绝打卡（不落库），data 带重拍次数上限供 App 端计数
			issue := strings.TrimSpace(res.Quality.Issue)
			if issue == "" {
				issue = errs.ErrPhotoQuality.Msg
			}
			return nil, nil, errs.ErrPhotoQuality.WithMsg(issue).
				WithData(gin.H{"max_attempts": s.cfgInt("ai.max_photo_attempts", 3)})
		default:
			syncRes = res
			s.applySyncResult(&rec, checkItems, res)
		}
	}
	// 逐项 AI 识别确认提交：逐项采用识别队列带回的 AI 结论（服务端不再调大模型）
	if req.AIConfirmed {
		s.applyConfirmedAI(&rec, checkItems, req)
	}
	// 审核与识别统一：audit_status 完全由识别状态推导。未经 AI 识别（同步判定关闭/离线补传）
	// 的记录有效性未被机器确认，落库即 pending；事务后 AI 可用则异步补识别（pass/abnormal 自动放行），
	// AI 未启用则直接待人工审核。识别出确定性结论（含 abnormal）的记录一律 auto_pass。
	if !useSyncAI && !req.Force && !req.AIConfirmed {
		rec.AuditStatus = insmodel.AuditPending
		rec.AIReason = "未经 AI 识别，待复核"
	}

	overwrite := false // 覆盖修改模式：同任务同点位已有未锁定记录时置真（事务外写操作日志用）
	var supersededID string
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var currentTask insmodel.InspectionTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&currentTask, "id = ?", task.ID).Error; err != nil {
			return errs.ErrTaskNotOwned
		}
		if currentTask.InspectorID != inspectorID {
			return errs.ErrTaskNotOwned
		}
		if currentTask.Status == insmodel.TaskDone {
			return errs.ErrDuplicateCheckin.WithMsg("任务已完成")
		}
		// 同任务同点位已有打卡：取当前有效记录（被覆盖的旧记录不参与判定）。
		// 已归档锁定（locked_at 非空）→ 43109 拒绝；未锁定 → 允许覆盖修改（旧记录置 superseded_by）。
		var prev insmodel.CheckinRecord
		prevErr := tx.Where("task_id = ? AND point_id = ? AND superseded_by IS NULL", currentTask.ID, req.PointID).First(&prev).Error
		switch {
		case prevErr == nil && prev.LockedAt != nil:
			return errs.ErrCheckinLocked
		case prevErr != nil && !errors.Is(prevErr, gorm.ErrRecordNotFound):
			return prevErr
		}
		overwrite = prevErr == nil
		if err := tx.Create(&rec).Error; err != nil {
			if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate") {
				return errs.ErrDuplicateCheckin
			}
			return err
		}
		// 逐项结果快照行（v18 起独立表；与记录同事务写入）
		for i := range checkItems {
			checkItems[i].RecordID = rec.ID
		}
		if len(checkItems) > 0 {
			if err := tx.Create(&checkItems).Error; err != nil {
				return err
			}
		}
		// 最终落库：点位正式提交成功，逐项识别过程草稿同事务清除（草稿仅"进行中"有效）
		if err := tx.Where("task_id = ? AND point_id = ?", currentTask.ID, req.PointID).
			Delete(&insmodel.CheckinItemDraft{}).Error; err != nil {
			return err
		}
		if overwrite {
			// 覆盖修改：同事务内旧记录指向新记录；任务进度不重复 +1，任务状态不重推
			if err := tx.Model(&insmodel.CheckinRecord{}).Where("id = ?", prev.ID).
				Update("superseded_by", rec.ID).Error; err != nil {
				return err
			}
			supersededID = prev.ID
			return nil
		}
		// 任务进度原子推进
		updates := map[string]any{"done_points": gorm.Expr("done_points + 1")}
		if currentTask.StartedAt == nil {
			updates["started_at"] = now
		}
		newDone := currentTask.DonePoints + 1
		if newDone >= currentTask.TotalPoints {
			updates["status"] = insmodel.TaskDone
			updates["finished_at"] = now
		} else if currentTask.Status == insmodel.TaskPending || currentTask.Status == insmodel.TaskOverdue {
			updates["status"] = insmodel.TaskDoing
		}
		if result := tx.Model(&insmodel.InspectionTask{}).Where("id = ?", currentTask.ID).Updates(updates); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != 1 {
			return errs.ErrTaskNotOwned
		}
		task.DonePoints = newDone
		return nil
	})
	if err != nil {
		if be, ok2 := err.(*errs.Error); ok2 {
			return nil, nil, be
		}
		return nil, nil, errs.ErrInternal
	}
	// 覆盖修改留痕（service 层无 OperLog 中间件上下文，记运行日志：旧记录 → 新记录）
	if overwrite {
		logger.L.Info("打卡覆盖修改",
			zap.String("task_id", rec.TaskID), zap.String("point_id", rec.PointID),
			zap.String("inspector_id", inspectorID),
			zap.String("superseded_id", supersededID), zap.String("new_checkin_id", rec.ID))
	}
	// local 模式：打卡成功后异步打水印（点位/时间/坐标/姓名；按逐项照片烧录）
	if s.store.IsLocal() && s.cfgBool("inspection.watermark_enabled", true) {
		go s.applyWatermarks(&rec, &point, checkItems, inspectorID)
	}
	// 审核分支（识别即审核）：已完成识别的路径（同步判定/强制提交/逐项识别确认）状态已定型，pending 直接通知；
	// 未经识别路径：AI 可用 → 异步补识别（pass/abnormal 自动放行，存疑/失败保持 pending 并通知）；AI 未启用 → 直接通知人工审核。
	switch {
	case useSyncAI || req.Force || req.AIConfirmed:
		if rec.AuditStatus == insmodel.AuditPending {
			s.notifyAuditors(rec.ID, point.Name, rec.AIReason)
		}
	default:
		if s.aiCli.Enabled() {
			go s.aiReview(rec.ID, &point, checkItems, req.Remark)
		} else {
			s.notifyAuditors(rec.ID, point.Name, "未启用 AI 识别，转人工审核")
		}
	}
	// 任务进度缓存失效
	s.rdb.Del(ctx, "cache:task:progress:"+task.ID)
	return &rec, syncRes, nil
}

// buildReviewInput 组装大模型审核上下文（同步判定与异步补识别共用）。
// 照片全部来自逐项快照（ItemPhotos）；manual（手动确认项）不入 CheckItems/ItemPhotos。
func (s *CheckinService) buildReviewInput(point *insmodel.InspectionPoint, items []insmodel.CheckinRecordItem, remark string) ai.ReviewInput {
	names := make([]string, 0, len(items))
	for _, it := range items {
		if it.JudgeType == ai.JudgeManual {
			continue
		}
		names = append(names, it.Name)
	}
	return ai.ReviewInput{
		PointName: point.Name, PointType: point.Type, CheckItems: names, Remark: remark,
		ItemPhotos: s.itemPhotoRefs(items),
	}
}

// applySyncResult 同步 AI 结果写入待落库记录与逐项快照（随事务一并写入，替代异步回写）：
// 记录级 ai_verdict/ai_reason/ai_quality_*；逐项 verdict/reason/reading；
// 有明确异常项时记录级 ai_verdict=abnormal（AI 判异常，记录自动通过，后台按正常/异常筛选）；
// 仅 review 项/整体 review（AI 存疑 = 记录有效性未确认）翻 audit_status=pending。
func (s *CheckinService) applySyncResult(rec *insmodel.CheckinRecord, items []insmodel.CheckinRecordItem, res *ai.ReviewResult) {
	rec.AIVerdict = res.Verdict
	rec.AIReason = truncateStr(res.Reason, 500)
	qualityPass := res.Quality.Pass
	rec.AIQualityPass = &qualityPass
	rec.AIQualityIssue = truncateStr(res.Quality.Issue, 255)
	verdicts := make(map[string]ai.ItemVerdict, len(res.Items))
	for _, iv := range res.Items {
		verdicts[iv.Name] = iv
	}
	hasIssue := res.Verdict == insmodel.AIVerdictReview
	abnormalCnt := 0
	for i := range items {
		iv, ok := verdicts[items[i].Name]
		if !ok {
			continue
		}
		v, r := iv.Verdict, truncateStr(iv.Reason, 500)
		items[i].AIVerdict = &v
		items[i].AIReason = &r
		if rd := truncateStr(strings.TrimSpace(iv.Reading), 64); rd != "" {
			items[i].AIReading = &rd
		}
		if iv.Verdict == insmodel.AIVerdictAbnormal {
			abnormalCnt++
		} else if iv.Verdict == insmodel.AIVerdictReview {
			hasIssue = true // AI 存疑 = 记录有效性未确认，转人工复核
		}
	}
	// 有明确异常项：记录级 ai_verdict=abnormal（AI 判异常；不影响放行，记录自动通过）
	if abnormalCnt > 0 {
		rec.AIVerdict = insmodel.AIVerdictAbnormal
	}
	if hasIssue {
		rec.AuditStatus = insmodel.AuditPending
	}
}

// applyConfirmedAI 逐项 AI 识别确认提交（ai_confirmed=true）：逐项快照写入识别结论
// （截断规则同 applySyncResult），不再调大模型；
// 结论取值优先级：服务端 DB 过程草稿（识别时实时落库，防客户端篡改/丢失）> 客户端带回（兜底）；
// 记录级 ai_verdict 逐项汇总（任一 abnormal/review → review，全 pass → pass，皆无 → 空）；
// abnormal 是巡检成果（记录自动通过）；仅 review 项（AI 存疑，记录有效性未确认）翻 audit_status=pending。
func (s *CheckinService) applyConfirmedAI(rec *insmodel.CheckinRecord, items []insmodel.CheckinRecordItem, req *dto.CheckinReq) {
	byName := make(map[string]dto.CheckinItemReq, len(req.CheckItems))
	for _, ci := range req.CheckItems {
		byName[ci.Name] = ci
	}
	draftByName := make(map[string]insmodel.CheckinItemDraft)
	var drafts []insmodel.CheckinItemDraft
	s.db.Where("task_id = ? AND point_id = ? AND ai_status = ?", rec.TaskID, rec.PointID, insmodel.ItemDraftDone).Find(&drafts)
	for _, d := range drafts {
		draftByName[d.ItemName] = d
	}
	passCnt, issueCnt, reviewCnt, abnormalCnt := 0, 0, 0, 0
	for i := range items {
		var v, r, rd string
		if d, ok := draftByName[items[i].Name]; ok && d.AIVerdict != nil {
			v = *d.AIVerdict
			if d.AIReason != nil {
				r = *d.AIReason
			}
			if d.AIReading != nil {
				rd = *d.AIReading
			}
		} else if ci, ok := byName[items[i].Name]; ok {
			v = strings.TrimSpace(ci.AIVerdict)
			r = ci.AIReason
			rd = ci.AIReading
		}
		if v != ai.VerdictPass && v != ai.VerdictReview && v != ai.VerdictAbnormal {
			continue // 非法/空结论忽略（该项按未识别处理）
		}
		r = truncateStr(r, 500)
		items[i].AIVerdict = &v
		items[i].AIReason = &r
		if rd = truncateStr(strings.TrimSpace(rd), 64); rd != "" {
			items[i].AIReading = &rd
		}
		switch v {
		case ai.VerdictPass:
			passCnt++
		case ai.VerdictReview:
			issueCnt++
			reviewCnt++
		case ai.VerdictAbnormal:
			issueCnt++
			abnormalCnt++
		}
	}
	// 记录级逐项汇总：有明确异常项 → abnormal（AI 判异常）；否则有存疑项 → review；全 pass → pass；皆无 → 空
	switch {
	case abnormalCnt > 0:
		rec.AIVerdict = insmodel.AIVerdictAbnormal
	case issueCnt > 0:
		rec.AIVerdict = insmodel.AIVerdictReview
	case passCnt > 0:
		rec.AIVerdict = insmodel.AIVerdictPass
	}
	if reviewCnt > 0 {
		// AI 存疑 = 记录有效性未确认，转人工复核（abnormal 不翻状态：异常是巡检成果，记录自动通过）
		rec.AuditStatus = insmodel.AuditPending
	}
}

// checkMode 凭证与围栏校验：credential 决定凭证比对（qrcode 码值 / nfc 卡号 / none 免凭证），
// require_fence 决定 GPS 距离硬校验；两者独立，同时启用则都须通过。
func (s *CheckinService) checkMode(req *dto.CheckinReq, point *insmodel.InspectionPoint, distance float64) *errs.Error {
	switch point.Credential {
	case insmodel.CredentialQRCode:
		if normalizeQRCode(req.QRCodeNo) != point.QRCodeNo {
			return errs.ErrQRCodeMismatch
		}
	case insmodel.CredentialNFC:
		if !nfcMatch(req.NFCID, point.NfcID) {
			return errs.ErrQRCodeMismatch.WithMsg("NFC 校验失败：卡号与点位不匹配")
		}
	case insmodel.CredentialAny:
		// 任一：二维码或 NFC 匹配其一即通过（NFC 仅当点位录了卡号才可比）
		qrOK := point.QRCodeNo != "" && normalizeQRCode(req.QRCodeNo) == point.QRCodeNo
		nfcOK := nfcMatch(req.NFCID, point.NfcID)
		if !qrOK && !nfcOK {
			return errs.ErrQRCodeMismatch.WithMsg("凭证校验失败：请扫描点位二维码或读取 NFC 标签")
		}
	}
	if point.RequireFence && distance > float64(point.FenceRadius) {
		return errs.ErrOutOfFence.WithMsg(fmt.Sprintf("距点位 %dm，超出围栏半径 %dm", int(distance), point.FenceRadius))
	}
	return nil
}

// nfcMatch NFC 卡号比对：双侧统一大写去空白后等值（兼容存量小写录入），空串不匹配。
func nfcMatch(reqID, pointID string) bool {
	a := strings.ToUpper(strings.TrimSpace(reqID))
	b := strings.ToUpper(strings.TrimSpace(pointID))
	return a != "" && b != "" && a == b
}

// normalizeQRCode 扫码内容归一化：兼容短链接贴纸（{站点源}/p/{code}）与早期 scheme 前缀（inspection://checkin?no=XXX），返回裸编号。
func normalizeQRCode(v string) string {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "inspection://checkin?no=")
	if i := strings.LastIndex(s, "/p/"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexAny(s, "?#"); j >= 0 {
			s = s[:j]
		}
		s = strings.TrimRight(s, "/")
	}
	return s
}

// resolveCheckItems 检查项模板校验并生成逐项快照行（v18 起写 checkin_record_item）：
// 点位必须已绑定模板（v21 起强制，无模板概念已消除）；模板每项都必须有提交结果（按 name 匹配）；
// 逐项照片硬约束：不合格项（pass=false）与模板 photo_required=required 的项均须 >=1 张该项照片，
// file_key 逐一上传确认（43104/43106）；照片唯一归属逐项，无记录级照片。
// 返回逐项快照行 + upload_file 索引（EXIF 判定/AI 输入/水印共用）；
// name/requirement/ai_hint/photo_required 打卡当时从模板项复制，template_item_id 仅作可空血缘字段。
func (s *CheckinService) resolveCheckItems(req *dto.CheckinReq, point *insmodel.InspectionPoint, ownerID string) ([]insmodel.CheckinRecordItem, map[string]sysmodel.UploadFile, *errs.Error) {
	if point.TemplateID == nil || *point.TemplateID == "" {
		return nil, nil, errs.ErrParam.WithMsg("该点位未绑定检查项模板，无法打卡")
	}
	var tplCount int64
	s.db.Model(&insmodel.CheckTemplate{}).Where("id = ?", *point.TemplateID).Count(&tplCount)
	if tplCount == 0 {
		return nil, nil, errs.ErrParam.WithMsg("点位绑定的检查项模板不存在")
	}
	var tplItems []insmodel.CheckTemplateItem
	s.db.Where("template_id = ?", *point.TemplateID).Order("sort ASC").Find(&tplItems)
	tplByName := map[string]*insmodel.CheckTemplateItem{}
	got := map[string]bool{}
	for _, it := range req.CheckItems {
		got[it.Name] = true
	}
	var missing []string
	for i := range tplItems {
		ti := &tplItems[i]
		if !got[ti.Name] {
			missing = append(missing, ti.Name)
		}
		tplByName[ti.Name] = ti
	}
	if len(missing) > 0 {
		return nil, nil, errs.ErrParam.WithMsg("检查项结果缺失：" + strings.Join(missing, "、"))
	}
	items := make([]insmodel.CheckinRecordItem, 0, len(req.CheckItems))
	files := map[string]sysmodel.UploadFile{}
	for i, it := range req.CheckItems {
		ti := tplByName[it.Name]
		if ti == nil {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」不属于该点位模板")
		}
		if !it.Pass && len(it.Photos) == 0 {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + it.Name + "」不合格，须至少上传 1 张该项照片")
		}
		if ti.PhotoRequired == types.PhotoReqRequired && len(it.Photos) == 0 {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + it.Name + "」要求必拍，须至少上传 1 张该项照片")
		}
		for _, key := range it.Photos {
			if _, ok := files[key]; ok {
				continue
			}
			var f sysmodel.UploadFile
			if err := s.db.Where("file_key = ? AND user_id = ?", key, ownerID).First(&f).Error; err != nil {
				return nil, nil, errs.ErrPhotoNotUploaded
			}
			files[key] = f
		}
		items = append(items, insmodel.CheckinRecordItem{
			Name: it.Name, Pass: it.Pass, Note: it.Note,
			Photos: types.StringArray(it.Photos), PhotoRequired: ti.PhotoRequired,
			TemplateItemID: &ti.ID, Requirement: ti.Requirement, AIHint: ti.AIHint,
			JudgeType: ti.JudgeType, JudgeConfig: ti.JudgeConfig, Sort: i,
		})
	}
	return items, files, nil
}

// suspectCheck 两类疑似作弊判定（距离超倍数 / EXIF 拍摄时间偏差）。
// 不含客户端时间偏差：手机时钟不准的误报多，且打卡时间以服务端为准、改客户端时间无伪造收益；
// client_time 字段仅保留作离线补传的实际打卡时刻记录。
func (s *CheckinService) suspectCheck(point *insmodel.InspectionPoint, distance float64, files map[string]sysmodel.UploadFile) (bool, string) {
	ratio := s.cfgFloat("inspection.suspect_distance_ratio", 1.0)
	if distance > float64(point.FenceRadius)*ratio {
		return true, fmt.Sprintf("距点位 %dm，超阈值 %dm", int(distance), int(float64(point.FenceRadius)*ratio))
	}
	exifLimit := s.cfgInt("inspection.exif_deviation_seconds", 300)
	for _, f := range files {
		if f.ExifTime == nil {
			continue
		}
		dev := f.ExifTime.Sub(time.Now()).Seconds()
		if dev < 0 {
			dev = -dev
		}
		if dev > float64(exifLimit) {
			return true, fmt.Sprintf("照片拍摄时间与打卡时间偏差 %ds，超阈值 %ds", int(dev), exifLimit)
		}
	}
	return false, ""
}

// applyWatermarks local 模式按逐项照片异步打水印（点位/时间/坐标/姓名），回写 upload_file.watermarked_url。
func (s *CheckinService) applyWatermarks(rec *insmodel.CheckinRecord, point *insmodel.InspectionPoint, items []insmodel.CheckinRecordItem, inspectorID string) {
	var inspector sysmodel.SysUser
	if s.db.Select("name").First(&inspector, "id = ?", inspectorID).Error != nil {
		return
	}
	lines := []string{
		"点位：" + point.Name,
		"时间：" + timefmt.T(rec.CheckinTime),
		fmt.Sprintf("坐标：%.6f,%.6f", *rec.Longitude, *rec.Latitude),
		"巡检员：" + inspector.Name,
	}
	seen := map[string]bool{}
	for _, it := range items {
		for _, key := range it.Photos {
			if seen[key] {
				continue // 同一文件被多个项引用时只烧一次
			}
			seen[key] = true
			srcPath := s.store.LocalPath(key)
			wmKey := strings.TrimSuffix(key, ".jpg") + "_wm.jpg"
			if key == wmKey {
				wmKey = key + "_wm.jpg"
			}
			if err := watermark.DrawToFile(srcPath, s.store.LocalPath(wmKey), "", lines); err != nil {
				logger.L.Warn("水印生成失败", zap.String("key", key), zap.Error(err))
				continue
			}
			s.db.Model(&sysmodel.UploadFile{}).Where("file_key = ?", key).
				Update("watermarked_url", s.store.URL(wmKey))
		}
	}
}

// aiReview 异步补识别（goroutine 内运行，请求 ctx 已结束故用 Background）。
// 识别即审核的统一语义：走这条路径的记录落库时未经 AI 识别（audit_status=pending）；
// 识别出确定性结论（pass，或存在明确异常项 → abnormal）→ 自动放行（仅当仍 pending 且人工未介入，不覆盖人工结论）；
// review（存疑）→ 保持 pending 并通知审核人；调用失败 → ai_verdict=error 保持 pending 并通知。
// 逐项结论（模型返回时）落到 checkin_record_item.ai_verdict/ai_reason/ai_reading；
// 质量判定结果只记 ai_quality_* 字段（异步路径不参与打卡放行）。
func (s *CheckinService) aiReview(recID string, point *insmodel.InspectionPoint, items []insmodel.CheckinRecordItem, remark string) {
	defer func() {
		if r := recover(); r != nil {
			logger.L.Error("AI 补识别 panic", zap.String("rec_id", recID), zap.Any("panic", r))
		}
	}()
	res, err := s.aiCli.ReviewCheckin(context.Background(), s.buildReviewInput(point, items, remark))
	if err != nil {
		logger.L.Warn("AI 补识别调用失败", zap.String("rec_id", recID), zap.Error(err))
		s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ?", recID, insmodel.AuditPending).
			Updates(map[string]any{"ai_verdict": insmodel.AIVerdictError, "ai_reason": truncateStr(err.Error(), 200)})
		s.notifyAuditors(recID, point.Name, "AI 识别失败，转人工审核")
		return
	}
	writeItemVerdicts(s.db, recID, res.Items)
	quality := map[string]any{
		"ai_quality_pass":  res.Quality.Pass,
		"ai_quality_issue": truncateStr(res.Quality.Issue, 255),
	}
	// 有明确异常项：记录级 ai_verdict=abnormal（AI 判异常；异常是巡检成果，同样自动放行）
	hasAbnormalItem := false
	for _, iv := range res.Items {
		if iv.Verdict == insmodel.AIVerdictAbnormal {
			hasAbnormalItem = true
			break
		}
	}
	if res.Verdict == insmodel.AIVerdictPass {
		quality["ai_verdict"] = insmodel.AIVerdictPass
		if hasAbnormalItem {
			quality["ai_verdict"] = insmodel.AIVerdictAbnormal
		}
		quality["ai_reason"] = truncateStr(res.Reason, 500)
		// 识别成功 = 记录有效：自动放行（audit_step=0 确保人工未介入，不覆盖人工结论）
		quality["audit_status"] = insmodel.AuditAutoPass
		s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ? AND audit_step = 0", recID, insmodel.AuditPending).
			Updates(quality)
		return
	}
	// review（存疑）→ 记录有效性未确认，保持 pending 并通知审核角色
	quality["ai_verdict"] = insmodel.AIVerdictReview
	quality["ai_reason"] = truncateStr(res.Reason, 500)
	res2 := s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ?", recID, insmodel.AuditPending).
		Updates(quality)
	if res2.Error != nil {
		logger.L.Warn("AI 补识别回写失败", zap.String("rec_id", recID), zap.Error(res2.Error))
		return
	}
	if res2.RowsAffected > 0 {
		s.notifyAuditors(recID, point.Name, res.Reason)
	}
}

// writeItemVerdicts 逐项 AI 结论落库（按 record_id+name 匹配快照行；模型未返回逐项结论时为空不做事）。
func writeItemVerdicts(db *gorm.DB, recID string, items []ai.ItemVerdict) {
	for _, iv := range items {
		v, r := iv.Verdict, truncateStr(iv.Reason, 500)
		updates := map[string]any{"ai_verdict": v, "ai_reason": r}
		if rd := truncateStr(strings.TrimSpace(iv.Reading), 64); rd != "" {
			updates["ai_reading"] = rd
		}
		if err := db.Model(&insmodel.CheckinRecordItem{}).
			Where("record_id = ? AND name = ?", recID, iv.Name).
			Updates(updates).Error; err != nil {
			logger.L.Warn("逐项 AI 结论回写失败", zap.String("rec_id", recID), zap.String("item", iv.Name), zap.Error(err))
		}
	}
}

// itemPhotoRefs 逐项照片（file_key → 可访问 URL）+ 标准要求/AI 识别要点，供大模型逐项核对；
// 无逐项照片返回 nil（回退整组照片逻辑）。
func (s *CheckinService) itemPhotoRefs(items []insmodel.CheckinRecordItem) []ai.ItemPhoto {
	var out []ai.ItemPhoto
	for _, it := range items {
		// manual（手动确认项）不调 AI、不占照片预算（噪音/气味等照片无法判定的项，巡检员手选结果）
		if it.JudgeType == ai.JudgeManual {
			continue
		}
		// 无逐项照片的项：仅当带判定元数据（判定类型/标准要求/识别要点）时才透出，
		// 由 AI 结合记录级照片判定（result=auto 模式下模板项均无单独照片）
		if len(it.Photos) == 0 && !hasJudgeMeta(it) {
			continue
		}
		refs := make([]ai.PhotoRef, 0, len(it.Photos))
		for _, key := range it.Photos {
			refs = append(refs, ai.PhotoRef{URL: s.store.URL(key)})
		}
		out = append(out, ai.ItemPhoto{
			Name: it.Name, Requirement: strVal(it.Requirement), AIHint: strVal(it.AIHint),
			JudgeType: it.JudgeType, JudgeConfig: it.JudgeConfig, Photos: refs,
		})
	}
	return out
}

// hasJudgeMeta 判断逐项是否带判定元数据（非 general 判定类型 / 标准要求 / AI 识别要点）。
func hasJudgeMeta(it insmodel.CheckinRecordItem) bool {
	if it.JudgeType != "" && it.JudgeType != ai.JudgeGeneral {
		return true
	}
	return strVal(it.Requirement) != "" || strVal(it.AIHint) != ""
}

// notifyAuditors AI 转人工时通知当前项目审批链首环节名单。
func (s *CheckinService) notifyAuditors(recID, pointName, reason string) {
	var rec struct {
		CommunityID string
		TaskID      string
	}
	if s.db.Model(&insmodel.CheckinRecord{}).Select("community_id", "task_id").Where("id = ?", recID).First(&rec).Error != nil {
		return
	}
	var task struct {
		PatrolType string
	}
	if s.db.Model(&insmodel.InspectionTask{}).Select("patrol_type").Where("id = ?", rec.TaskID).First(&task).Error != nil {
		return
	}
	flow := communitysvc.ResolveFlow(s.db, rec.CommunityID, sysmodel.FlowCheckinReview)
	if len(flow) == 0 {
		return
	}
	slot := communitysvc.FlowStepSlot(s.db, rec.CommunityID, task.PatrolType, flow[0].Slot)
	for _, uid := range communitysvc.SlotUserIDs(s.db, rec.CommunityID, slot) {
		_ = s.notifier.Send(uid, "checkin_audit",
			"AI 审核转人工："+pointName,
			fmt.Sprintf("点位「%s」的打卡记录经大模型审核存疑，已转人工审核。理由：%s", pointName, reason),
			&recID)
	}
}

// resultView 打卡响应视图。syncRes 为同步 AI 判定结果（非空时附带质量与逐项结论摘要，App 无需轮询）。
func (s *CheckinService) resultView(rec *insmodel.CheckinRecord, syncRes *ai.ReviewResult) gin.H {
	var task insmodel.InspectionTask
	s.db.Select("total_points", "done_points", "status").First(&task, "id = ?", rec.TaskID)
	progress := 0
	if task.TotalPoints > 0 {
		progress = task.DonePoints * 100 / task.TotalPoints
	}
	out := gin.H{
		"checkin_id": rec.ID, "checkin_time": timefmt.T(rec.CheckinTime),
		"distance_to_point": int(*rec.DistanceToPoint),
		"is_suspect":        rec.IsSuspect, "suspect_reason": rec.SuspectReason,
		// AI 审核是否启用：启用时 App 提交后延迟轮询 /checkins/:id/items 拿逐项结论；未启用跳过免白等
		"ai_enabled": s.aiCli.Enabled(),
		// 照片质量重拍放行次数上限（App 端计数，达到后可 force 提交）
		"ai_max_attempts": s.cfgInt("ai.max_photo_attempts", 3),
		"task_progress": gin.H{
			"total_points": task.TotalPoints, "done_points": task.DonePoints,
			"progress": progress, "task_status": task.Status,
		},
	}
	if syncRes != nil {
		// 同步判定：质量与逐项结论摘要随响应直接下发（App 不用轮询直接渲染）
		out["ai_verdict"] = syncRes.Verdict
		out["ai_reason"] = syncRes.Reason
		out["audit_status"] = rec.AuditStatus
		out["ai_quality"] = gin.H{"pass": syncRes.Quality.Pass, "issue": syncRes.Quality.Issue}
		items := make([]gin.H, 0, len(syncRes.Items))
		for _, iv := range syncRes.Items {
			items = append(items, gin.H{
				"name": iv.Name, "verdict": iv.Verdict, "reason": iv.Reason, "reading": iv.Reading,
			})
		}
		out["ai_items"] = items
	}
	return out
}

func truncateStr(s string, n int) string {
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

// validateClientID 校验客户端打卡 ID：必须 UUIDv7 且时间戳合理（30 天前 ~ 未来 5 分钟内）。
func validateClientID(id string) *errs.Error {
	u, err := uuid.Parse(id)
	if err != nil || u.Version() != 7 {
		return errs.ErrParam.WithMsg("id 须为 UUIDv7 格式")
	}
	sec, _ := u.Time().UnixTime()
	ts := time.Unix(sec, 0)
	if ts.After(time.Now().Add(5*time.Minute)) || ts.Before(time.Now().AddDate(0, 0, -30)) {
		return errs.ErrParam.WithMsg("id 时间戳超出合理范围")
	}
	return nil
}
