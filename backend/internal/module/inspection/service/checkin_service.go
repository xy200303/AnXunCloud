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
	eqmodel "anxuncloud/internal/module/equipment/model"
	eqsvc "anxuncloud/internal/module/equipment/service"
	filesvc "anxuncloud/internal/module/file/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/geo"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"
	queuepkg "anxuncloud/internal/pkg/queue"
	"anxuncloud/internal/pkg/safe"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/strutil"
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
	queue    *queuepkg.Queue
}

func (s *CheckinService) BindQueue(queue *queuepkg.Queue) { s.queue = queue }

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

// doCheckinLocked 打卡主流程（已持有防重锁）：定位校验 → 检查项解析 → AI 判定 → 审批链路由 → 事务落库 → 后置动作。
func (s *CheckinService) doCheckinLocked(ctx context.Context, inspectorID string, req *dto.CheckinReq, offline bool) (*insmodel.CheckinRecord, *ai.ReviewResult, *errs.Error) {
	task, point, clientTime, exist, be := s.loadCheckinTarget(req, inspectorID)
	if be != nil {
		return nil, nil, be
	}
	if exist != nil {
		return exist, nil, nil
	}
	if be := RequirePointExecutable(s.db, point); be != nil {
		communitysvc.NotifyBlockedFlowSteps(s.db, s.notifier, task.TenantID, "点位配置", []string{point.Name}, &point.ID)
		return nil, nil, be
	}
	// ③ 打卡方式校验（码值 / 围栏距离）
	// 坐标可选机制：点位未录坐标或手机定位失败（0,0）时距离无意义，
	// 跳过距离类校验与疑似判定（凭证校验不受影响），distance_to_point 不落库
	reqGeoOK := req.Longitude != 0 || req.Latitude != 0
	pointGeoOK := point.Longitude != 0 || point.Latitude != 0
	distance := geo.Haversine(req.Longitude, req.Latitude, point.Longitude, point.Latitude)
	if be := s.checkMode(req, point, distance); be != nil {
		return nil, nil, be
	}
	// ④ 检查项模板与逐项照片：模板每项都必须有提交结果（按 name 匹配）并生成逐项快照行；
	// 必拍约束（模板 required 项/不合格项 ≥1 张）与 file_id 上传确认（43104 / 43106）在逐项内完成；
	// 返回 upload_file 索引（EXIF 判定/水印/AI 输入共用）。照片唯一归属逐项，无记录级照片。
	checkItems, uploadFiles, be := s.resolveCheckItems(req, task, point, inspectorID)
	if be != nil {
		return nil, nil, be
	}
	// 照片时空信息带出与可疑判定（§14.3 防作弊：标记可疑不拒收）：逐项快照从过程草稿带出 shoot_*，
	// 围栏外拍摄/拍照早于凭证核验的项打 suspicious（随逐项快照同事务落库）
	s.applyShootMeta(point, req.TaskID, req.PointID, inspectorID, checkItems)
	// 打卡 × 维保融合：带新标签照片的有效期合成项逐项同步 AI 核验（网络调用在打卡事务前，
	// 超时/失败降级 pending 动作），可信项翻转正常；动作落库由 persistCheckinMaintenances 同事务完成
	maintActions := s.resolveCheckinMaintenances(ctx, point, checkItems)
	equipmentForced, equipmentNote := equipmentForcedVerdict(checkItems)
	// 异常 tag 一致性：任一项带异常 tag 即记录级异常（tag 本身即异常说明，不强制要求异常描述）
	if hasAbnormalTags(checkItems) {
		req.Result = insmodel.ResultAbnormal
	}
	// 异常时描述必填
	if req.Result == insmodel.ResultAbnormal && strings.TrimSpace(req.Remark) == "" {
		return nil, nil, errs.ErrParam.WithMsg("异常打卡必须填写异常描述")
	}
	// ⑤ 疑似作弊判定（距离倍数 / EXIF 拍摄时间偏差，不阻断提交）
	geoOK := reqGeoOK && pointGeoOK
	isSuspect, suspectReason := s.suspectCheck(point, distance, geoOK, uploadFiles)
	now := time.Now()
	rec := buildCheckinRecord(req, task, inspectorID, clientTime, distance, geoOK, offline, isSuspect, suspectReason, now)

	syncRes, be := s.applyAIDecision(ctx, req, offline, point, checkItems, &rec, equipmentForced, equipmentNote)
	if be != nil {
		return nil, nil, be
	}
	// ===== 通用审批引擎路由（打卡链 checkin_review）=====
	// 空流程 = 默认通过；AI 环节 = 闸门（按结论三分支路由）；人工环节 = pending 待审。
	// 强制人工（台账判异常/强制提交）按「存疑」桶路由；异常项是巡检产出不是审核理由，AI 结论按票仓正常路由。
	walk, routeReason := s.routeCheckinReview(task, &rec, req.Force, equipmentForced, equipmentNote)

	overwrite, supersededID, err := s.persistCheckin(task, req, &rec, checkItems, maintActions, inspectorID, now)
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
		safe.Go(func() { s.applyWatermarks(&rec, point, checkItems, inspectorID) })
	}
	// 打卡触发的 pending 维保流水：通知项目经理/租户管理员确认或安排整改（甲方口径）
	s.notifyPendingMaintenances(&rec, maintActions)
	// 审核动作（按走链结果）：停在 AI 环节且无结论 → 异步闸门接力；落人工环节/兜底 → 通知名单；AI 直接打回 → 通知巡检员
	switch {
	case walk.NeedGate:
		go s.RunAIGate(rec.ID)
	case walk.Fallback || walk.NotifyIdx >= 0:
		s.notifyStepReviewers(rec.ID, point.Name, walk.Step, walk.Fallback, routeReason)
	case walk.Reject:
		_ = s.notifier.Send(inspectorID, "checkin_audit", "打卡记录被驳回",
			fmt.Sprintf("你在点位「%s」的打卡记录经 AI 审核不通过：%s。请核实后按要求补巡。", point.Name, routeReason), &rec.ID)
	}
	// 任务进度缓存失效
	s.rdb.Del(ctx, "cache:task:progress:"+task.ID)
	return &rec, syncRes, nil
}

// loadCheckinTarget 定位打卡目标：任务归属 → 客户端幂等 → 路线快照/点位/客户端时间。
// exist 非空表示幂等命中（已有记录直接返回，调用方短路）。
func (s *CheckinService) loadCheckinTarget(req *dto.CheckinReq, inspectorID string) (*insmodel.InspectionTask, *insmodel.InspectionPoint, time.Time, *insmodel.CheckinRecord, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, nil, time.Time{}, nil, errs.ErrTaskNotOwned
	}
	// 客户端 UUIDv7 幂等：必须先确认任务归属，再返回已有记录，避免利用已知 ID 读取他人打卡。
	if req.ID != "" {
		if be := validateClientID(req.ID); be != nil {
			return nil, nil, time.Time{}, nil, be
		}
		var exist insmodel.CheckinRecord
		if err := s.db.Where("id = ?", req.ID).First(&exist).Error; err == nil {
			if exist.TaskID != task.ID || exist.PointID != req.PointID || exist.InspectorID != inspectorID || exist.CommunityID != task.CommunityID {
				return nil, nil, time.Time{}, nil, errs.ErrTaskNotOwned
			}
			return nil, nil, time.Time{}, &exist, nil
		}
	}
	if task.Status == insmodel.TaskDone {
		return nil, nil, time.Time{}, nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	// 点位须属于任务生成时固化的路线快照。
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, nil, time.Time{}, nil, errs.ErrTaskNotOwned
	}
	if !insmodel.TaskPointIDs(&task).Contains(req.PointID) {
		return nil, nil, time.Time{}, nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, nil, time.Time{}, nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	// 客户端时间解析
	clientTime, err := timefmt.Parse(req.ClientTime)
	if err != nil {
		return nil, nil, time.Time{}, nil, errs.ErrParam.WithMsg("client_time 格式应为 YYYY-MM-DD HH:mm:ss")
	}
	return &task, &point, clientTime, nil, nil
}

// equipmentForcedVerdict 台账有效期/标签抽查合成项判异常（逾期/报废/抽查不符/标签缺失，不可人工改判）：
// 命中即记录级强制异常并转人工审核；note 取首个命中项的判定说明。
func equipmentForcedVerdict(items []insmodel.CheckinRecordItem) (forced bool, note string) {
	for i := range items {
		if items[i].BuiltinKind != "" && items[i].Result != insmodel.ItemResultNormal {
			return true, items[i].Note
		}
	}
	return false, ""
}

// buildCheckinRecord 组装打卡记录（落库前的纯装配，不含 AI/审批链结论）。
func buildCheckinRecord(req *dto.CheckinReq, task *insmodel.InspectionTask, inspectorID string, clientTime time.Time, distance float64, geoOK, offline, isSuspect bool, suspectReason string, now time.Time) insmodel.CheckinRecord {
	// 落库类型：离线补传统一记 offline
	checkinType := req.CheckinType
	if offline {
		checkinType = "offline"
	}
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
	return rec
}

// applyAIDecision 同步 AI 判定（质量+内容两层一次调用）：开关开启且非强制提交、非离线补传、非逐项识别确认提交时，在事务落库前执行。
// Force=true（重拍次数用尽）跳过同步判定直接落库，转人工复核。
// AIConfirmed=true（逐项 AI 识别确认）：采纳逐项带回的 AI 结论，不再调大模型（不触发 43107）。
func (s *CheckinService) applyAIDecision(ctx context.Context, req *dto.CheckinReq, offline bool, point *insmodel.InspectionPoint, checkItems []insmodel.CheckinRecordItem, rec *insmodel.CheckinRecord, equipmentForced bool, equipmentNote string) (*ai.ReviewResult, *errs.Error) {
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
		res, aiErr := s.aiCli.ReviewCheckin(actx, s.buildReviewInput(point, checkItems, req.Remark))
		cancel()
		switch {
		case aiErr != nil:
			// 调用失败/超时：放行落库，ai_verdict=error 转人工复核；不再起异步 goroutine
			logger.L.Warn("同步 AI 判定失败，放行转人工", zap.String("point_id", req.PointID), zap.Error(aiErr))
			rec.AIVerdict = insmodel.AIVerdictError
			rec.AIReason = strutil.Truncate(aiErr.Error(), 200)
			rec.AuditStatus = insmodel.AuditPending
		case !res.Quality.Pass:
			// 质量不达标：拒绝打卡（不落库），data 带重拍次数上限供 App 端计数
			issue := strings.TrimSpace(res.Quality.Issue)
			if issue == "" {
				issue = errs.ErrPhotoQuality.Msg
			}
			return nil, errs.ErrPhotoQuality.WithMsg(issue).
				WithData(gin.H{"max_attempts": s.cfgInt("ai.max_photo_attempts", 3)})
		default:
			syncRes = res
			s.applySyncResult(rec, checkItems, res)
		}
	}
	// 逐项 AI 识别确认提交：逐项采用识别队列带回的 AI 结论（服务端不再调大模型）
	if req.AIConfirmed {
		unjudged, be := s.validateConfirmedAI(req, checkItems)
		if be != nil {
			return nil, be
		}
		s.applyConfirmedAI(rec, checkItems, req)
		if unjudged {
			// 存在识别超时/失败的项（巡检员手动确认兜底）：转人工复核，不静默放行
			rec.AuditStatus = insmodel.AuditPending
			rec.AIReason = strutil.Truncate(strings.TrimSpace(rec.AIReason+" 部分检查项 AI 识别失败/超时，转人工复核"), 500)
		}
	}
	// 未经 AI 识别（同步判定关闭/离线补传）的记录打标待复核原因；最终 audit_status 由
	// 审批链路由（checkin_review）统一裁定：空流程默认通过，AI 环节由闸门接力，人工环节 pending。
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
	return syncRes, nil
}

// routeCheckinReview 通用审批引擎路由（打卡链 checkin_review）：流程快照固化，在途记录按提交时的规则审完，改流程只影响新单。
// 返回走链结果与路由原因（通知/打回备注用）。
func (s *CheckinService) routeCheckinReview(task *insmodel.InspectionTask, rec *insmodel.CheckinRecord, force, equipmentForced bool, equipmentNote string) (communitysvc.WalkResult, string) {
	flow := communitysvc.ResolveFlow(s.db, task.CommunityID, sysmodel.FlowCheckinReview)
	rec.FlowSnapshot = flow
	outcome := ""
	if rec.HasAIVerdict() {
		outcome = gateBucketOf(rec.AIVerdict)
	}
	forcedHuman := equipmentForced || force
	if forcedHuman {
		outcome = sysmodel.AIGateReview
	}
	routeReason := rec.AIReason
	switch {
	case equipmentForced:
		// 台账判异常的 note 更具体（equipmentForced 蕴含存在异常项），优先于通用异常项原因
		routeReason = equipmentNote
	case force:
		routeReason = "强制提交"
	}
	walk := communitysvc.WalkFlowWithVoters(s.db, task.CommunityID, flow, 0, outcome, forcedHuman)
	communitysvc.NotifyBlockedFlowSteps(s.db, s.notifier, task.TenantID, "打卡审核", walk.BlockedSteps, nil)
	rec.AuditStep = int16(walk.Step)
	switch {
	case walk.Finish:
		rec.AuditStatus = insmodel.AuditPending
		rec.AIReason = "审批链缺少人工审核，记录保持待处理"
	case walk.Reject:
		rec.AuditStatus = insmodel.AuditRejected
		rec.AuditRemark = "AI 审核不通过：" + routeReason
	default:
		rec.AuditStatus = insmodel.AuditPending
	}
	return walk, routeReason
}

// persistCheckin 打卡落库事务：任务行锁复核 → 覆盖修改判定 → 记录+逐项+维保流水+草稿清理同事务写入 → 进度原子推进。
// 返回 overwrite（覆盖修改模式）与 supersededID（被覆盖旧记录 ID，事务外日志用）。
func (s *CheckinService) persistCheckin(task *insmodel.InspectionTask, req *dto.CheckinReq, rec *insmodel.CheckinRecord, checkItems []insmodel.CheckinRecordItem, maintActions []checkinMaintAction, inspectorID string, now time.Time) (bool, string, error) {
	overwrite := false
	var supersededID string
	err := s.db.Transaction(func(tx *gorm.DB) error {
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
		if overwrite {
			rec.OriginalRecordID = &prev.ID
			rec.Revision = prev.Revision + 1
			rec.RoundSeq = prev.RoundSeq
		}
		if err := tx.Create(rec).Error; err != nil {
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
		if rec.TenantID == nil {
			return errs.ErrDataScope
		}
		for _, item := range checkItems {
			if err := filesvc.LinkObject(tx, *rec.TenantID, rec.CommunityID, "checkin_record", rec.ID, item.Photos); err != nil {
				return err
			}
		}
		// 打卡触发的维保流水同事务落库（source=checkin；confirmed 动作同事务回写台账，幂等跳过）
		if err := s.persistCheckinMaintenances(tx, rec, maintActions, inspectorID); err != nil {
			return err
		}
		// 最终落库：点位正式提交成功，逐项识别过程草稿与凭证核验草稿同事务清除（草稿仅"进行中"有效）
		if err := tx.Where("task_id = ? AND point_id = ?", currentTask.ID, req.PointID).
			Delete(&insmodel.CheckinItemDraft{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ? AND point_id = ?", currentTask.ID, req.PointID).
			Delete(&insmodel.CheckinPointCredDraft{}).Error; err != nil {
			return err
		}
		if overwrite {
			// 覆盖修改：同事务内旧记录指向新记录；任务进度不重复 +1，任务状态不重推
			if err := tx.Model(&insmodel.CheckinRecord{}).Where("id = ?", prev.ID).
				Update("superseded_by", rec.ID).Error; err != nil {
				return err
			}
			supersededID = prev.ID
			return SyncTaskProjection(tx, currentTask.ID)
		}
		// 任务进度原子推进
		updates := map[string]any{"done_points": gorm.Expr("done_points + 1")}
		if currentTask.StartedAt == nil {
			updates["started_at"] = now
		}
		newDone := currentTask.DonePoints + 1
		blockedIDs, err := BlockedUnfinishedPointIDs(tx, currentTask.ID, []string(insmodel.TaskPointIDs(&currentTask)))
		if err != nil {
			return err
		}
		if newDone >= currentTask.TotalPoints-len(blockedIDs) {
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
		return SyncTaskProjection(tx, currentTask.ID)
	})
	return overwrite, supersededID, err
}

func (s *CheckinService) buildReviewInput(point *insmodel.InspectionPoint, items []insmodel.CheckinRecordItem, remark string) ai.ReviewInput {
	names := make([]string, 0, len(items))
	for _, it := range items {
		if it.BuiltinKind == insmodel.BuiltinEquipmentValidity || (it.Definition != nil && !it.Definition.AIEnabled) {
			continue
		}
		names = append(names, it.Name)
	}
	return ai.ReviewInput{PointName: point.Name, PointType: point.Type, CheckItems: names, Remark: remark, ItemPhotos: s.itemPhotoRefs(items)}
}

func (s *CheckinService) applySyncResult(rec *insmodel.CheckinRecord, items []insmodel.CheckinRecordItem, res *ai.ReviewResult) {
	rec.AIVerdict = res.Verdict
	rec.AIReason = strutil.Truncate(res.Reason, 500)
	qualityPass := res.Quality.Pass
	rec.AIQualityPass = &qualityPass
	rec.AIQualityIssue = strutil.Truncate(res.Quality.Issue, 255)
	verdicts := make(map[string]ai.ItemVerdict, len(res.Items))
	for _, iv := range res.Items {
		verdicts[iv.InstanceKey] = iv
	}
	hasIssue := res.Verdict == insmodel.AIVerdictReview
	abnormalCnt := 0
	for i := range items {
		iv, ok := verdicts[items[i].InstanceKey()]
		if !ok {
			continue
		}
		v, r := iv.Verdict, strutil.Truncate(iv.Reason, 500)
		if len(iv.AbnormalTags) > 0 && items[i].Definition != nil {
			allow := make(map[string]bool, len(items[i].Definition.Tags))
			for _, t := range items[i].Definition.Tags {
				allow[t] = true
			}
			seen := make(map[string]bool, len(items[i].AbnormalTags)+len(iv.AbnormalTags))
			merged := make(types.StringArray, 0, len(items[i].AbnormalTags)+len(iv.AbnormalTags))
			for _, t := range items[i].AbnormalTags {
				if !seen[t] {
					seen[t] = true
					merged = append(merged, t)
				}
			}
			for _, t := range iv.AbnormalTags {
				if allow[t] && !seen[t] {
					seen[t] = true
					merged = append(merged, t)
				}
			}
			items[i].AbnormalTags = merged
		}
		if len(items[i].AbnormalTags) > 0 && v == ai.VerdictPass {
			v = ai.VerdictReview
		}
		items[i].AIVerdict = &v
		items[i].AIReason = &r
		if rd := strutil.Truncate(strings.TrimSpace(iv.Reading), 64); rd != "" {
			items[i].AIReading = &rd
		}
		if v == insmodel.AIVerdictAbnormal {
			abnormalCnt++
		} else if v == insmodel.AIVerdictReview {
			hasIssue = true
		}
	}
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
// 草稿按 draft_kind 分发：escape=逃生（没检成，显式 escaped，不采纳 AI 结论）；
// ai=逐项识别（done 且有结论才采纳）；manual=手动选择（结果以提交载荷为准，无 AI 结论可采纳）。
func (s *CheckinService) applyConfirmedAI(rec *insmodel.CheckinRecord, items []insmodel.CheckinRecordItem, req *dto.CheckinReq) {
	byKey := make(map[string]dto.CheckinItemReq, len(req.CheckItems))
	for _, ci := range req.CheckItems {
		byKey[insmodel.ItemInstanceKey(ci.TemplateID, ci.ItemCode, ci.EquipmentID)] = ci
	}
	draftByKey := make(map[string]insmodel.CheckinItemDraft)
	var drafts []insmodel.CheckinItemDraft
	s.db.Where("task_id = ? AND point_id = ?", rec.TaskID, rec.PointID).Find(&drafts)
	for _, d := range drafts {
		draftByKey[d.InstanceKey()] = d
	}
	passCnt, issueCnt, reviewCnt, abnormalCnt := 0, 0, 0, 0
	for i := range items {
		var v, r, rd string
		if d, ok := draftByKey[items[i].InstanceKey()]; ok {
			switch d.DraftKind {
			case insmodel.DraftKindEscape:
				items[i].Result = insmodel.ItemResultEscaped
				items[i].ExceptionType = d.ExceptionType
			case insmodel.DraftKindAI:
				if d.AIStatus == insmodel.ItemDraftDone && d.AIVerdict != nil {
					v = *d.AIVerdict
					if d.AIReason != nil {
						r = *d.AIReason
					}
					if d.AIReading != nil {
						rd = *d.AIReading
					}
					// AI 异常 tag 以服务端草稿为准与客户端提交取并集（草稿已按模板 tags 过滤）
					if len(d.AbnormalTags) > 0 {
						seen := map[string]bool{}
						merged := make(types.StringArray, 0, len(items[i].AbnormalTags)+len(d.AbnormalTags))
						for _, t := range items[i].AbnormalTags {
							if !seen[t] {
								seen[t] = true
								merged = append(merged, t)
							}
						}
						for _, t := range d.AbnormalTags {
							if !seen[t] {
								seen[t] = true
								merged = append(merged, t)
							}
						}
						items[i].AbnormalTags = merged
					}
				}
			}
		} else if ci, ok := byKey[items[i].InstanceKey()]; ok {
			v = strings.TrimSpace(ci.AIVerdict)
			r = ci.AIReason
			rd = ci.AIReading
		}
		if v != ai.VerdictPass && v != ai.VerdictReview && v != ai.VerdictAbnormal {
			continue // 非法/空结论忽略（该项按未识别处理）
		}
		r = strutil.Truncate(r, 500)
		items[i].AIVerdict = &v
		items[i].AIReason = &r
		if rd = strutil.Truncate(strings.TrimSpace(rd), 64); rd != "" {
			items[i].AIReading = &rd
		}
		if len(items[i].AbnormalTags) > 0 && items[i].Result == insmodel.ItemResultNormal {
			items[i].Result = insmodel.ItemResultAbnormal // 草稿并入的异常 tag 同样强制该项异常（与 resolveCheckItems 口径一致；escaped 不翻）
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

// validateConfirmedAI 防止客户端伪造 ai_confirmed 结论：每个拍照项必须存在服务端过程草稿（按 draft_kind 分发），
// ai 草稿须结论为受支持的 AI 值，escape 草稿须与提交的 escaped 状态/类型一致；草稿照片与本次提交逐项一致。
// AI 基础设施故障逃生：ai 草稿存在但识别超时/失败（ai_status pending/failed）的项允许无 AI 结论提交
// （巡检员手动确认），返回 unjudged=true 由调用方把记录转人工复核——故障不阻断巡检，也不静默放行。
func (s *CheckinService) validateConfirmedAI(req *dto.CheckinReq, items []insmodel.CheckinRecordItem) (bool, *errs.Error) {
	var drafts []insmodel.CheckinItemDraft
	if err := s.db.Where("task_id = ? AND point_id = ?", req.TaskID, req.PointID).Find(&drafts).Error; err != nil {
		return false, errs.ErrInternal
	}
	draftByKey := make(map[string]insmodel.CheckinItemDraft, len(drafts))
	for _, draft := range drafts {
		draftByKey[draft.InstanceKey()] = draft
	}
	unjudged := false
	for _, item := range items {
		if item.BuiltinKind == insmodel.BuiltinEquipmentValidity || (item.Definition != nil && !item.Definition.AIEnabled) ||
			(item.BuiltinKind == insmodel.BuiltinEquipmentDateSpot && item.Result == insmodel.ItemResultEscaped) {
			continue
		}
		draft, ok := draftByKey[item.InstanceKey()]
		if !ok {
			return false, errs.ErrParam.WithMsg("检查项「" + item.Name + "」尚未完成 AI 识别")
		}
		switch draft.DraftKind {
		case insmodel.DraftKindEscape:
			// 逃生草稿以服务端记录为准：提交必须同为 escaped 且逃生类型一致
			if item.Result != insmodel.ItemResultEscaped || item.ExceptionType != draft.ExceptionType {
				return false, errs.ErrParam.WithMsg("检查项「" + item.Name + "」与逃生上报状态不一致，请重新提交")
			}
		case insmodel.DraftKindAI:
			if draft.AIStatus == insmodel.ItemDraftPending {
				return false, errs.ErrConflict.WithMsg("检查项「" + item.Name + "」仍在识别中，请稍后提交")
			}
			if draft.AIStatus != insmodel.ItemDraftDone || draft.AIVerdict == nil {
				unjudged = true
			} else {
				verdict := *draft.AIVerdict
				if verdict != ai.VerdictPass && verdict != ai.VerdictReview && verdict != ai.VerdictAbnormal {
					return false, errs.ErrParam.WithMsg("检查项「" + item.Name + "」识别结论无效，请重新识别")
				}
			}
		default:
			// manual 草稿（手动档向导的拍照项）：无 AI 结论，按未识别处理（转人工复核）
			unjudged = true
		}
		if len(draft.FileIDs) > 0 {
			if len(draft.FileIDs) != len(item.Photos) {
				return false, errs.ErrParam.WithMsg("检查项「" + item.Name + "」照片与识别结果不一致，请重新识别")
			}
			for i := range draft.FileIDs {
				if draft.FileIDs[i] != item.Photos[i] {
					return false, errs.ErrParam.WithMsg("检查项「" + item.Name + "」照片与识别结果不一致，请重新识别")
				}
			}
		}
	}
	return unjudged, nil
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
// 点位检查项 = 其全部模板（point_template）检查项并集；点位必须已绑定模板（v21 起强制）；每项都必须有提交结果（按 name 匹配）；
// result 显式三态只校验不折算（escaped ⇔ exception_type 非空；带异常 tag 须 abnormal）；
// 逐项照片硬约束（一项一图）：每项最多 1 张；abnormal 项与模板 required 项须恰好 1 张；
// escaped 佐证分流：device_missing/label_missing 须 ≥1 张佐证，unable_to_capture/camera_broken 免佐证。
// file_id 逐一上传确认（43104/43106）；照片唯一归属逐项，无记录级照片。
// 台账有效期合成项客户端可仅上送 name+photos（≤3 张新标签照片，归属校验；pass 忽略仍以服务端判定为准，
// 提交时按 name 对应服务端合成项触发维保核验）；标签抽查合成项触发即必交
// （必拍 1 张 + 生产日期/维修日期，服务端按四规则与台账比对，客户端 pass 被忽略）。
func (s *CheckinService) resolveCheckItems(req *dto.CheckinReq, task *insmodel.InspectionTask, point *insmodel.InspectionPoint, ownerID string) ([]insmodel.CheckinRecordItem, map[string]sysmodel.UploadFile, *errs.Error) {
	tplSet := insmodel.LoadTaskTemplateSets(s.db, task, []string{point.ID})[point.ID]
	if tplSet == nil || len(tplSet.TemplateIDs) == 0 || len(tplSet.Items) == 0 {
		return nil, nil, errs.ErrParam.WithMsg("该任务没有适用的检查项模板，请调整计划模板选择")
	}
	tplItems := tplSet.Items
	tplByDefinition := map[string]*insmodel.UnionItem{}
	got := map[string]bool{}
	for _, it := range req.CheckItems {
		got[insmodel.ItemInstanceKey(it.TemplateID, it.ItemCode, it.EquipmentID)] = true
	}
	var missing []string
	for i := range tplItems {
		ti := &tplItems[i]
		definitionKey := insmodel.ItemInstanceKey(ti.TemplateID, ti.Code, nil)
		if !got[definitionKey] {
			missing = append(missing, ti.Name)
		}
		tplByDefinition[definitionKey] = ti
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
	validityPhotos := map[string][]string{}
	equipmentJudges, _, _ := s.equipmentSynthetics(task, point)
	validityItems := map[string]bool{}
	for _, judge := range equipmentJudges {
		equipmentID := judge.EquipmentID
		validityItems[insmodel.ItemInstanceKey("", "builtin_"+insmodel.BuiltinEquipmentValidity, &equipmentID)] = true
	}
	for i, it := range req.CheckItems {
		key := insmodel.ItemInstanceKey(it.TemplateID, it.ItemCode, it.EquipmentID)
		if seenItems[key] {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.ItemCode + "」重复提交")
		}
		seenItems[key] = true
		ti := tplByDefinition[insmodel.ItemInstanceKey(it.TemplateID, it.ItemCode, nil)]
		if ti == nil {
			if sj, ok := spotMap[key]; ok {
				row, f, be := s.resolveSpotItem(task.ID, point.ID, it, sj, ownerID)
				if be != nil {
					return nil, nil, be
				}
				row.TenantID, row.Sort = task.TenantID, i
				files[f.ID] = f
				items = append(items, row)
				spotSubmitted[key] = true
				continue
			}
			if validityItems[key] {
				if len(it.Photos) > 3 {
					return nil, nil, errs.ErrParam.WithMsg("台账有效期检查项新标签照片最多 3 张")
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
					validityPhotos[key] = photoIDs
				}
				continue
			}
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.ItemCode + "」不属于该任务")
		}
		if len(it.Photos) > 1 {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」照片超出上限：一项一图，最多 1 张")
		}
		// tag 校验：提交 ⊆ 模板 tag 快照（trim/去重）；带异常 tag 的项必须标 abnormal（只校验不折算）
		abnTags, tagErr := resolveAbnormalTags(it.AbnormalTags, ti.Tags)
		if tagErr != "" {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」" + tagErr)
		}
		if len(abnTags) > 0 && it.Result != insmodel.ItemResultAbnormal {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」勾选了异常观察点，结果必须为异常")
		}
		exceptionType := strings.TrimSpace(it.ExceptionType)
		if exceptionType != "" && (!validItemExceptionType(exceptionType) || exceptionType == "label_missing") {
			// label_missing 仅标签抽查合成项可用（resolveSpotItem 单独处理）
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + it.Name + "」异常类型无效")
		}
		var valueDate *time.Time
		if it.ValueDate != "" {
			parsed, err := time.ParseInLocation("2006-01-02", it.ValueDate, time.Local)
			if err != nil {
				return nil, nil, errs.ErrParam.WithMsg("检查项「" + ti.Name + "」日期格式应为 YYYY-MM-DD")
			}
			valueDate = &parsed
		}
		result, valueErr := ti.ResultForValue(it.Result, it.ValueNum, valueDate, strutil.StrVal(it.ValueText))
		if valueErr != nil {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + ti.Name + "」：" + valueErr.Error())
		}
		if (result == insmodel.ItemResultEscaped) != (exceptionType != "") {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + ti.Name + "」结果与无法检查类型不一致")
		}
		if ti.NoteRequired && strings.TrimSpace(it.Note) == "" {
			return nil, nil, errs.ErrParam.WithMsg("检查项「" + ti.Name + "」必须填写备注")
		}
		// 照片约束：abnormal 项须 ≥1 张该项照片；escaped 佐证分流——device_missing/label_missing 须 ≥1 张，
		// unable_to_capture/camera_broken 免佐证（无法拍摄还要照片是矛盾的）
		if result == insmodel.ItemResultAbnormal && len(it.Photos) == 0 {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + ti.Name + "」异常，须至少上传 1 张该项照片")
		}
		if result == insmodel.ItemResultEscaped && !escapePhotoExempt(exceptionType) && len(it.Photos) == 0 {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + ti.Name + "」无法检查，须至少上传 1 张佐证照片")
		}
		if ti.PhotoRequired == types.PhotoReqRequired && len(it.Photos) == 0 &&
			!(result == insmodel.ItemResultEscaped && escapePhotoExempt(exceptionType)) {
			return nil, nil, errs.ErrPhotoMissing.WithMsg("检查项「" + ti.Name + "」要求必拍，须至少上传 1 张该项照片")
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
		templateID := ti.TemplateID
		row := insmodel.CheckinRecordItem{
			TenantID: task.TenantID, TemplateID: &templateID, ItemCode: ti.Code, EquipmentID: it.EquipmentID,
			Name: ti.Name, Standard: ti.Standard, Definition: &ti.CheckTemplateItem,
			Result: result, ValueNum: it.ValueNum, ValueDate: valueDate, ValueText: it.ValueText,
			Note: strings.TrimSpace(it.Note), ExceptionType: exceptionType,
			Photos: types.StringArray(photoIDs), AbnormalTags: abnTags, Sort: i,
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
	// equipment_id 标识逐台实例；新标签照片按实例键关联，不再按项名匹配
	if be := s.appendEquipmentItems(&items, point); be != nil {
		return nil, nil, be
	}
	if len(validityPhotos) > 0 {
		for i := range items {
			if ids, ok := validityPhotos[items[i].InstanceKey()]; ok {
				items[i].Photos = types.StringArray(ids)
			}
		}
	}
	return items, files, nil
}

// resolveAbnormalTags 校验提交的异常 tag 列表 ⊆ 模板 tag 快照（trim/去重）。返回归一后的列表与错误原因（空串=通过）。
func resolveAbnormalTags(submitted []string, tags types.StringArray) (types.StringArray, string) {
	if len(submitted) == 0 {
		return types.StringArray{}, ""
	}
	allowed := make(map[string]bool, len(tags))
	for _, t := range tags {
		allowed[t] = true
	}
	out := make(types.StringArray, 0, len(submitted))
	seen := map[string]bool{}
	for _, t := range submitted {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		if !allowed[t] {
			return nil, "tag「" + t + "」不属于该检查项"
		}
		seen[t] = true
		out = append(out, t)
	}
	return out, ""
}

// escapePhotoExempt 逃生类型免佐证照片（无法拍摄/相机故障还要照片是矛盾的）；
// device_missing（现场佐证）与 label_missing（最后一张标签照）仍须照片。
func escapePhotoExempt(exceptionType string) bool {
	return exceptionType == "unable_to_capture" || exceptionType == "camera_broken"
}

// hasAbnormalTags 任一项存在异常 tag（记录级结果一致性：强制打卡结果为异常）。
func hasAbnormalTags(items []insmodel.CheckinRecordItem) bool {
	for _, it := range items {
		if len(it.AbnormalTags) > 0 {
			return true
		}
	}
	return false
}

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
		equipmentID := j.EquipmentID
		*items = append(*items, insmodel.CheckinRecordItem{
			TenantID: point.TenantID, ItemCode: "builtin_" + insmodel.BuiltinEquipmentValidity,
			EquipmentID: &equipmentID, BuiltinKind: insmodel.BuiltinEquipmentValidity,
			Name: eqsvc.SyntheticItemName(j), Result: insmodel.ItemResultOf("", pass), Note: note,
			Photos: types.StringArray{}, Sort: len(*items),
		})
	}
	return nil
}

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
	globalRatio := eqsvc.CfgInt(s.db, "equipment.spotcheck_ratio", 2)
	salt := eqsvc.CfgString(s.db, "equipment.spotcheck_salt", "")
	rules := eqsvc.NewEquipmentService(s.db).TypeRules()
	verified := eqsvc.LastVerifiedMap(s.db, ids)
	now := time.Now()
	taskDate := task.TaskDate.Format("2006-01-02")
	for _, j := range judges {
		if j.State != eqsvc.DueNormal {
			continue
		}
		lv, ok := verified[j.EquipmentID]
		var lvPtr *time.Time
		if ok {
			lvPtr = &lv
		}
		if eqsvc.SpotTriggered(task.ID, point.ID, j.EquipmentID, taskDate, salt,
			eqsvc.SpotRatioFor(rules[j.Type], globalRatio, lvPtr, now)) {
			equipmentID := j.EquipmentID
			spotMap[insmodel.ItemInstanceKey("", "builtin_"+insmodel.BuiltinEquipmentDateSpot, &equipmentID)] = j
		}
	}
	return judges, spotMap, nil
}

func (s *CheckinService) resolveSpotItem(taskID, pointID string, it dto.CheckinItemReq, j eqsvc.DeviceJudge, ownerID string) (insmodel.CheckinRecordItem, sysmodel.UploadFile, *errs.Error) {
	equipmentID := j.EquipmentID
	row := insmodel.CheckinRecordItem{
		ItemCode: "builtin_" + insmodel.BuiltinEquipmentDateSpot, EquipmentID: &equipmentID,
		BuiltinKind: insmodel.BuiltinEquipmentDateSpot, Name: eqsvc.SpotItemPrefix + j.Name + "(" + j.Code + ")",
		Standard: "拍 1 张瓶体标签/钢印照片", Result: insmodel.ItemResultNormal,
	}
	var file sysmodel.UploadFile
	if len(it.Photos) != 1 {
		return row, file, errs.ErrPhotoMissing.WithMsg("抽查项「" + row.Name + "」须恰好 1 张标签/设备照片")
	}
	f, err := uploadfile.ByID(s.db, it.Photos[0])
	if err != nil || f.UserID != ownerID {
		return row, file, errs.ErrPhotoNotUploaded
	}
	file = f
	var e eqmodel.Equipment
	if err := s.db.First(&e, "id = ?", j.EquipmentID).Error; err != nil {
		return row, file, errs.ErrNotFound.WithMsg("抽查设备不存在")
	}
	rule := eqsvc.NewEquipmentService(s.db).TypeRules()[e.Type]
	labelMissing := strings.TrimSpace(it.ExceptionType) == "label_missing"
	var reading string
	var draft insmodel.CheckinItemDraft
	if err := s.db.Where("task_id = ? AND point_id = ? AND template_id IS NULL AND item_code = ? AND equipment_id = ? AND draft_kind = ? AND ai_status = ?",
		taskID, pointID, row.ItemCode, equipmentID, insmodel.DraftKindAI, insmodel.ItemDraftDone).First(&draft).Error; err == nil && draft.AIReading != nil {
		reading = *draft.AIReading
	}
	labelMfg, labelMaint := parseSpotReading(reading)
	unread := !labelMissing && reading == ""
	res := eqsvc.CompareSpot(eqsvc.SpotCompareInput{
		LedgerManufacture: e.ManufactureDate, LedgerLastMaint: e.LastMaintenanceDate,
		LabelManufacture: labelMfg, LabelMaint: labelMaint, NoSticker: false,
		LabelMissing: labelMissing, FirstMonths: rule.FirstMonths, CycleMonths: rule.CycleMonths, Now: time.Now(),
	})
	switch {
	case labelMissing:
		row.Result = insmodel.ItemResultEscaped
		row.ExceptionType = "label_missing"
		row.Note = strings.Join(res.Mismatches, "；")
	case unread:
		row.Result = insmodel.ItemResultAbnormal
		row.Note = "AI 未能读出标签日期，转人工核对"
	case !res.Pass:
		row.Result = insmodel.ItemResultAbnormal
		row.Note = strings.Join(res.Mismatches, "；")
	}
	row.Photos = types.StringArray{file.ID}
	if reading != "" {
		row.AIReading = &reading
	}
	return row, file, nil
}

// parseSpotReading 解析 AI 读标签读数「M{生产年月}|W{维修年月}」（无为空），返回月初日期。
func parseSpotReading(reading string) (mfg, maint *time.Time) {
	parse := func(v string) *time.Time {
		v = strings.TrimSpace(v)
		if v == "" || v == "无" {
			return nil
		}
		t, err := time.ParseInLocation("2006-01", v, time.Local)
		if err != nil {
			return nil
		}
		return &t
	}
	for _, seg := range strings.Split(reading, "|") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, "M") {
			mfg = parse(strings.TrimPrefix(seg, "M"))
		}
		if strings.HasPrefix(seg, "W") {
			maint = parse(strings.TrimPrefix(seg, "W"))
		}
	}
	return mfg, maint
}

// applyShootMeta 照片时空信息带出与可疑判定（《打卡向导交互重构设计方案》§14.3：标记可疑不拒收）：
// 逐项快照从过程草稿带出 shoot_lng/lat/at（按项实例键匹配）；
// 开围栏点位照片坐标超围栏半径、或拍照时间早于该点位凭证核验时间的项打 suspicious（多原因「；」连接）。
func (s *CheckinService) applyShootMeta(point *insmodel.InspectionPoint, taskID, pointID, inspectorID string, items []insmodel.CheckinRecordItem) {
	var drafts []insmodel.CheckinItemDraft
	if err := s.db.Where("task_id = ? AND point_id = ?", taskID, pointID).Find(&drafts).Error; err != nil || len(drafts) == 0 {
		return
	}
	draftByKey := make(map[string]insmodel.CheckinItemDraft, len(drafts))
	for _, d := range drafts {
		draftByKey[d.InstanceKey()] = d
	}
	var verifiedAt *time.Time
	var cred insmodel.CheckinPointCredDraft
	if err := s.db.Where("task_id = ? AND point_id = ? AND inspector_id = ?", taskID, pointID, inspectorID).
		First(&cred).Error; err == nil {
		verifiedAt = &cred.VerifiedAt
	}
	pointGeoOK := point.Longitude != 0 || point.Latitude != 0
	for i := range items {
		d, ok := draftByKey[items[i].InstanceKey()]
		if !ok {
			continue
		}
		items[i].ShootLng = d.ShootLng
		items[i].ShootLat = d.ShootLat
		items[i].ShootAt = d.ShootAt
		var reasons []string
		// 围栏外拍摄：点位开围栏且照片带坐标（点位未录坐标时距离无意义，跳过）
		if point.RequireFence && pointGeoOK && d.ShootLng != nil && d.ShootLat != nil {
			if dist := geo.Haversine(*d.ShootLng, *d.ShootLat, point.Longitude, point.Latitude); dist > float64(point.FenceRadius) {
				reasons = append(reasons, fmt.Sprintf("拍照位置超出围栏（%dm）", int(dist)))
			}
		}
		// 先拍后签到：拍照时间早于凭证核验时间
		if verifiedAt != nil && d.ShootAt != nil && d.ShootAt.Before(*verifiedAt) {
			reasons = append(reasons, "拍照时间早于凭证核验")
		}
		if len(reasons) > 0 {
			items[i].Suspicious = true
			items[i].SuspiciousReason = strutil.Truncate(strings.Join(reasons, "；"), 255)
		}
	}
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
			data, err := s.store.ReadFile(f.StorageKey)
			if err != nil {
				logger.L.Warn("水印读取原图失败", zap.String("key", f.StorageKey), zap.Error(err))
				continue
			}
			out, err := watermark.DrawBytes(data, "", lines)
			if err != nil {
				logger.L.Warn("水印生成失败", zap.String("key", f.StorageKey), zap.Error(err))
				continue
			}
			child, err := filesvc.RegisterDerivative(s.db, s.store, &f, "watermarked", "_wm.jpg", out, "image/jpeg", types.JSONMap{"lines": lines})
			if err != nil {
				logger.L.Warn("水印副本登记失败", zap.String("key", f.StorageKey), zap.Error(err))
				continue
			}
			s.db.Model(&sysmodel.UploadFile{}).Where("id = ?", f.ID).
				Update("watermarked_url", child.URL)
		}
	}
}

func writeItemVerdicts(db *gorm.DB, recID string, items []ai.ItemVerdict) {
	WriteItemVerdicts(db, recID, items)
}

func (s *CheckinService) itemPhotoRefs(items []insmodel.CheckinRecordItem) []ai.ItemPhoto {
	LoadItemDefinitions(s.db, items)
	var out []ai.ItemPhoto
	for _, it := range items {
		if it.BuiltinKind == insmodel.BuiltinEquipmentValidity || (it.Definition != nil && !it.Definition.AIEnabled) {
			continue
		}
		if len(it.Photos) == 0 && it.Standard == "" && it.BuiltinKind == "" {
			continue
		}
		refs := make([]ai.PhotoRef, 0, len(it.Photos))
		for _, ref := range it.Photos {
			if f, err := uploadfile.ByID(s.db, ref); err == nil {
				refs = append(refs, ai.PhotoRef{URL: f.URL})
			}
		}
		photo := ItemPhoto(it, refs)
		if it.BuiltinKind == insmodel.BuiltinEquipmentDateSpot {
			photo.AIPromptKind = ai.PromptLabel
		}
		out = append(out, photo)
	}
	return out
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
				"instance_key": iv.InstanceKey, "name": iv.Name, "verdict": iv.Verdict,
				"reason": iv.Reason, "reading": iv.Reading, "abnormal_tags": iv.AbnormalTags,
			})
		}
		out["ai_items"] = items
	}
	return out
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
