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
	eqsvc "anxuncloud/internal/module/equipment/service"
	eqmodel "anxuncloud/internal/module/equipment/model"
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
	"anxuncloud/internal/pkg/uploadfile"
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
	// 点位须属于任务生成时固化的路线快照。
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
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
	// 坐标可选机制：点位未录坐标或手机定位失败（0,0）时距离无意义，
	// 跳过距离类校验与疑似判定（凭证校验不受影响），distance_to_point 不落库
	reqGeoOK := req.Longitude != 0 || req.Latitude != 0
	pointGeoOK := point.Longitude != 0 || point.Latitude != 0
	distance := geo.Haversine(req.Longitude, req.Latitude, point.Longitude, point.Latitude)
	if be := s.checkMode(req, &point, distance); be != nil {
		return nil, nil, be
	}
	// ④ 检查项模板与逐项照片：模板每项都必须有提交结果（按 name 匹配）并生成逐项快照行；
	// 必拍约束（模板 required 项/不合格项 ≥1 张）与 file_id 上传确认（43104 / 43106）在逐项内完成；
	// 返回 upload_file 索引（EXIF 判定/水印/AI 输入共用）。照片唯一归属逐项，无记录级照片。
	checkItems, uploadFiles, be := s.resolveCheckItems(req, &task, &point, inspectorID)
	if be != nil {
		return nil, nil, be
	}
	// 打卡 × 维保融合：带新标签照片的有效期合成项逐项同步 AI 核验（网络调用在打卡事务前，
	// 超时/失败降级 pending 动作），可信项翻转正常；动作落库由 persistCheckinMaintenances 同事务完成
	maintActions := s.resolveCheckinMaintenances(ctx, &point, checkItems)
	// 台账有效期/标签抽查合成项判异常（逾期/报废/抽查不符/标签缺失，不可人工改判）：记录级强制异常并转人工审核
	equipmentForced, equipmentNote := false, ""
	for i := range checkItems {
		if (checkItems[i].JudgeType == ai.JudgeEquipmentValidity || checkItems[i].JudgeType == ai.JudgeEquipmentDateSpot) && !checkItems[i].Pass {
			equipmentForced = true
			if equipmentNote == "" {
				equipmentNote = checkItems[i].Note
			}
		}
	}
	// 异常时描述必填
	if req.Result == insmodel.ResultAbnormal && strings.TrimSpace(req.Remark) == "" {
		return nil, nil, errs.ErrParam.WithMsg("异常打卡必须填写异常描述")
	}
	// ⑤ 疑似作弊判定（距离倍数 / EXIF 拍摄时间偏差，不阻断提交）
	geoOK := reqGeoOK && pointGeoOK
	isSuspect, suspectReason := s.suspectCheck(&point, distance, geoOK, uploadFiles)
	// 落库类型：离线补传统一记 offline
	checkinType := req.CheckinType
	if offline {
		checkinType = "offline"
	}
	now := time.Now()
	// 定位辅助信息（海拔/精度）：>0 才落库，仅参考展示不参与校验
	var altitude, accuracy *float64
	if req.Altitude > 0 {
		altitude = &req.Altitude
	}
	if req.Accuracy > 0 {
		accuracy = &req.Accuracy
	}
	rec := insmodel.CheckinRecord{
		TenantID: task.TenantID, // 冗余列随任务快照（=所属小区租户）
		TaskID:   req.TaskID, PointID: req.PointID, InspectorID: inspectorID,
		CommunityID: task.CommunityID, CheckinTime: now, ClientTime: &clientTime,
		Longitude: &req.Longitude, Latitude: &req.Latitude, DistanceToPoint: distancePtr(distance, geoOK),
		Altitude: altitude, Accuracy: accuracy,
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
		if be := s.validateConfirmedAI(req, checkItems); be != nil {
			return nil, nil, be
		}
		s.applyConfirmedAI(&rec, checkItems, req)
	}
	// 审核与识别统一：audit_status 完全由识别状态推导。未经 AI 识别（同步判定关闭/离线补传）
	// 的记录有效性未被机器确认，落库即 pending；事务后 AI 可用则异步补识别（pass/abnormal 自动放行），
	// AI 未启用则直接待人工审核。识别出确定性结论（含 abnormal）的记录一律 auto_pass。
	if !useSyncAI && !req.Force && !req.AIConfirmed {
		rec.AuditStatus = insmodel.AuditPending
		rec.AIReason = "未经 AI 识别，待复核"
	}
	// 台账判异常最后覆盖（优先级高于 AI 结论）：记录级强制异常 + 转人工审核；描述缺省带自动备注
	if equipmentForced {
		rec.Result = insmodel.ResultAbnormal
		rec.AuditStatus = insmodel.AuditPending
		if strings.TrimSpace(rec.Remark) == "" {
			rec.Remark = equipmentNote
		}
	}
	// 上报待处理（任一异常项 disposition=report_pending）：记录强制转人工审核（必通知审核人），
	// AI 不得自动放行（异步补识别的放行分支按记录逐项排除，见 aiReview）
	reportPending := false
	for i := range checkItems {
		if checkItems[i].Disposition == insmodel.DispositionReportPending {
			reportPending = true
			break
		}
	}
	if reportPending {
		rec.AuditStatus = insmodel.AuditPending
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
		// 打卡触发的维保流水同事务落库（source=checkin；confirmed 动作同事务回写台账，幂等跳过）
		if err := s.persistCheckinMaintenances(tx, &rec, maintActions, inspectorID); err != nil {
			return err
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
	// 打卡成功后异步打水印（点位/时间/坐标/姓名；按逐项照片烧录；本地/COS 可用，OSS 不支持服务端写入自动跳过）
	if s.cfgBool("inspection.watermark_enabled", true) {
		go s.applyWatermarks(&rec, &point, checkItems, inspectorID)
	}
	// 打卡触发的 pending 维保流水：通知项目经理/租户管理员确认或安排整改（甲方口径）
	s.notifyPendingMaintenances(&rec, maintActions)
	// 审核分支（识别即审核）：已完成识别的路径（同步判定/强制提交/逐项识别确认）状态已定型，pending 直接通知；
	// 未经识别路径：AI 可用 → 异步补识别（pass/abnormal 自动放行，存疑/失败保持 pending 并通知）；AI 未启用 → 直接通知人工审核。
	switch {
	case useSyncAI || req.Force || req.AIConfirmed:
		if rec.AuditStatus == insmodel.AuditPending {
			reason := rec.AIReason
			if reportPending {
				reason = appendItemNote("存在上报待处理异常项", reason)
			}
			s.notifyAuditors(rec.ID, point.Name, reason)
		}
	default:
		if equipmentForced {
			// 台账判异常直接转人工：跳过异步补识别，避免 AI pass 把记录自动放行出审核队列
			s.notifyAuditors(rec.ID, point.Name, equipmentNote)
		} else if s.aiCli.Enabled() {
			// 异步补识别（report_pending 记录在放行分支被排除，见 aiReview）
			go s.aiReview(rec.ID, &point, checkItems, req.Remark)
		} else {
			reason := "未启用 AI 识别，转人工审核"
			if reportPending {
				reason = appendItemNote("存在上报待处理异常项", reason)
			}
			s.notifyAuditors(rec.ID, point.Name, reason)
		}
	}
	// 任务进度缓存失效
	s.rdb.Del(ctx, "cache:task:progress:"+task.ID)
	return &rec, syncRes, nil
}

// buildReviewInput 组装大模型审核上下文（同步判定与异步补识别共用）。
// 照片全部来自逐项快照（ItemPhotos）；manual（手动确认项）与 equipment_validity（台账有效期，
// 服务端判定、无照片）不入 CheckItems/ItemPhotos。
func (s *CheckinService) buildReviewInput(point *insmodel.InspectionPoint, items []insmodel.CheckinRecordItem, remark string) ai.ReviewInput {
	names := make([]string, 0, len(items))
	for _, it := range items {
		if it.JudgeType == ai.JudgeManual || it.JudgeType == ai.JudgeEquipmentValidity || it.JudgeType == ai.JudgeEquipmentDateSpot {
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
			items[i].ExceptionType = d.ExceptionType
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

// validateConfirmedAI 防止客户端伪造 ai_confirmed 结论：每个拍照项必须存在服务端已完成草稿，
// 且草稿照片与本次提交照片逐项一致、结论为受支持的 AI 值。异常逃生草稿同样以服务端记录为准。
func (s *CheckinService) validateConfirmedAI(req *dto.CheckinReq, items []insmodel.CheckinRecordItem) *errs.Error {
	var drafts []insmodel.CheckinItemDraft
	if err := s.db.Where("task_id = ? AND point_id = ? AND ai_status = ?", req.TaskID, req.PointID, insmodel.ItemDraftDone).Find(&drafts).Error; err != nil {
		return errs.ErrInternal
	}
	draftByName := make(map[string]insmodel.CheckinItemDraft, len(drafts))
	for _, draft := range drafts {
		draftByName[draft.ItemName] = draft
	}
	for _, item := range items {
		// 手动确认项/台账有效期项/标签抽查项不走逐项 AI 识别结论校验（抽查由服务端四规则比对）
		if item.JudgeType == ai.JudgeManual || item.JudgeType == ai.JudgeEquipmentValidity || item.JudgeType == ai.JudgeEquipmentDateSpot {
			continue
		}
		draft, ok := draftByName[item.Name]
		if !ok || draft.AIVerdict == nil {
			return errs.ErrParam.WithMsg("检查项「" + item.Name + "」尚未完成 AI 识别")
		}
		if len(draft.FileIDs) != len(item.Photos) {
			return errs.ErrParam.WithMsg("检查项「" + item.Name + "」照片与识别结果不一致，请重新识别")
		}
		for i := range draft.FileIDs {
			if draft.FileIDs[i] != item.Photos[i] {
				return errs.ErrParam.WithMsg("检查项「" + item.Name + "」照片与识别结果不一致，请重新识别")
			}
		}
		verdict := *draft.AIVerdict
		if verdict != ai.VerdictPass && verdict != ai.VerdictReview && verdict != ai.VerdictAbnormal {
			return errs.ErrParam.WithMsg("检查项「" + item.Name + "」识别结论无效，请重新识别")
		}
	}
	return nil
}

// checkMode 凭证与围栏校验：credential 决定凭证比对（qrcode 码值 / nfc 卡号 / none 免凭证），
// require_fence 决定 GPS 距离硬校验；两者独立，同时启用则都须通过。
func (s *CheckinService) checkMode(req *dto.CheckinReq, point *insmodel.InspectionPoint, distance float64) *errs.Error {
	switch point.Credential {
	case insmodel.CredentialQRCode:
		if strings.TrimSpace(req.QRCodeNo) != point.QRCodeNo {
			return errs.ErrQRCodeMismatch
		}
	case insmodel.CredentialNFC:
		if !nfcMatch(req.NFCID, point.NfcID) {
			return errs.ErrQRCodeMismatch.WithMsg("NFC 校验失败：卡号与点位不匹配")
		}
	case insmodel.CredentialAny:
		// 任一：二维码或 NFC 匹配其一即通过（NFC 仅当点位录了卡号才可比）
		qrOK := point.QRCodeNo != "" && strings.TrimSpace(req.QRCodeNo) == point.QRCodeNo
		nfcOK := nfcMatch(req.NFCID, point.NfcID)
		if !qrOK && !nfcOK {
			return errs.ErrQRCodeMismatch.WithMsg("凭证校验失败：请扫描点位二维码或读取 NFC 标签")
		}
	}
	if !point.RequireFence {
		return nil
	}
	if point.Longitude == 0 && point.Latitude == 0 {
		// 点位未录坐标：围栏补录前不生效，跳过（允许先开围栏后由 App 现场补录坐标）
		return nil
	}
	if req.Longitude == 0 && req.Latitude == 0 {
		return errs.ErrOutOfFence.WithMsg("未获取到手机定位，该点位要求围栏校验，请开启定位后重试")
	}
	if distance > float64(point.FenceRadius) {
		return errs.ErrOutOfFence.WithMsg(fmt.Sprintf("距点位 %dm，超出围栏半径 %dm", int(distance), point.FenceRadius))
	}
	return nil
}

// distancePtr 距离落库：双方坐标任一缺失时距离无意义，存 NULL。
func distancePtr(distance float64, geoOK bool) *float64 {
	if !geoOK {
		return nil
	}
	return &distance
}

// nfcMatch NFC 卡号按统一入库格式精确比对，空串不匹配。
func nfcMatch(reqID, pointID string) bool {
	a := strings.TrimSpace(reqID)
	b := strings.TrimSpace(pointID)
	return a != "" && b != "" && a == b
}

// resolveCheckItems 检查项模板校验并生成逐项快照行（v18 起写 checkin_record_item）：
// 点位必须已绑定模板（v21 起强制，无模板概念已消除）；模板每项都必须有提交结果（按 name 匹配）；
// 逐项照片硬约束（一项一图）：每项最多 1 张；不合格项（pass=false）与模板 photo_required=required 的项须恰好 1 张，
// file_id 逐一上传确认（43104/43106）；照片唯一归属逐项，无记录级照片。
// 台账有效期合成项客户端可仅上送 name+photos（≤3 张新标签照片，归属校验；pass 忽略仍以服务端判定为准，
// 提交时按 name 对应服务端合成项触发维保核验）；标签抽查合成项触发即必交
// （必拍 1 张 + 生产日期/维修日期，服务端按四规则与台账比对，客户端 pass 被忽略）。
// 异常项可携带处置方式（disposition）与处置照片（resolution_file_ids，on_site_resolved 必传 ≥1 张）。
// 返回逐项快照行 + upload_file 索引（EXIF 判定/AI 输入/水印共用）；
// name/requirement/ai_hint/photo_required 打卡当时从模板项复制。
func (s *CheckinService) resolveCheckItems(req *dto.CheckinReq, task *insmodel.InspectionTask, point *insmodel.InspectionPoint, ownerID string) ([]insmodel.CheckinRecordItem, map[string]sysmodel.UploadFile, *errs.Error) {
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
	// 合成项预期（与任务详情注入同口径重算）：有效期项客户端忽略；抽查项触发即必交
	_, spotMap, be := s.equipmentSynthetics(task, point)
	if be != nil {
		return nil, nil, be
	}
	items := make([]insmodel.CheckinRecordItem, 0, len(req.CheckItems))
	files := map[string]sysmodel.UploadFile{}
	seenItems := make(map[string]bool, len(req.CheckItems))
	spotSubmitted := map[string]bool{}
	validityPhotos := map[string][]string{} // 有效期合成项新标签照片（name → file_ids）
	for i, it := range req.CheckItems {
		if seenItems[it.Name] {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」重复提交")
		}
		seenItems[it.Name] = true
		ti := tplByName[it.Name]
		if ti == nil {
			// 抽查合成项：服务端比对落快照；有效期合成项：允许携带新标签照片（服务端重算判定）
			if sj, ok := spotMap[it.Name]; ok {
				row, f, be := s.resolveSpotItem(it, sj, ownerID)
				if be != nil {
					return nil, nil, be
				}
				row.Sort = i
				files[f.ID] = f
				items = append(items, row)
				spotSubmitted[it.Name] = true
				continue
			}
			if strings.HasPrefix(it.Name, "设备维保·") {
				// 台账有效期合成项：仅接收 name+photos（≤3 张新标签照片，归属校验同逐项照片）；
				// pass/disposition 忽略——判定以服务端重算为准，处置方式由服务端在生成维保流水后回填
				if len(it.Photos) > 3 {
					return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」新标签照片最多 3 张")
				}
				photoIDs := make([]string, 0, len(it.Photos))
				for _, ref := range it.Photos {
					if _, ok := files[ref]; ok {
						photoIDs = append(photoIDs, ref)
						continue
					}
					f, err := uploadfile.ByID(s.db, ref)
					if err != nil || f.UserID != ownerID {
						return nil, nil, errs.ErrPhotoNotUploaded
					}
					files[f.ID] = f
					photoIDs = append(photoIDs, f.ID)
				}
				if len(photoIDs) > 0 {
					validityPhotos[it.Name] = photoIDs
				}
				continue
			}
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」不属于该点位模板")
		}
		// 台账有效期项：判定由服务端实时给出（不调 AI 不要求照片，模板 photo_required 配置忽略）
		isEqValidity := ti.JudgeType == ai.JudgeEquipmentValidity
		if len(it.Photos) > 1 {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」照片超出上限：一项一图，最多 1 张")
		}
		if !isEqValidity && !it.Pass && len(it.Photos) == 0 {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + it.Name + "」不合格，须至少上传 1 张该项照片")
		}
		exceptionType := strings.TrimSpace(it.ExceptionType)
		if exceptionType != "" && !validItemExceptionType(exceptionType) {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」异常类型无效")
		}
		if exceptionType != "" && it.Pass {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」已上报项目异常，结果必须为异常")
		}
		// 处置方式（disposition）白名单/照片约束校验（纯函数规则见 checkItemDisposition）
		disposition := strings.TrimSpace(it.Disposition)
		if msg := checkItemDisposition(disposition, it.Pass, len(it.ResolutionFileIDs)); msg != "" {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」" + msg)
		}
		if !isEqValidity && ti.PhotoRequired == types.PhotoReqRequired && len(it.Photos) == 0 {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + it.Name + "」要求必拍，须至少上传 1 张该项照片")
		}
		photoIDs := make([]string, 0, len(it.Photos))
		for _, ref := range it.Photos {
			if _, ok := files[ref]; ok {
				photoIDs = append(photoIDs, ref)
				continue
			}
			f, err := uploadfile.ByID(s.db, ref)
			if err != nil || f.UserID != ownerID {
				return nil, nil, errs.ErrPhotoNotUploaded
			}
			files[f.ID] = f
			photoIDs = append(photoIDs, f.ID)
		}
		// 处置照片归属校验（与逐项照片同口径）
		resIDs := make([]string, 0, len(it.ResolutionFileIDs))
		for _, ref := range it.ResolutionFileIDs {
			if _, ok := files[ref]; ok {
				resIDs = append(resIDs, ref)
				continue
			}
			f, err := uploadfile.ByID(s.db, ref)
			if err != nil || f.UserID != ownerID {
				return nil, nil, errs.ErrPhotoNotUploaded
			}
			files[f.ID] = f
			resIDs = append(resIDs, f.ID)
		}
		row := insmodel.CheckinRecordItem{
			Name: it.Name, Pass: it.Pass, Note: it.Note, ExceptionType: exceptionType,
			Photos: types.StringArray(photoIDs), PhotoRequired: ti.PhotoRequired,
			Requirement: ti.Requirement, AIHint: ti.AIHint,
			JudgeType: ti.JudgeType, JudgeConfig: ti.JudgeConfig, Sort: i,
			Disposition: disposition, ResolutionFileIDs: types.StringArray(resIDs),
			ResolutionNote: strings.TrimSpace(it.ResolutionNote),
		}
		items = append(items, row)
	}
	// 触发的抽查项必须全部提交（必拍+必填，缺失即阻断——防跳过抽查）
	for name := range spotMap {
		if !spotSubmitted[name] {
			return nil, nil, errs.ErrParam.WithMsg("标签抽查项「" + name + "」未完成：须拍标签照并填写日期")
		}
	}
	// 台账有效期合成项（§3.5 v1.6：绑定即启用、逐台独立）：服务端按点位实时逐台判定并追加快照——
	// 每台在用设备一条；逾期逐台去重（首次产异常，持续逾期只标「催办中」，pending 登记标「待确认」），
	// judge_config 携 equipment_id 快照键控；客户端携带的新标签照片按 name 挂到对应合成项快照
	if be := s.appendEquipmentItems(&items, point); be != nil {
		return nil, nil, be
	}
	if len(validityPhotos) > 0 {
		for i := range items {
			if items[i].JudgeType != ai.JudgeEquipmentValidity {
				continue
			}
			if ids, ok := validityPhotos[items[i].Name]; ok {
				items[i].Photos = types.StringArray(ids)
			}
		}
	}
	return items, files, nil
}

// checkItemDisposition 异常项处置方式校验（纯函数）：白名单 ''/on_site_resolved/maintenance_registered/
// report_pending；仅异常（!pass）项可填非空值；on_site_resolved 必须带 ≥1 张处置照片。返回错误原因（空串=通过）。
func checkItemDisposition(disposition string, pass bool, resolutionPhotos int) string {
	switch disposition {
	case "":
		return ""
	case insmodel.DispositionOnSiteResolved, insmodel.DispositionMaintenanceReg, insmodel.DispositionReportPending:
	default:
		return "处置方式无效"
	}
	if pass {
		return "正常项无须填写处置方式"
	}
	if disposition == insmodel.DispositionOnSiteResolved && resolutionPhotos == 0 {
		return "现场已处理须至少上传 1 张处置照片"
	}
	return ""
}

// appendEquipmentItems 逐台追加台账有效期合成快照项（无关联设备不追加）。
func (s *CheckinService) appendEquipmentItems(items *[]insmodel.CheckinRecordItem, point *insmodel.InspectionPoint) *errs.Error {
	byPoint, err := eqsvc.LoadPointEquipment(s.db, []string{point.ID})
	if err != nil {
		return errs.ErrInternal
	}
	eqs := byPoint[point.ID]
	if len(eqs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(eqs))
	for i := range eqs {
		ids = append(ids, eqs[i].ID)
	}
	pendingSet := eqsvc.PendingMaintenanceSet(s.db, ids)
	reportedSet := eqsvc.OverdueReportedSet(s.db, ids)
	now := time.Now()
	judges := eqsvc.JudgeDevices(eqs, pendingSet, eqsvc.GlobalWarnDays(s.db), now)
	for _, j := range judges {
		pass, note := eqsvc.DeviceJudgeSubmit(j, !reportedSet[j.EquipmentID], now)
		*items = append(*items, insmodel.CheckinRecordItem{
			Name: eqsvc.SyntheticItemName(j), Pass: pass, Note: note,
			Photos: types.StringArray{}, PhotoRequired: types.PhotoReqNone,
			JudgeType: ai.JudgeEquipmentValidity,
			// 设备快照：逐台去重与后续抽查比对的键
			JudgeConfig: types.JSONMap{"equipment_id": j.EquipmentID, "equipment_no": j.Code},
			Sort:        len(*items),
		})
	}
	return nil
}

// equipmentSynthetics 计算该点位本任务的合成项预期（与任务详情注入同口径重算）：
// 返回全部有效期项判定 + 触发的抽查项（名→判定）。抽查触发需任务上下文（哈希含 task_id/日期）。
func (s *CheckinService) equipmentSynthetics(task *insmodel.InspectionTask, point *insmodel.InspectionPoint) ([]eqsvc.DeviceJudge, map[string]eqsvc.DeviceJudge, *errs.Error) {
	spotMap := map[string]eqsvc.DeviceJudge{}
	byPoint, err := eqsvc.LoadPointEquipment(s.db, []string{point.ID})
	if err != nil {
		return nil, nil, errs.ErrInternal
	}
	eqs := byPoint[point.ID]
	if len(eqs) == 0 {
		return nil, spotMap, nil
	}
	judges := eqsvc.JudgeDevices(eqs, nil, eqsvc.GlobalWarnDays(s.db), time.Now())
	if !eqsvc.CfgBool(s.db, "equipment.spotcheck_enabled", true) {
		return judges, spotMap, nil
	}
	ids := make([]string, 0, len(eqs))
	for i := range eqs {
		ids = append(ids, eqs[i].ID)
	}
	globalRatio := eqsvc.CfgInt(s.db, "equipment.spotcheck_ratio", 10)
	salt := eqsvc.CfgString(s.db, "equipment.spotcheck_salt", "")
	rules := eqsvc.NewEquipmentService(s.db).TypeRules()
	verified := eqsvc.LastVerifiedMap(s.db, ids)
	now := time.Now()
	taskDate := task.TaskDate.Format("2006-01-02")
	for _, j := range judges {
		// 临期/逾期必触发（到期核验）；否则确定性哈希随机；长期未验证翻倍
		triggered := j.State == eqsvc.DueWarning || j.State == eqsvc.DueOverdue
		if !triggered {
			lv, ok := verified[j.EquipmentID]
			var lvPtr *time.Time
			if ok {
				lvPtr = &lv
			}
			triggered = eqsvc.SpotTriggered(task.ID, point.ID, j.EquipmentID, taskDate, salt,
				eqsvc.SpotRatioFor(rules[j.Type], globalRatio, lvPtr, now))
		}
		if triggered {
			spotMap[eqsvc.SpotItemPrefix+j.Name+"("+j.Code+")"] = j
		}
	}
	return judges, spotMap, nil
}

// resolveSpotItem 处理抽查合成项提交：必拍 1 张（归属本人），服务端按四规则与台账比对（只核对不写台账），
// 客户端 pass 被忽略；勾选「标签缺失」→ 强制异常进审核（等经理处置）。
func (s *CheckinService) resolveSpotItem(it dto.CheckinItemReq, j eqsvc.DeviceJudge, ownerID string) (insmodel.CheckinRecordItem, sysmodel.UploadFile, *errs.Error) {
	row := insmodel.CheckinRecordItem{
		Name: it.Name, PhotoRequired: types.PhotoReqRequired,
		JudgeType: ai.JudgeEquipmentDateSpot,
	}
	var file sysmodel.UploadFile
	if len(it.Photos) != 1 {
		return row, file, errs.ErrPhotoMissing.WithMsg("抽查项「" + it.Name + "」须恰好 1 张标签/设备照片")
	}
	f, err := uploadfile.ByID(s.db, it.Photos[0])
	if err != nil || f.UserID != ownerID {
		return row, file, errs.ErrPhotoNotUploaded
	}
	file = f
	// 台账侧数据
	var e eqmodel.Equipment
	if err := s.db.First(&e, "id = ?", j.EquipmentID).Error; err != nil {
		return row, file, errs.ErrNotFound.WithMsg("抽查设备不存在")
	}
	rule := eqsvc.NewEquipmentService(s.db).TypeRules()[e.Type]
	// 现场侧日期（严格 YYYY-MM-DD；标签缺失/无贴纸可空）
	var labelMfg, labelMaint *time.Time
	if v := strings.TrimSpace(it.SpotManufactureDate); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return row, file, errs.ErrParam.WithMsg("生产日期格式应为 YYYY-MM-DD")
		}
		labelMfg = &t
	}
	if v := strings.TrimSpace(it.SpotMaintenanceDate); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return row, file, errs.ErrParam.WithMsg("维修日期格式应为 YYYY-MM-DD")
		}
		labelMaint = &t
	}
	if !it.SpotLabelMissing && !it.SpotNoSticker && labelMfg == nil {
		return row, file, errs.ErrParam.WithMsg("请填写生产日期（读不到则勾选「标签缺失」）")
	}
	res := eqsvc.CompareSpot(eqsvc.SpotCompareInput{
		LedgerManufacture: e.ManufactureDate,
		LedgerLastMaint:   e.LastMaintenanceDate,
		LabelManufacture:  labelMfg,
		LabelMaint:        labelMaint,
		NoSticker:         it.SpotNoSticker,
		LabelMissing:      it.SpotLabelMissing,
		FirstMonths:       rule.FirstMonths,
		CycleMonths:       rule.CycleMonths,
		Now:               time.Now(),
	})
	row.Pass = res.Pass
	if !res.Pass {
		row.Note = strings.Join(res.Mismatches, "；")
	}
	row.Photos = types.StringArray{file.ID}
	// 快照留证：设备 + 现场提交值（比对只核对不写台账）
	row.JudgeConfig = types.JSONMap{
		"equipment_id": e.ID, "equipment_no": e.Code,
		"spot_manufacture_date": strings.TrimSpace(it.SpotManufactureDate),
		"spot_maintenance_date": strings.TrimSpace(it.SpotMaintenanceDate),
		"spot_no_sticker": it.SpotNoSticker, "spot_label_missing": it.SpotLabelMissing,
	}
	return row, file, nil
}
// 不含客户端时间偏差：手机时钟不准的误报多，且打卡时间以服务端为准、改客户端时间无伪造收益；
// client_time 字段仅保留作离线补传的实际打卡时刻记录。
func (s *CheckinService) suspectCheck(point *insmodel.InspectionPoint, distance float64, geoOK bool, files map[string]sysmodel.UploadFile) (bool, string) {
	ratio := s.cfgFloat("inspection.suspect_distance_ratio", 1.0)
	// geoOK=false（点位未录坐标或手机定位失败）：距离无意义，跳过距离项，EXIF 项照常
	if geoOK && distance > float64(point.FenceRadius)*ratio {
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

// applyWatermarks 按逐项照片异步打水印（点位/时间/坐标/姓名），回写 upload_file.watermarked_url。
// 通过存储抽象读写字节：local 读盘写盘，COS 走 HTTP 读+签名写回；OSS 不支持服务端写入，按文件 warn 跳过。
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
		for _, ref := range it.Photos {
			if seen[ref] {
				continue // 同一文件被多个项引用时只烧一次
			}
			seen[ref] = true
			f, err := uploadfile.ByID(s.db, ref)
			if err != nil {
				logger.L.Warn("水印文件记录不存在", zap.String("ref", ref), zap.Error(err))
				continue
			}
			fileKey := f.StorageKey
			wmKey := strings.TrimSuffix(fileKey, ".jpg") + "_wm.jpg"
			if fileKey == wmKey {
				wmKey = fileKey + "_wm.jpg"
			}
			data, err := s.store.ReadFile(fileKey)
			if err != nil {
				logger.L.Warn("水印读取原图失败", zap.String("key", fileKey), zap.Error(err))
				continue
			}
			out, err := watermark.DrawBytes(data, "", lines)
			if err != nil {
				logger.L.Warn("水印生成失败", zap.String("key", fileKey), zap.Error(err))
				continue
			}
			if err := s.store.PutBytes(wmKey, out); err != nil {
				logger.L.Warn("水印写回失败", zap.String("key", wmKey), zap.Error(err))
				continue
			}
			s.db.Model(&sysmodel.UploadFile{}).Where("id = ?", f.ID).
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
		// 存在「上报待处理」异常项的记录禁止自动放行（必转人工）：仍回写 AI 结论，保持 pending 并通知
		if s.hasReportPendingItem(recID) {
			res2 := s.db.Model(&insmodel.CheckinRecord{}).Where("id = ? AND audit_status = ?", recID, insmodel.AuditPending).
				Updates(quality)
			if res2.Error == nil && res2.RowsAffected > 0 {
				s.notifyAuditors(recID, point.Name, "存在上报待处理异常项，转人工审核")
			}
			return
		}
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

// itemPhotoRefs 逐项照片（file_id → 可访问 URL）+ 标准要求/AI 识别要点，供大模型逐项核对；
// 无逐项照片且无判定元数据的项不透出（无图可判）。
func (s *CheckinService) itemPhotoRefs(items []insmodel.CheckinRecordItem) []ai.ItemPhoto {
	var out []ai.ItemPhoto
	for _, it := range items {
		// manual（手动确认项）不调 AI、不占照片预算（噪音/气味等照片无法判定的项，巡检员手选结果）
		if it.JudgeType == ai.JudgeManual {
			continue
		}
		// 无逐项照片的项：仅当带判定元数据（判定类型/标准要求/识别要点）时才透出，
		// 由 AI 按文字要求判定（无图可核对时模型应判存疑）
		if len(it.Photos) == 0 && !hasJudgeMeta(it) {
			continue
		}
		refs := make([]ai.PhotoRef, 0, len(it.Photos))
		for _, ref := range it.Photos {
			if f, err := uploadfile.ByID(s.db, ref); err == nil {
				refs = append(refs, ai.PhotoRef{URL: f.URL})
			}
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

// hasReportPendingItem 记录是否存在「上报待处理」异常项（存在则禁止 AI 自动放行，必转人工审核）。
func (s *CheckinService) hasReportPendingItem(recID string) bool {
	var cnt int64
	s.db.Model(&insmodel.CheckinRecordItem{}).
		Where("record_id = ? AND disposition = ?", recID, insmodel.DispositionReportPending).Count(&cnt)
	return cnt > 0
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
	var distView any // 坐标缺失时 distance_to_point 为 NULL，响应给 null 而非 0（0 是合法距离）
	if rec.DistanceToPoint != nil {
		distView = int(*rec.DistanceToPoint)
	}
	out := gin.H{
		"checkin_id": rec.ID, "checkin_time": timefmt.T(rec.CheckinTime),
		"distance_to_point": distView,
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
