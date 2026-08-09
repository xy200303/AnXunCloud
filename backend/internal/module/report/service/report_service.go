// Package service 月度巡检报告业务逻辑：生成/重算 + 三级电子确认签字 + PDF 归档。
package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/report/dto"
	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	womodel "anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/pdf"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"

	"go.uber.org/zap"
)

// ReportService 月度报告服务。
type ReportService struct {
	db     *gorm.DB
	store  *storage.Storage
	getCfg func(key string) (string, bool) // 读取系统参数（report.company_name；公章自 v16 起走 sign_asset 资产表）
}

func NewReportService(db *gorm.DB, store *storage.Storage, getCfg func(string) (string, bool)) *ReportService {
	return &ReportService{db: db, store: store, getCfg: getCfg}
}

// cfgString 读取系统参数（未配置返回空串）。
func (s *ReportService) cfgString(key string) string {
	if s.getCfg == nil {
		return ""
	}
	v, _ := s.getCfg(key)
	return v
}

// periodRange 解析 YYYY-MM 期间，返回 [月初, 次月初) 时间范围。
func periodRange(period string) (time.Time, time.Time, *errs.Error) {
	start, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return start, start, errs.ErrParam.WithMsg("period 格式应为 YYYY-MM")
	}
	return start, start.AddDate(0, 1, 0), nil
}

// pct 百分比（保留 1 位小数）。
func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	v := float64(part) * 100 / float64(total)
	return float64(int(v*10+0.5)) / 10
}

// title 报告标题：{小区名}{YYYY年M月}月度巡检工作报告。
func reportTitle(commName, period string) string {
	t, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return commName + period + "月度巡检工作报告"
	}
	return fmt.Sprintf("%s%d年%d月月度巡检工作报告", commName, t.Year(), int(t.Month()))
}

// buildStats 聚合指定小区指定月度的统计数据与应确认巡检员集合。
func (s *ReportService) buildStats(communityID, period string) (types.JSONMap, types.IDArray, *errs.Error) {
	start, end, be := periodRange(period)
	if be != nil {
		return nil, nil, be
	}
	startStr, endStr := start.Format("2006-01-02"), end.Format("2006-01-02")

	// 任务汇总
	var taskSum struct {
		Total   int64
		Done    int64
		Overdue int64
		Should  int64
		DonePts int64
	}
	s.db.Model(&insmodel.InspectionTask{}).
		Where("community_id = ? AND task_date >= ? AND task_date < ?", communityID, startStr, endStr).
		Select(`COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'done') AS done,
			COUNT(*) FILTER (WHERE status = 'overdue') AS overdue,
			COALESCE(SUM(total_points),0) AS should,
			COALESCE(SUM(done_points),0) AS done_pts`).Scan(&taskSum)

	// 应确认巡检员：当月该小区有任务的巡检员去重
	var inspectorIDs []string
	s.db.Model(&insmodel.InspectionTask{}).Distinct().
		Where("community_id = ? AND task_date >= ? AND task_date < ?", communityID, startStr, endStr).
		Pluck("inspector_id", &inspectorIDs)

	// 打卡异常/疑似
	var ckSum struct {
		Abnormal int64
		Suspect  int64
	}
	s.db.Model(&insmodel.CheckinRecord{}).
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ?", communityID, start, end).
		Select("COUNT(*) FILTER (WHERE result = 'abnormal') AS abnormal, COUNT(*) FILTER (WHERE is_suspect) AS suspect").Scan(&ckSum)

	// 工单：当月新建 / 其中已关闭
	var woSum struct {
		Created int64
		Closed  int64
	}
	s.db.Model(&womodel.WorkOrder{}).
		Where("community_id = ? AND created_at >= ? AND created_at < ?", communityID, start, end).
		Select("COUNT(*) AS created, COUNT(*) FILTER (WHERE status = 'closed') AS closed").Scan(&woSum)

	// 逐日明细：任务（日期/任务/完成）+ 当日异常打卡
	type taskDay struct {
		Date  string
		Total int64
		Done  int64
	}
	var taskDays []taskDay
	s.db.Model(&insmodel.InspectionTask{}).
		Where("community_id = ? AND task_date >= ? AND task_date < ?", communityID, startStr, endStr).
		Select("to_char(task_date, 'YYYY-MM-DD') AS date, COUNT(*) AS total, COUNT(*) FILTER (WHERE status = 'done') AS done").
		Group("date").Order("date").Scan(&taskDays)
	type ckDay struct {
		Date     string
		Abnormal int64
	}
	var ckDays []ckDay
	s.db.Model(&insmodel.CheckinRecord{}).
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ? AND result = 'abnormal'", communityID, start, end).
		Select("to_char(checkin_time, 'YYYY-MM-DD') AS date, COUNT(*) AS abnormal").
		Group("date").Scan(&ckDays)
	abByDay := map[string]int64{}
	for _, d := range ckDays {
		abByDay[d.Date] = d.Abnormal
	}
	daily := make([]gin.H, 0, len(taskDays))
	for _, d := range taskDays {
		daily = append(daily, gin.H{"date": d.Date, "task_total": d.Total, "task_done": d.Done, "abnormal": abByDay[d.Date]})
	}

	stats := types.JSONMap{
		"task_total":     taskSum.Total,
		"task_done":      taskSum.Done,
		"task_overdue":   taskSum.Overdue,
		"should_points":  taskSum.Should,
		"done_points":    taskSum.DonePts,
		"coverage_rate":  pct(taskSum.DonePts, taskSum.Should),
		"abnormal_count": ckSum.Abnormal,
		"suspect_count":  ckSum.Suspect,
		"wo_created":     woSum.Created,
		"wo_closed":      woSum.Closed,
		"wo_unclosed":    woSum.Created - woSum.Closed,
		"wo_close_rate":  pct(woSum.Closed, woSum.Created),
		"daily":          daily,
		"records":        s.collectRecords(communityID, start, end),
	}
	return stats, types.IDArray(inspectorIDs), nil
}

// collectRecords 收集该小区该期间全部打卡记录明细（按打卡时间正序），供 stats.records 快照存储。
// 字段：打卡时间/巡检员姓名/点位名称/打卡方式/距点位距离/结果/疑似标记/审核状态/照片 file_key 列表。
func (s *ReportService) collectRecords(communityID string, start, end time.Time) []gin.H {
	var recs []insmodel.CheckinRecord
	if err := s.db.Where("community_id = ? AND checkin_time >= ? AND checkin_time < ?", communityID, start, end).
		Order("checkin_time ASC, id ASC").Find(&recs).Error; err != nil {
		return []gin.H{}
	}
	// 批量解析巡检员姓名与点位名称
	userIDSet, pointIDSet := map[string]bool{}, map[string]bool{}
	for _, rec := range recs {
		userIDSet[rec.InspectorID] = true
		pointIDSet[rec.PointID] = true
	}
	userNames := map[string]string{}
	if len(userIDSet) > 0 {
		ids := make([]string, 0, len(userIDSet))
		for id := range userIDSet {
			ids = append(ids, id)
		}
		var users []sysmodel.SysUser
		s.db.Select("id", "name").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			userNames[u.ID] = u.Name
		}
	}
	pointNames := map[string]string{}
	if len(pointIDSet) > 0 {
		ids := make([]string, 0, len(pointIDSet))
		for id := range pointIDSet {
			ids = append(ids, id)
		}
		var points []insmodel.InspectionPoint
		s.db.Select("id", "name").Where("id IN ?", ids).Find(&points)
		for _, pt := range points {
			pointNames[pt.ID] = pt.Name
		}
	}
	rows := make([]gin.H, 0, len(recs))
	for _, rec := range recs {
		var distance any
		if rec.DistanceToPoint != nil {
			distance = *rec.DistanceToPoint
		}
		photoKeys := make([]string, 0, len(rec.Photos))
		for _, p := range rec.Photos {
			if k := photoFileKey(p.URL); k != "" {
				photoKeys = append(photoKeys, k)
			}
		}
		rows = append(rows, gin.H{
			"checkin_time":   timefmt.T(rec.CheckinTime),
			"inspector_name": userNames[rec.InspectorID],
			"point_name":     pointNames[rec.PointID],
			"checkin_type":   rec.CheckinType,
			"distance":       distance,
			"result":         rec.Result,
			"is_suspect":     rec.IsSuspect,
			"audit_status":   rec.AuditStatus,
			"photo_keys":     photoKeys,
		})
	}
	return rows
}

// photoFileKey 从照片 URL 反推 file_key（dev：.../uploads/{key}；oss：https://{bucket}.{endpoint}/{key}）。
func photoFileKey(u string) string {
	if i := strings.Index(u, "/uploads/"); i >= 0 {
		return u[i+len("/uploads/"):]
	}
	if pu, err := url.Parse(u); err == nil && pu.Host != "" {
		return strings.TrimPrefix(pu.Path, "/")
	}
	return ""
}

// statsRecords 从 stats JSONB 读取 records 快照并规范字段类型；无该字段返回 ok=false。
func statsRecords(st types.JSONMap) ([]gin.H, bool) {
	arr, ok := st["records"].([]any)
	if !ok {
		return nil, false
	}
	rows := make([]gin.H, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		keys := []string{}
		switch arr := m["photo_keys"].(type) {
		case []any:
			for _, k := range arr {
				keys = append(keys, fmt.Sprint(k))
			}
		case []string:
			keys = arr
		}
		rows = append(rows, gin.H{
			"checkin_time":   fmt.Sprint(m["checkin_time"]),
			"inspector_name": fmt.Sprint(m["inspector_name"]),
			"point_name":     fmt.Sprint(m["point_name"]),
			"checkin_type":   fmt.Sprint(m["checkin_type"]),
			"distance":       m["distance"],
			"result":         fmt.Sprint(m["result"]),
			"is_suspect":     m["is_suspect"] == true,
			"audit_status":   fmt.Sprint(m["audit_status"]),
			"photo_keys":     keys,
		})
	}
	return rows, true
}

// reportRecords 报告打卡明细行：stats.records 快照优先；历史报告缺快照时实时查询兜底。
func (s *ReportService) reportRecords(r *model.InspectionReport) []gin.H {
	if rows, ok := statsRecords(r.Stats); ok {
		return rows
	}
	start, end, be := periodRange(r.Period)
	if be != nil {
		return []gin.H{}
	}
	return s.collectRecords(r.CommunityID, start, end)
}

// recordsWithURLs 明细行 photo_keys 转 photos[{url}]，供详情接口输出。
func (s *ReportService) recordsWithURLs(rows []gin.H) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		keys, _ := row["photo_keys"].([]string)
		photos := make([]gin.H, 0, len(keys))
		for _, k := range keys {
			photos = append(photos, gin.H{"url": s.store.URL(k)})
		}
		out = append(out, gin.H{
			"checkin_time":   row["checkin_time"],
			"inspector_name": row["inspector_name"],
			"point_name":     row["point_name"],
			"checkin_type":   row["checkin_type"],
			"distance":       row["distance"],
			"result":         row["result"],
			"is_suspect":     row["is_suspect"],
			"audit_status":   row["audit_status"],
			"photos":         photos,
		})
	}
	return out
}

// List 报告分页列表（含小区名与签字概要）。
func (s *ReportService) List(c *gin.Context, q *dto.ReportListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.InspectionReport{})
	db = middleware.ApplyCommunityFilter(db, c, "inspection_report.community_id")
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.Period != "" {
		db = db.Where("period = ?", q.Period)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.InspectionReport
	offset, limit := q.Normalize()
	if err := db.Order("period DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		list = append(list, gin.H{
			"id": r.ID, "community_id": r.CommunityID, "community_name": s.commName(r.CommunityID),
			"period": r.Period, "title": r.Title, "status": r.Status,
			"inspector_total":        len(r.InspectorIDs),
			"inspector_signed_count": len(r.InspectorSigned),
			"supervisor_name":        s.userNamePtr(r.SupervisorBy),
			"manager_name":           s.userNamePtr(r.ManagerBy),
			"has_file":               r.FileKey != "",
			"created_at":             timefmt.T(r.CreatedAt),
			"updated_at":             timefmt.T(r.UpdatedAt),
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Detail 报告详情（stats 全量 + 签字明细 + file_url）。
func (s *ReportService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, be
	}
	// 应确认巡检员明细（补姓名、签字状态与签名图快照 URL）
	signedBy := map[string]string{}
	signedSig := map[string]string{}
	for _, e := range r.InspectorSigned {
		signedBy[e.UserID] = e.SignedAt
		signedSig[e.UserID] = e.SignatureKey
	}
	inspectors := make([]gin.H, 0, len(r.InspectorIDs))
	for _, uid := range r.InspectorIDs {
		at, ok := signedBy[uid]
		inspectors = append(inspectors, gin.H{
			"user_id": uid, "name": s.userName(uid), "signed": ok, "signed_at": at,
			"signature_url": s.sigURL(signedSig[uid]),
		})
	}
	var fileURL any
	if r.FileKey != "" {
		fileURL = s.store.URL(r.FileKey)
	}
	return gin.H{
		"id": r.ID, "community_id": r.CommunityID, "community_name": s.commName(r.CommunityID),
		"period": r.Period, "title": r.Title, "status": r.Status, "stats": r.Stats,
		"records":       s.recordsWithURLs(s.reportRecords(r)),
		"inspector_ids": r.InspectorIDs, "inspectors": inspectors,
		"inspector_signed":  r.InspectorSigned,
		"supervisor_by":     r.SupervisorBy,
		"supervisor_name":   s.userNamePtr(r.SupervisorBy),
		"supervisor_at":     timefmt.TP(r.SupervisorAt),
		"supervisor_remark": r.SupervisorRemark,
		"supervisor_signature_url": s.sigURL(r.SupervisorSignatureKey),
		"manager_by":        r.ManagerBy,
		"manager_name":      s.userNamePtr(r.ManagerBy),
		"manager_at":        timefmt.TP(r.ManagerAt),
		"manager_remark":    r.ManagerRemark,
		"manager_signature_url": s.sigURL(r.ManagerSignatureKey),
		"reject_reason":     r.RejectReason,
		"file_key":          r.FileKey,
		"file_url":          fileURL,
		"created_at":        timefmt.T(r.CreatedAt),
		"updated_at":        timefmt.T(r.UpdatedAt),
	}, nil
}

// Generate 手动生成/重算报告（approved 不可重算；已存在则重算 stats 并重置签字流程）。
func (s *ReportService) Generate(c *gin.Context, req *dto.GenerateReq) (gin.H, *errs.Error) {
	if _, _, be := periodRange(req.Period); be != nil {
		return nil, be
	}
	if be := middleware.CheckCommunity(c, req.CommunityID); be != nil {
		return nil, be
	}
	name := s.commName(req.CommunityID)
	if name == "" {
		return nil, errs.ErrCommunityNotExist
	}
	stats, inspectorIDs, be := s.buildStats(req.CommunityID, req.Period)
	if be != nil {
		return nil, be
	}
	title := reportTitle(name, req.Period)

	var r model.InspectionReport
	err := s.db.Where("community_id = ? AND period = ?", req.CommunityID, req.Period).First(&r).Error
	if err == nil {
		if r.Status == model.StatusApproved {
			return nil, errs.ErrReportApproved
		}
		// 重算：重置签字流程
		updates := map[string]any{
			"title": title, "status": model.StatusPendingInspector,
			"stats": stats, "inspector_ids": inspectorIDs,
			"inspector_signed": types.SignArray{},
			"supervisor_by":    nil, "supervisor_at": nil, "supervisor_remark": "",
			"manager_by": nil, "manager_at": nil, "manager_remark": "",
			"reject_reason": "", "file_key": "", "seal_file_key": "",
		}
		if err := s.db.Model(&r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		return gin.H{"id": r.ID, "title": title, "status": model.StatusPendingInspector, "regenerated": true}, nil
	}
	r = model.InspectionReport{
		CommunityID: req.CommunityID, Period: req.Period, Title: title,
		Status: model.StatusPendingInspector, Stats: stats,
		InspectorIDs: inspectorIDs, InspectorSigned: types.SignArray{},
	}
	if err := s.db.Create(&r).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"id": r.ID, "title": title, "status": r.Status, "regenerated": false}, nil
}

// GenerateMonthlyAll 定时任务：为每个启用小区生成指定期间报告（已存在则跳过，幂等）。
func (s *ReportService) GenerateMonthlyAll(period string) (int, error) {
	var comms []sysmodel.Community
	if err := s.db.Where("status = ?", sysmodel.StatusEnabled).Find(&comms).Error; err != nil {
		return 0, err
	}
	created := 0
	for _, comm := range comms {
		var count int64
		s.db.Model(&model.InspectionReport{}).
			Where("community_id = ? AND period = ?", comm.ID, period).Count(&count)
		if count > 0 {
			continue
		}
		stats, inspectorIDs, be := s.buildStats(comm.ID, period)
		if be != nil {
			return created, be
		}
		r := model.InspectionReport{
			CommunityID: comm.ID, Period: period, Title: reportTitle(comm.Name, period),
			Status: model.StatusPendingInspector, Stats: stats,
			InspectorIDs: inspectorIDs, InspectorSigned: types.SignArray{},
		}
		if err := s.db.Create(&r).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// SignInspector 巡检员电子确认：inspector_signed 追加留痕；全员签完流转 pending_supervisor 并通知主管。
func (s *ReportService) SignInspector(c *gin.Context, id string) (gin.H, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, be
	}
	if r.Status != model.StatusPendingInspector {
		return nil, errs.ErrReportStatusNotAllowed.WithMsg("当前状态不可巡检员确认")
	}
	identity := middleware.CurrentIdentity(c)
	if !identity.SuperAdmin && !r.InspectorIDs.Contains(identity.UserID) {
		return nil, errs.ErrReportNotInspector
	}
	for _, e := range r.InspectorSigned {
		if e.UserID == identity.UserID {
			return nil, errs.ErrReportAlreadySigned
		}
	}
	sigKey, sigAssetID := s.signatureAssetOf(identity.UserID)
	signed := append(types.SignArray{}, r.InspectorSigned...)
	signed = append(signed, types.SignEntry{
		UserID: identity.UserID, Name: identity.Name, SignedAt: timefmt.T(time.Now()),
		SignatureKey: sigKey, AssetID: sigAssetID, // 签名图+资产快照：防止后续换签名影响历史报告
	})
	// 全部应签巡检员均已确认（无应签巡检员时超管确认即视为完成）
	allSigned := true
	for _, uid := range r.InspectorIDs {
		found := false
		for _, e := range signed {
			if e.UserID == uid {
				found = true
				break
			}
		}
		if !found {
			allSigned = false
			break
		}
	}
	updates := map[string]any{"inspector_signed": signed}
	newStatus := r.Status
	if allSigned {
		newStatus = model.StatusPendingSupervisor
		updates["status"] = newStatus
	}
	if err := s.db.Model(r).Updates(updates).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if allSigned {
		s.notifyRole("manager", "月报待主管审批",
			fmt.Sprintf("「%s」全体巡检员已确认，请安全主管审批", r.Title), &r.ID)
	}
	return gin.H{"status": newStatus, "signed_count": len(signed), "inspector_total": len(r.InspectorIDs)}, nil
}

// SignSupervisor 主管签批（approve → pending_manager 通知超管；reject → 回退 pending_inspector）。
func (s *ReportService) SignSupervisor(c *gin.Context, id string, req *dto.SignReq) (gin.H, *errs.Error) {
	return s.sign(c, id, req, model.StatusPendingSupervisor)
}

// SignManager 经理终审（approve → approved + 异步 PDF 归档；reject → 回退 pending_inspector）。
func (s *ReportService) SignManager(c *gin.Context, id string, req *dto.SignReq) (gin.H, *errs.Error) {
	return s.sign(c, id, req, model.StatusPendingManager)
}

// sign 主管/经理共用签批逻辑。
func (s *ReportService) sign(c *gin.Context, id string, req *dto.SignReq, expectStatus string) (gin.H, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, be
	}
	if r.Status != expectStatus {
		return nil, errs.ErrReportStatusNotAllowed
	}
	identity := middleware.CurrentIdentity(c)
	now := time.Now()

	if req.Action == "reject" {
		if req.Reason == "" {
			return nil, errs.ErrReportRejectReasonRequired
		}
		// 驳回回 pending_inspector：清空巡检员确认，记驳回原因
		signed := types.SignArray{}
		signed = append(signed, r.InspectorSigned...)
		updates := map[string]any{
			"status":           model.StatusPendingInspector,
			"inspector_signed": types.SignArray{},
			"reject_reason":    req.Reason,
		}
		if err := s.db.Model(r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		// 通知已签巡检员重新确认
		for _, e := range signed {
			s.notify(e.UserID, "report", "月报被驳回",
				fmt.Sprintf("「%s」被驳回：%s，请重新确认", r.Title, req.Reason), &r.ID)
		}
		return gin.H{"status": model.StatusPendingInspector}, nil
	}

	switch expectStatus {
	case model.StatusPendingSupervisor:
		sigKey, _ := s.signatureAssetOf(identity.UserID)
		updates := map[string]any{
			"status":        model.StatusPendingManager,
			"supervisor_by": identity.UserID, "supervisor_at": now, "supervisor_remark": req.Remark,
			"supervisor_signature_key": sigKey,
		}
		if err := s.db.Model(r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		s.notifyRole(sysmodel.SuperAdminCode, "月报待经理终审",
			fmt.Sprintf("「%s」主管已审批通过，请物业经理终审", r.Title), &r.ID)
		return gin.H{"status": model.StatusPendingManager}, nil
	default: // pending_manager → approved
		sigKey, _ := s.signatureAssetOf(identity.UserID)
		updates := map[string]any{
			"status":     model.StatusApproved,
			"manager_by": identity.UserID, "manager_at": now, "manager_remark": req.Remark,
			"manager_signature_key": sigKey,
			// 公章快照：固化终审时点的 active 公章，后续换章不影响已归档报告
			"seal_file_key": s.activeSealKey(),
		}
		if err := s.db.Model(r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		go s.archivePDF(r.ID)
		return gin.H{"status": model.StatusApproved}, nil
	}
}

// PDF 返回报告 PDF 字节与文件名；file_key 已归档则直接读归档文件，否则即时生成临时版。
func (s *ReportService) PDF(c *gin.Context, id string) ([]byte, string, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, "", be
	}
	filename := r.Title + ".pdf"
	if r.FileKey != "" && s.store.IsDev() {
		data, err := os.ReadFile(s.store.LocalPath(r.FileKey))
		if err != nil {
			return nil, "", errs.ErrInternal
		}
		return data, filename, nil
	}
	data, err := pdf.GenerateMonthly(s.pdfData(r))
	if err != nil {
		logger.L.Warn("月报 PDF 即时生成失败", zap.Error(err), zap.String("report_id", r.ID))
		return nil, "", errs.ErrInternal
	}
	return data, filename, nil
}

// loadPhoto 按 file_key 读取照片字节与图片类型（dev 读本地文件；oss HTTP 下载）。
func (s *ReportService) loadPhoto(fileKey string) ([]byte, string, error) {
	var imgType string
	switch strings.ToLower(filepath.Ext(fileKey)) {
	case ".jpg", ".jpeg":
		imgType = "JPG"
	case ".png":
		imgType = "PNG"
	default:
		return nil, "", fmt.Errorf("不支持的照片格式: %s", fileKey)
	}
	if s.store.IsDev() {
		data, err := os.ReadFile(s.store.LocalPath(fileKey))
		return data, imgType, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(s.store.URL(fileKey))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("下载照片失败: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	return data, imgType, err
}

// RebuildPDF 用当前模板重渲染报告 PDF 并覆盖归档文件（状态/签字/统计数据保持不变）。
// 供模板升级后人工刷新存量报告：dev 覆盖 file_key 对应本地文件；oss 重新保存并回写 file_key。
func (s *ReportService) RebuildPDF(reportID string) error {
	var r model.InspectionReport
	if err := s.db.First(&r, "id = ?", reportID).Error; err != nil {
		return err
	}
	data, err := pdf.GenerateMonthly(s.pdfData(&r))
	if err != nil {
		return err
	}
	if s.store.IsDev() && r.FileKey != "" {
		if err := os.WriteFile(s.store.LocalPath(r.FileKey), data, 0o644); err != nil {
			return err
		}
		logger.L.Info("月报 PDF 重渲染完成", zap.String("report_id", reportID), zap.String("file_key", r.FileKey))
		return nil
	}
	key, _, err := s.store.SaveGenerated("reports", r.Title+".pdf", data)
	if err != nil {
		return err
	}
	if err := s.db.Model(&r).Update("file_key", key).Error; err != nil {
		return err
	}
	logger.L.Info("月报 PDF 重渲染完成", zap.String("report_id", reportID), zap.String("file_key", key))
	return nil
}

// archivePDF 终审后异步生成 PDF 并归档（回写 file_key）。
func (s *ReportService) archivePDF(reportID string) {
	var r model.InspectionReport
	if err := s.db.First(&r, "id = ?", reportID).Error; err != nil {
		logger.L.Warn("月报归档：报告不存在", zap.String("report_id", reportID))
		return
	}
	data, err := pdf.GenerateMonthly(s.pdfData(&r))
	if err != nil {
		logger.L.Warn("月报归档：PDF 生成失败", zap.Error(err), zap.String("report_id", reportID))
		return
	}
	key, _, err := s.store.SaveGenerated("reports", r.Title+".pdf", data)
	if err != nil {
		logger.L.Warn("月报归档：保存失败", zap.Error(err), zap.String("report_id", reportID))
		return
	}
	if err := s.db.Model(&r).Update("file_key", key).Error; err != nil {
		logger.L.Warn("月报归档：回写 file_key 失败", zap.Error(err), zap.String("report_id", reportID))
		return
	}
	logger.L.Info("月报 PDF 归档完成", zap.String("report_id", reportID), zap.String("file_key", key))
}

// notify 写站内消息（仿 OrderService.Notify）。
func (s *ReportService) notify(userID, msgType, title, content string, bizID *string) {
	s.db.Create(&sysmodel.SysMessage{UserID: userID, Type: msgType, Title: title, Content: content, BizID: bizID})
}

// notifyRole 通知某角色全部启用用户（逐人插入 sys_message）。
func (s *ReportService) notifyRole(roleCode, title, content string, bizID *string) {
	var role sysmodel.SysRole
	if err := s.db.Select("id").First(&role, "code = ? AND status = ?", roleCode, sysmodel.StatusEnabled).Error; err != nil {
		return
	}
	var users []sysmodel.SysUser
	s.db.Select("id").Where("status = ? AND role_ids @> ?", sysmodel.StatusEnabled, fmt.Sprintf(`["%s"]`, role.ID)).Find(&users)
	for _, u := range users {
		s.notify(u.ID, "report", title, content, bizID)
	}
}

// getWithScope 取报告并做数据权限校验。
func (s *ReportService) getWithScope(c *gin.Context, id string) (*model.InspectionReport, *errs.Error) {
	var r model.InspectionReport
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, r.CommunityID); be != nil {
		return nil, be
	}
	return &r, nil
}

func (s *ReportService) commName(id string) string {
	var c sysmodel.Community
	if s.db.Select("name").First(&c, "id = ?", id).Error == nil {
		return c.Name
	}
	return ""
}

func (s *ReportService) userName(id string) string {
	var u sysmodel.SysUser
	if s.db.Select("name").First(&u, "id = ?", id).Error == nil {
		return u.Name
	}
	return ""
}

func (s *ReportService) userNamePtr(id *string) any {
	if id == nil {
		return nil
	}
	return s.userName(*id)
}

// signatureAssetOf 读取用户当前 active 签名资产（签字时快照 file_key 与资产 id，防后续换签名影响历史报告）。
func (s *ReportService) signatureAssetOf(userID string) (fileKey, assetID string) {
	var a sysmodel.SignAsset
	if err := s.db.Select("id", "file_key").
		Where("asset_type = ? AND owner_id = ? AND status = ?",
			sysmodel.SignAssetTypeUserSignature, userID, sysmodel.SignAssetStatusActive).
		First(&a).Error; err != nil {
		return "", ""
	}
	return a.FileKey, a.ID
}

// activeSealKey 当前 active 公章资产 file_key（无则空串）。
func (s *ReportService) activeSealKey() string {
	var a sysmodel.SignAsset
	if err := s.db.Select("file_key").
		Where("asset_type = ? AND status = ?", sysmodel.SignAssetTypeCompanySeal, sysmodel.SignAssetStatusActive).
		First(&a).Error; err != nil {
		return ""
	}
	return a.FileKey
}

// sigURL 签名图快照 file_key → 可访问 URL（空返回 nil）。
func (s *ReportService) sigURL(key string) any {
	if key == "" {
		return nil
	}
	return s.store.URL(key)
}
