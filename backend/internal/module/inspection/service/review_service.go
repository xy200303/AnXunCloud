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
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/ai"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
)

// spotcheckLimit 单次抽查抽取上限（防呆）。
const spotcheckLimit = 500

// spotcheckAILimit AI 通道单次抽取上限（逐条同步调用大模型，防失控）。
const spotcheckAILimit = 50

// ReviewService 打卡记录审核与抽查服务。
type ReviewService struct {
	db    *gorm.DB
	aiCli *ai.Client
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
	if cfg, err := config.Load(); err == nil {
		opts = append(opts, ai.WithStorage(storage.New(cfg.Upload, cfg.OSS, cfg.App.BaseURL)))
	}
	return &ReviewService{db: db, aiCli: ai.NewClient(getCfg, opts...)}
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
		"photos": r.Photos, "check_items": r.CheckItems,
		"audit_status": r.AuditStatus, "audit_by": r.AuditBy,
		"audit_at": timefmt.TP(r.AuditAt), "audit_remark": r.AuditRemark,
		"ai_verdict": r.AIVerdict, "ai_reason": r.AIReason,
	}
}

// Pass 审核通过（仅 pending 可审）。
func (s *ReviewService) Pass(c *gin.Context, id string) *errs.Error {
	r, be := s.loadPending(c, id)
	if be != nil {
		return be
	}
	now := time.Now()
	by := middleware.CurrentUserID(c)
	updates := map[string]any{
		"audit_status": model.AuditPass, "audit_by": by, "audit_at": now, "audit_remark": "",
	}
	if err := s.db.Model(&model.CheckinRecord{}).Where("id = ?", r.ID).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Reject 审核打回（仅 pending 可审），并站内通知巡检员。
func (s *ReviewService) Reject(c *gin.Context, id, reason string) *errs.Error {
	r, be := s.loadPending(c, id)
	if be != nil {
		return be
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
	if be := middleware.CheckCommunity(c, r.CommunityID); be != nil {
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
	picked, toReview, passed, failed := 0, 0, 0, 0
	for i, id := range ids {
		var r model.CheckinRecord
		if err := s.db.First(&r, "id = ?", id).Error; err != nil {
			continue
		}
		picked++
		verdict, reason, err := s.aiCli.ReviewCheckin(c.Request.Context(), s.reviewInputOf(&r))
		updates := map[string]any{"ai_reason": truncateRunes(reason, 500)}
		switch {
		case err != nil:
			updates["ai_verdict"] = model.AIVerdictError
			updates["ai_reason"] = truncateRunes(err.Error(), 200)
			failed++
		case verdict == model.AIVerdictPass:
			updates["ai_verdict"] = model.AIVerdictPass
			passed++
		default:
			updates["ai_verdict"] = model.AIVerdictReview
			updates["audit_status"] = model.AuditPending
			toReview++
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

// reviewInputOf 由打卡记录组装大模型审核上下文。
func (s *ReviewService) reviewInputOf(r *model.CheckinRecord) ai.ReviewInput {
	var point model.InspectionPoint
	s.db.Select("name", "type").First(&point, "id = ?", r.PointID)
	names := make([]string, 0, len(r.CheckItems))
	for _, it := range r.CheckItems {
		names = append(names, it.Name)
	}
	refs := make([]ai.PhotoRef, 0, len(r.Photos))
	for _, p := range r.Photos {
		refs = append(refs, ai.PhotoRef{URL: p.URL})
	}
	return ai.ReviewInput{
		PointName: point.Name, PointType: point.Type, CheckItems: names, Remark: r.Remark, Photos: refs,
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
