// Package service 月度巡检报告业务逻辑：生成/重算 + 三级电子确认签字 + PDF 归档。
package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/report/dto"
	"anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/pdf"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"

	"go.uber.org/zap"
)

// ReportService 月度报告服务。
type ReportService struct {
	db       *gorm.DB
	rdb      *redis.Client
	store    *storage.Storage
	getCfg   func(key string) (string, bool) // 读取系统参数（report.company_name；公章自 v16 起走 sign_asset 资产表）
	notifier *notify.Notifier
}

func NewReportService(db *gorm.DB, rdb *redis.Client, store *storage.Storage, getCfg func(string) (string, bool), notifier *notify.Notifier) *ReportService {
	return &ReportService{db: db, rdb: rdb, store: store, getCfg: getCfg, notifier: notifier}
}

// reportLayout 报告版式（report.layout 配置：ledger=台账版（甲方样稿骨架，分项按点位类型）/ generic=通用版；
// 缺省 ledger——台账版分项由点位类型与检查项模板数据驱动，任何巡查类型通用）。
func (s *ReportService) reportLayout() string {
	if v := s.cfgString("report.layout"); v == "generic" {
		return "generic"
	}
	return "ledger"
}

// reportTitleLine 封面大标题首行：report.title 配置优先（如「物业消防设施（器材类）月度」）；
// 缺省按巡查类型推导（「物业{类型label}月度」，综合月报回落「物业综合月度」）。
func (s *ReportService) reportTitleLine(r *model.InspectionReport) string {
	if v := s.cfgString("report.title"); v != "" {
		return v
	}
	label := s.patrolTypeLabel(r.PatrolType)
	if label == "" || label == "综合" || label == "综合月报" {
		return "物业综合月度"
	}
	return "物业" + label + "月度"
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

// specialReportTitle 专项检查报告标题（§3.4）：{小区名}{YYYY年M月}{类型名}专项检查报告。
// 类型字典 label 尾部含「专项巡查/专项/巡查」时先裁掉再拼接，避免「专项专项/巡查专项」叠字
// （如「设备设施专项巡查」→「设备设施专项检查报告」）；其他 label 原样拼接。
func specialReportTitle(commName, period, typeLabel string) string {
	label := strings.TrimSpace(typeLabel)
	for _, suffix := range []string{"专项巡查", "专项", "巡查"} {
		if trimmed := strings.TrimSuffix(label, suffix); trimmed != label && trimmed != "" {
			label = trimmed
			break
		}
	}
	t, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return commName + period + label + "专项检查报告"
	}
	return fmt.Sprintf("%s%d年%d月%s专项检查报告", commName, t.Year(), int(t.Month()), label)
}

// supervisorSlot 主管级签字默认槽位（§3.4「签字名单怎么定」）：
// patrolType 空=综合月报，维持固定「月报主管级签字」槽位；非空=该类型汇报线槽位
// （resolve 为维度槽位解析器：patrol_report_line.<type> 有绑定即命中，否则回落通用 patrol_report_line）。
func supervisorSlot(patrolType string, resolve func(patrolType string) string) string {
	if patrolType == "" {
		return sysmodel.SlotReportSignSupervisor
	}
	return resolve(patrolType)
}

// scopeTaskType 任务查询叠加巡查类型过滤（空=综合口径全类型，不过滤）。
func scopeTaskType(db *gorm.DB, patrolType string) *gorm.DB {
	if patrolType == "" {
		return db
	}
	return db.Where("patrol_type = ?", patrolType)
}

// scopeCheckinType 打卡查询叠加巡查类型过滤（打卡无 patrol_type 列，经 task_id 关联任务快照类型）。
func scopeCheckinType(db *gorm.DB, patrolType string) *gorm.DB {
	if patrolType == "" {
		return db
	}
	return db.Where("task_id IN (SELECT id FROM inspection_task WHERE patrol_type = ? AND deleted_at IS NULL)", patrolType)
}

// buildStats 聚合指定小区指定月度的统计数据与应确认巡检员集合。
// patrolType 空=综合口径（全类型）；非空=只统计该类型（任务/打卡/异常均按类型过滤）。
func (s *ReportService) buildStats(communityID, period, patrolType string) (types.JSONMap, types.IDArray, *errs.Error) {
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
	scopeTaskType(s.db.Model(&insmodel.InspectionTask{}), patrolType).
		Where("community_id = ? AND task_date >= ? AND task_date < ?", communityID, startStr, endStr).
		Select(`COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'done') AS done,
			COUNT(*) FILTER (WHERE status = 'overdue') AS overdue,
			COALESCE(SUM(total_points),0) AS should,
			COALESCE(SUM(done_points),0) AS done_pts`).Scan(&taskSum)

	// 应确认巡检员：当月该小区有任务的巡检员去重
	var inspectorIDs []string
	scopeTaskType(s.db.Model(&insmodel.InspectionTask{}), patrolType).Distinct().
		Where("community_id = ? AND task_date >= ? AND task_date < ?", communityID, startStr, endStr).
		Pluck("inspector_id", &inspectorIDs)

	// 打卡异常/疑似
	var ckSum struct {
		Abnormal int64
		Suspect  int64
	}
	scopeCheckinType(s.db.Model(&insmodel.CheckinRecord{}), patrolType).
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ? AND superseded_by IS NULL", communityID, start, end).
		Select("COUNT(*) FILTER (WHERE result = 'abnormal') AS abnormal, COUNT(*) FILTER (WHERE is_suspect) AS suspect").Scan(&ckSum)

	// 逐日明细：任务（日期/任务/完成）+ 当日异常打卡
	type taskDay struct {
		Date  string
		Total int64
		Done  int64
	}
	var taskDays []taskDay
	scopeTaskType(s.db.Model(&insmodel.InspectionTask{}), patrolType).
		Where("community_id = ? AND task_date >= ? AND task_date < ?", communityID, startStr, endStr).
		Select("to_char(task_date, 'YYYY-MM-DD') AS date, COUNT(*) AS total, COUNT(*) FILTER (WHERE status = 'done') AS done").
		Group("date").Order("date").Scan(&taskDays)
	type ckDay struct {
		Date     string
		Abnormal int64
	}
	var ckDays []ckDay
	scopeCheckinType(s.db.Model(&insmodel.CheckinRecord{}), patrolType).
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ? AND result = 'abnormal' AND superseded_by IS NULL", communityID, start, end).
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
		"issue_count":    ckSum.Abnormal, // 当期异常打卡数（原 wo_created 口径的替代指标）
		"daily":          daily,
		"records":        s.collectRecords(communityID, start, end, patrolType),
	}
	return stats, types.IDArray(inspectorIDs), nil
}

// lockPeriodCheckins 报告生成成功后归档锁定当期打卡记录：锁定后该点位打卡不可覆盖修改（43109）。
// 紧随报告写库之后的写操作（报告生成本身非事务，失败仅记日志不影响报告结果）；
// 已被覆盖的旧记录（superseded_by 非空）与已锁定记录不重复处理。
func (s *ReportService) lockPeriodCheckins(communityID, period string) {
	start, end, be := periodRange(period)
	if be != nil {
		return
	}
	if err := s.db.Model(&insmodel.CheckinRecord{}).
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ? AND superseded_by IS NULL AND locked_at IS NULL", communityID, start, end).
		Update("locked_at", time.Now()).Error; err != nil {
		logger.L.Warn("打卡记录归档锁定失败", zap.String("community_id", communityID), zap.String("period", period), zap.Error(err))
	}
}

// collectRecords 收集该小区该期间打卡记录明细（按打卡时间正序；patrolType 非空只取该类型），供 stats.records 快照存储。
// 字段：打卡时间/巡检员姓名/点位名称/打卡方式/距点位距离/结果/疑似标记/审核状态/照片 file_key 列表。
func (s *ReportService) collectRecords(communityID string, start, end time.Time, patrolType string) []gin.H {
	var recs []insmodel.CheckinRecord
	if err := scopeCheckinType(s.db, patrolType).
		Where("community_id = ? AND checkin_time >= ? AND checkin_time < ? AND superseded_by IS NULL", communityID, start, end).
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
	// 批量取逐项照片（v21 起照片唯一归属逐项）
	recIDs := make([]string, 0, len(recs))
	for _, rec := range recs {
		recIDs = append(recIDs, rec.ID)
	}
	photoKeysByRec := map[string][]string{}
	if len(recIDs) > 0 {
		var recItems []insmodel.CheckinRecordItem
		s.db.Select("record_id", "photos").Where("record_id IN ?", recIDs).Find(&recItems)
		for _, ci := range recItems {
			photoKeysByRec[ci.RecordID] = append(photoKeysByRec[ci.RecordID], ci.Photos...)
		}
	}
	rows := make([]gin.H, 0, len(recs))
	for _, rec := range recs {
		var distance any
		if rec.DistanceToPoint != nil {
			distance = *rec.DistanceToPoint
		}
		photoKeys := photoKeysByRec[rec.ID]
		rows = append(rows, gin.H{
			"checkin_time":   timefmt.T(rec.CheckinTime),
			"inspector_name": userNames[rec.InspectorID],
			"point_name":     pointNames[rec.PointID],
			"checkin_type":   rec.CheckinType,
			"distance":       distance,
			"result":         rec.Result,
			"is_suspect":     rec.IsSuspect,
			"audit_status":   rec.AuditStatus,
			"photo_ids":      photoKeys,
		})
	}
	return rows
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
		switch arr := m["photo_ids"].(type) {
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
			"photo_ids":      keys,
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
	return s.collectRecords(r.CommunityID, start, end, r.PatrolType)
}

// recordsWithURLs 明细行 photo_ids 转 photos[{file_id,file_key,url}]，供详情接口输出。
func (s *ReportService) recordsWithURLs(rows []gin.H) []gin.H {
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		keys, _ := row["photo_ids"].([]string)
		photos := make([]gin.H, 0, len(keys))
		files := uploadfile.ByIDs(s.db, keys)
		for _, k := range keys {
			f, ok := files[k]
			if !ok {
				continue
			}
			url := f.URL
			if f.WatermarkedURL != "" {
				url = f.WatermarkedURL
			}
			photos = append(photos, gin.H{"file_id": f.ID, "url": url})
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
	if identity := middleware.CurrentIdentity(c); identity != nil && identity.ScopeSelf {
		// self 档（纯一线岗位）：仅本人相关报告（三级签字名单任一含本人）
		j := fmt.Sprintf(`["%s"]`, identity.UserID)
		db = db.Where(`inspector_ids @> ?::jsonb OR supervisor_ids @> ?::jsonb OR manager_ids @> ?::jsonb`, j, j, j)
	} else {
		db = middleware.ApplyCommunityFilter(db, c, "inspection_report.community_id")
	}
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.Period != "" {
		db = db.Where("period = ?", q.Period)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 巡查类型筛选：none=综合月报（patrol_type 为空）；其余按类型等值（存量行 patrol_type 为 NULL，综合筛选须带上 IS NULL）
	if q.PatrolType == "none" {
		db = db.Where("patrol_type IS NULL OR patrol_type = ''")
	} else if q.PatrolType != "" {
		db = db.Where("patrol_type = ?", q.PatrolType)
	}
	// 只看待我签：当前用户处于该报告当前级签字人名单内（巡检员级按应签名单，主管/经理级按指定名单）
	if q.PendingMine == "1" || q.PendingMine == "true" {
		if identity := middleware.CurrentIdentity(c); identity != nil {
			j := fmt.Sprintf(`["%s"]`, identity.UserID)
			db = db.Where(`(status = 'pending_inspector' AND inspector_ids @> ?::jsonb)
				OR (status = 'pending_supervisor' AND supervisor_ids @> ?::jsonb)
				OR (status = 'pending_manager' AND manager_ids @> ?::jsonb)`, j, j, j)
		}
	}
	// 我签过的：三级任一签署留痕含当前用户。1/true 含已归档（签完不消失，可回看完整报告）；
	// doing 只取流程未走完的（App「进行中」选项卡）。
	if q.SignedMine != "" {
		if identity := middleware.CurrentIdentity(c); identity != nil {
			uid := identity.UserID
			j := fmt.Sprintf(`[{"user_id":"%s"}]`, uid)
			signedCond := `inspector_signed @> ?::jsonb OR supervisor_by = ? OR manager_by = ?`
			if q.SignedMine == "doing" {
				db = db.Where(`status <> 'approved' AND (`+signedCond+`)`, j, uid, uid)
			} else if q.SignedMine == "1" || q.SignedMine == "true" {
				db = db.Where(`status = 'approved' OR `+signedCond, j, uid, uid)
			}
		}
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
			"patrol_type":            r.PatrolType,
			"patrol_type_label":      s.patrolTypeLabel(r.PatrolType),
			"plan_id":                r.PlanID,
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

// Detail 报告详情（stats 全量 + 签字明细 + file_id/file_url）。
func (s *ReportService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, be
	}
	// 应确认巡检员明细（补姓名、签字状态与签名图快照 URL；代签附带代签人与原因）
	signedBy := map[string]string{}
	signedSig := map[string]string{}
	proxyBy := map[string][2]string{}
	for _, e := range r.InspectorSigned {
		signedBy[e.UserID] = e.SignedAt
		signedSig[e.UserID] = e.SignatureKey
		if e.ProxyBy != "" {
			proxyBy[e.UserID] = [2]string{e.ProxyName, e.ProxyReason}
		}
	}
	inspectors := make([]gin.H, 0, len(r.InspectorIDs))
	for _, uid := range r.InspectorIDs {
		at, ok := signedBy[uid]
		entry := gin.H{
			"user_id": uid, "name": s.userName(uid), "signed": ok, "signed_at": at,
			"signature_url": s.sigURL(signedSig[uid]),
		}
		if p, isProxy := proxyBy[uid]; isProxy {
			entry["proxy_name"] = p[0]
			entry["proxy_reason"] = p[1]
		}
		inspectors = append(inspectors, entry)
	}
	var fileURL any
	fileID := ""
	if r.FileKey != "" {
		fileURL = s.store.URL(r.FileKey)
		var f sysmodel.UploadFile
		if s.db.Where("storage_key = ?", r.FileKey).First(&f).Error == nil {
			fileID = f.ID
		}
	}
	return gin.H{
		"id": r.ID, "community_id": r.CommunityID, "community_name": s.commName(r.CommunityID),
		"period": r.Period, "title": r.Title, "status": r.Status, "stats": r.Stats,
		"patrol_type":       r.PatrolType,
		"patrol_type_label": s.patrolTypeLabel(r.PatrolType),
		"plan_id":           r.PlanID,
		"records":           s.recordsWithURLs(s.reportRecords(r)),
		"inspector_ids":     r.InspectorIDs, "inspectors": inspectors,
		"inspector_signed":         r.InspectorSigned,
		"supervisor_ids":           r.SupervisorIDs,
		"supervisors":              s.signerItems(r.SupervisorIDs, r.SupervisorBy),
		"manager_ids":              r.ManagerIDs,
		"managers":                 s.signerItems(r.ManagerIDs, r.ManagerBy),
		"supervisor_by":            r.SupervisorBy,
		"supervisor_name":          s.userNamePtr(r.SupervisorBy),
		"supervisor_at":            timefmt.TP(r.SupervisorAt),
		"supervisor_remark":        r.SupervisorRemark,
		"supervisor_signature_url": s.sigURL(r.SupervisorSignatureKey),
		"manager_by":               r.ManagerBy,
		"manager_name":             s.userNamePtr(r.ManagerBy),
		"manager_at":               timefmt.TP(r.ManagerAt),
		"manager_remark":           r.ManagerRemark,
		"manager_signature_url":    s.sigURL(r.ManagerSignatureKey),
		"reject_reason":            r.RejectReason,
		"file_id":                  fileID,
		"file_url":                 fileURL,
		"created_at":               timefmt.T(r.CreatedAt),
		"updated_at":               timefmt.T(r.UpdatedAt),
	}, nil
}

// Generate 手动生成/重算报告（approved 不可重算；已存在则重算 stats 并重置签字流程）。
// patrol_type 空=综合月报（现状），非空=该类型专项检查报告（§3.4：须为字典 patrol_type 启用项；
// 判重按 community_id+period+patrol_type，综合与专项可同月共存）；plan_id 仅溯源（须属于该小区）。
// 签字人名单：req 未传（nil）取槽位默认名单（项目级槽位绑定 → 平台默认 → 编制在职成员）；
// 专项报告主管级默认槽位换成该类型汇报线槽位（patrol_report_line.<type>，如 fire→工程主管）；
// 显式传数组按名单（仅校验用户存在且启用），空数组该级跳过。
func (s *ReportService) Generate(c *gin.Context, req *dto.GenerateReq) (gin.H, *errs.Error) {
	if _, _, be := periodRange(req.Period); be != nil {
		return nil, be
	}
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return nil, be
	}
	name := s.commName(req.CommunityID)
	if name == "" {
		return nil, errs.ErrCommunityNotExist
	}
	if req.PatrolType != "" && !s.validPatrolType(req.PatrolType) {
		return nil, errs.ErrParam.WithMsg("patrol_type 取值非法（须为字典 patrol_type 的启用项）")
	}
	var planID *string
	if req.PlanID != "" {
		var cnt int64
		if err := s.db.Model(&insmodel.InspectionPlan{}).
			Where("id = ? AND community_id = ?", req.PlanID, req.CommunityID).Count(&cnt).Error; err != nil {
			return nil, errs.ErrInternal
		}
		if cnt == 0 {
			return nil, errs.ErrParam.WithMsg("plan_id 须为该小区下的巡检计划")
		}
		planID = &req.PlanID
	}
	stats, inspectorIDs, be := s.buildStats(req.CommunityID, req.Period, req.PatrolType)
	if be != nil {
		return nil, be
	}
	supSlot := supervisorSlot(req.PatrolType, func(t string) string {
		return communitysvc.ResolveReportLineSlot(s.db, req.CommunityID, t)
	})
	supervisorIDs, be := s.resolveSigners(req.CommunityID, supSlot, req.SupervisorIDs)
	if be != nil {
		return nil, be
	}
	managerIDs, be := s.resolveSigners(req.CommunityID, sysmodel.SlotReportSignManager, req.ManagerIDs)
	if be != nil {
		return nil, be
	}
	title := reportTitle(name, req.Period)
	if req.PatrolType != "" {
		title = specialReportTitle(name, req.Period, s.patrolTypeLabel(req.PatrolType))
	}
	// 首个有签字人的级别起签；某级无签字人自动跳过（不伪造通过），三级全空则直接归档、签字栏留空
	initialStatus := firstSignStatus(inspectorIDs, supervisorIDs, managerIDs)

	var r model.InspectionReport
	// 判重/重算按 community_id+period+patrol_type（COALESCE 归一，综合月报 NULL/'' 等价，与唯一索引一致）
	err := s.db.Where("community_id = ? AND period = ? AND COALESCE(patrol_type, '') = ?",
		req.CommunityID, req.Period, req.PatrolType).First(&r).Error
	if err == nil {
		if r.Status == model.StatusApproved {
			return nil, errs.ErrReportApproved
		}
		// 重算：重置签字流程（plan_id 随请求刷新溯源）
		updates := map[string]any{
			"title": title, "status": initialStatus, "plan_id": planID,
			"stats": stats, "inspector_ids": inspectorIDs,
			"inspector_signed": types.SignArray{},
			"supervisor_ids":   supervisorIDs, "manager_ids": managerIDs,
			"supervisor_by": nil, "supervisor_at": nil, "supervisor_remark": "",
			"supervisor_signature_key": "",
			"manager_by":               nil, "manager_at": nil, "manager_remark": "",
			"manager_signature_key": "",
			"reject_reason":         "", "file_key": "", "seal_file_key": "",
		}
		if err := s.db.Model(&r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		s.lockPeriodCheckins(req.CommunityID, req.Period) // 归档锁定当期打卡（不可再覆盖修改）
		if initialStatus == model.StatusApproved {
			go s.archivePDF(r.ID)
		}
		return gin.H{"id": r.ID, "title": title, "status": initialStatus, "regenerated": true}, nil
	}
	r = model.InspectionReport{
		CommunityID: req.CommunityID, Period: req.Period, PatrolType: req.PatrolType, PlanID: planID, Title: title,
		TenantID: middleware.CommunityTenantID(s.db, req.CommunityID), // 冗余列（=所属小区租户）
		Status:   initialStatus, Stats: stats,
		InspectorIDs: inspectorIDs, InspectorSigned: types.SignArray{},
		SupervisorIDs: supervisorIDs, ManagerIDs: managerIDs,
	}
	if err := s.db.Create(&r).Error; err != nil {
		return nil, errs.ErrInternal
	}
	s.lockPeriodCheckins(req.CommunityID, req.Period) // 归档锁定当期打卡（不可再覆盖修改）
	if initialStatus == model.StatusApproved {
		go s.archivePDF(r.ID)
	}
	return gin.H{"id": r.ID, "title": title, "status": r.Status, "regenerated": false}, nil
}

// GenerateMonthlyAll 定时任务：为每个启用小区生成指定期间报告（已存在则跳过，幂等；
// 签字人取槽位默认名单，无绑定或无人员该级自动跳过）。
func (s *ReportService) GenerateMonthlyAll(period string) (int, error) {
	var comms []sysmodel.Community
	if err := s.db.Where("status = ?", sysmodel.StatusEnabled).Find(&comms).Error; err != nil {
		return 0, err
	}
	created := 0
	for _, comm := range comms {
		var count int64
		s.db.Model(&model.InspectionReport{}).
			Where("community_id = ? AND period = ? AND COALESCE(patrol_type, '') = ''", comm.ID, period).Count(&count)
		if count > 0 {
			continue
		}
		stats, inspectorIDs, be := s.buildStats(comm.ID, period, "") // 自动月报只生成综合月报（全类型口径）
		if be != nil {
			return created, be
		}
		supervisorIDs, be := s.resolveSigners(comm.ID, sysmodel.SlotReportSignSupervisor, nil)
		if be != nil {
			return created, be
		}
		managerIDs, be := s.resolveSigners(comm.ID, sysmodel.SlotReportSignManager, nil)
		if be != nil {
			return created, be
		}
		initialStatus := firstSignStatus(inspectorIDs, supervisorIDs, managerIDs)
		r := model.InspectionReport{
			TenantID:    &comm.TenantID, // 冗余列（=所属小区租户）
			CommunityID: comm.ID, Period: period, Title: reportTitle(comm.Name, period),
			Status: initialStatus, Stats: stats,
			InspectorIDs: inspectorIDs, InspectorSigned: types.SignArray{},
			SupervisorIDs: supervisorIDs, ManagerIDs: managerIDs,
		}
		if err := s.db.Create(&r).Error; err != nil {
			return created, err
		}
		s.lockPeriodCheckins(comm.ID, period) // 归档锁定当期打卡（不可再覆盖修改）
		if initialStatus == model.StatusApproved {
			go s.archivePDF(r.ID)
		}
		created++
	}
	return created, nil
}

// firstSignStatus 首个有签字人的签字级别；三级均无签字人返回 approved（签字栏留空直接归档，不伪造通过）。
func firstSignStatus(inspectorIDs, supervisorIDs, managerIDs types.IDArray) string {
	switch {
	case len(inspectorIDs) > 0:
		return model.StatusPendingInspector
	case len(supervisorIDs) > 0:
		return model.StatusPendingSupervisor
	case len(managerIDs) > 0:
		return model.StatusPendingManager
	default:
		return model.StatusApproved
	}
}

// resolveSigners 生成报告时的签字人名单（名单制授权，设计方案 §5.3）：
// picked 为 nil 时取槽位默认名单——duty_binding 槽位绑定（项目级覆盖 → 平台默认）→ 绑定岗位在该项目编制内的在职启用成员；
// 槽位无绑定或无人员 → 该级为空（跳过）；显式名单仅校验用户存在且启用（授权以报告名单为准，不限角色/权限点/岗位，去重）；
// picked 为空数组表示该级跳过（与 nil 区分）。
func (s *ReportService) resolveSigners(communityID, slot string, picked []string) (types.IDArray, *errs.Error) {
	if picked == nil {
		return communitysvc.SlotUserIDs(s.db, communityID, slot), nil
	}
	if len(picked) == 0 {
		return types.IDArray{}, nil
	}
	enabled := map[string]bool{}
	var valid []string
	tenantID := middleware.CommunityTenantID(s.db, communityID)
	if tenantID == nil {
		return nil, errs.ErrCommunityNotExist
	}
	if err := s.db.Model(&sysmodel.SysUser{}).
		Where("id IN ? AND tenant_id = ? AND status = ?", picked, *tenantID, sysmodel.StatusEnabled).
		Pluck("id", &valid).Error; err != nil {
		return nil, errs.ErrInternal
	}
	for _, id := range valid {
		enabled[id] = true
	}
	ids := make(types.IDArray, 0, len(picked))
	seen := map[string]bool{}
	for _, id := range picked {
		if !enabled[id] {
			return nil, errs.ErrParam.WithMsg("签字人须为存在且启用的用户")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// SignCandidates 生成报告弹窗的签字人选项（名单制授权）：
// default_supervisor_ids/default_manager_ids 为槽位默认名单（项目级覆盖 → 平台默认绑定 → 编制在职成员）；
// patrolType 非空时主管级默认名单按该类型汇报线槽位取（同 Generate，§3.4）；
// users 为全部启用用户（供生成时手动增删调整，授权以报告名单为准，不限岗位/权限点）。
func (s *ReportService) SignCandidates(c *gin.Context, communityID, patrolType string) (gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	if patrolType != "" && !s.validPatrolType(patrolType) {
		return nil, errs.ErrParam.WithMsg("patrol_type 取值非法（须为字典 patrol_type 的启用项）")
	}
	supSlot := supervisorSlot(patrolType, func(t string) string {
		return communitysvc.ResolveReportLineSlot(s.db, communityID, t)
	})
	defaultSupervisorIDs := communitysvc.SlotUserIDs(s.db, communityID, supSlot)
	defaultManagerIDs := communitysvc.SlotUserIDs(s.db, communityID, sysmodel.SlotReportSignManager)
	var users []sysmodel.SysUser
	tenantID := middleware.CommunityTenantID(s.db, communityID)
	if tenantID == nil {
		return nil, errs.ErrCommunityNotExist
	}
	if err := s.db.Select("id", "name").
		Where("tenant_id = ? AND status = ?", *tenantID, sysmodel.StatusEnabled).Order("id ASC").Find(&users).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{
		"users":                  s.candidateItems(users),
		"default_supervisor_ids": defaultSupervisorIDs,
		"default_manager_ids":    defaultManagerIDs,
	}, nil
}

// candidateItems 候选人 → 前端选项（批量查 active 签名资产，避免逐人查询）。
func (s *ReportService) candidateItems(users []sysmodel.SysUser) []gin.H {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	signedSet := map[string]bool{}
	if len(ids) > 0 {
		var ownerIDs []string
		s.db.Model(&sysmodel.SignAsset{}).
			Where("asset_type = ? AND status = ? AND owner_id IN ?",
				sysmodel.SignAssetTypeUserSignature, sysmodel.SignAssetStatusActive, ids).
			Pluck("owner_id", &ownerIDs)
		for _, id := range ownerIDs {
			signedSet[id] = true
		}
	}
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{"id": u.ID, "name": u.Name, "has_signature": signedSet[u.ID]})
	}
	return items
}

// SignInspector 巡检员电子确认：inspector_signed 追加留痕；全员签完流转 pending_supervisor 并通知主管。
// 代签（req.ProxyFor 非空）：须 report:sign:proxy 权限 + 代签原因必填；留痕记录被代签人，签名图取代签人本人资产（代签人对该次确认负责）。
func (s *ReportService) SignInspector(c *gin.Context, id string, req *dto.InspectorSignReq) (gin.H, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, be
	}
	if r.Status != model.StatusPendingInspector {
		return nil, errs.ErrReportStatusNotAllowed.WithMsg("当前状态不可巡检员确认")
	}
	identity := middleware.CurrentIdentity(c)

	// 确定被签人：默认本人；proxy_for 非空走代签
	targetUID, targetName := identity.UserID, identity.Name
	var proxy *types.SignEntry
	if req.ProxyFor != "" {
		if !identity.SuperAdmin {
			ok, err := authz.EnforceAny(identity.UserID, "report:sign:proxy")
			if err != nil || !ok {
				return nil, errs.ErrNoPerm
			}
		}
		if !r.InspectorIDs.Contains(req.ProxyFor) {
			return nil, errs.ErrParam.WithMsg("被代签人不在应签巡检员名单内")
		}
		if req.Reason == "" {
			return nil, errs.ErrParam.WithMsg("代签须填写代签原因")
		}
		targetUID = req.ProxyFor
		targetName = s.userName(req.ProxyFor)
		proxy = &types.SignEntry{ProxyBy: identity.UserID, ProxyName: identity.Name, ProxyReason: req.Reason}
	} else if !r.InspectorIDs.Contains(identity.UserID) {
		return nil, errs.ErrReportNotInspector
	}
	for _, e := range r.InspectorSigned {
		if e.UserID == targetUID {
			return nil, errs.ErrReportAlreadySigned
		}
	}
	// 签名图：本人签用本人资产；代签用代签人资产。未配置签名时允许随请求传一次性签名文件
	sigUID := identity.UserID
	if proxy == nil {
		sigUID = targetUID
	}
	sigKey, sigAssetID, sbe := s.resolveSignKey(sigUID, req.SignatureFileID)
	if sbe != nil {
		return nil, sbe
	}
	entry := types.SignEntry{
		UserID: targetUID, Name: targetName, SignedAt: timefmt.T(time.Now()),
		SignatureKey: sigKey, AssetID: sigAssetID, // 签名图+资产快照：防止后续换签名影响历史报告
	}
	if proxy != nil {
		entry.ProxyBy, entry.ProxyName, entry.ProxyReason = proxy.ProxyBy, proxy.ProxyName, proxy.ProxyReason
	}
	signed := append(types.SignArray{}, r.InspectorSigned...)
	signed = append(signed, entry)
	// 全部应签巡检员均已确认
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
		// 定向通知指定主管签字人（无签字人时该级本已跳过，不会走到这里仍保底校验）
		for _, uid := range r.SupervisorIDs {
			s.notify(uid, "report", "月报待主管审批",
				fmt.Sprintf("「%s」全体巡检员已确认，请安全主管审批", r.Title), &r.ID)
		}
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

	// 指定签字人校验：主管/经理两级须在生成时圈定的名单内（超管不豁免，驳回同样受限）
	if expectStatus == model.StatusPendingSupervisor && !r.SupervisorIDs.Contains(identity.UserID) {
		return nil, errs.ErrReportNotSigner.WithMsg("你不在本报告安全主管签字人名单内")
	}
	if expectStatus == model.StatusPendingManager && !r.ManagerIDs.Contains(identity.UserID) {
		return nil, errs.ErrReportNotSigner.WithMsg("你不在本报告物业经理签字人名单内")
	}

	if req.Action == "reject" {
		if req.Reason == "" {
			return nil, errs.ErrReportRejectReasonRequired
		}
		// 驳回回第一个有签字人的级别（清空巡检员确认重新签；经理驳回时同时清空主管签字痕迹，重走该级）
		signed := types.SignArray{}
		signed = append(signed, r.InspectorSigned...)
		backStatus := firstSignStatus(r.InspectorIDs, r.SupervisorIDs, r.ManagerIDs)
		if backStatus == model.StatusApproved { // 不可能驳回已归档报告，兜底
			backStatus = expectStatus
		}
		updates := map[string]any{
			"status":           backStatus,
			"inspector_signed": types.SignArray{},
			"reject_reason":    req.Reason,
			"supervisor_by":    nil, "supervisor_at": nil, "supervisor_remark": "",
			"supervisor_signature_key": "",
		}
		if err := s.db.Model(r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		// 通知已签巡检员重新确认
		for _, e := range signed {
			s.notify(e.UserID, "report", "月报被驳回",
				fmt.Sprintf("「%s」被驳回：%s，请重新确认", r.Title, req.Reason), &r.ID)
		}
		return gin.H{"status": backStatus}, nil
	}

	// 手写签名前置：优先已配置签名，否则用随请求提交的一次性签名文件；都没有则不允许通过签字
	sigKey, _, sbe := s.resolveSignKey(identity.UserID, req.SignatureFileID)
	if sbe != nil {
		return nil, sbe
	}

	switch expectStatus {
	case model.StatusPendingSupervisor:
		updates := map[string]any{
			"supervisor_by": identity.UserID, "supervisor_at": now, "supervisor_remark": req.Remark,
			"supervisor_signature_key": sigKey,
		}
		// 经理级有签字人 → 流转终审并定向通知；无签字人 → 跳过终审直接归档（签字栏留空）
		if len(r.ManagerIDs) > 0 {
			updates["status"] = model.StatusPendingManager
		} else {
			updates["status"] = model.StatusApproved
			updates["seal_file_key"] = s.activeSealKey(r.CommunityID)
		}
		if err := s.db.Model(r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		if len(r.ManagerIDs) > 0 {
			for _, uid := range r.ManagerIDs {
				s.notify(uid, "report", "月报待经理终审",
					fmt.Sprintf("「%s」主管已审批通过，请物业经理终审", r.Title), &r.ID)
			}
			return gin.H{"status": model.StatusPendingManager}, nil
		}
		go s.archivePDF(r.ID)
		return gin.H{"status": model.StatusApproved}, nil
	default: // pending_manager → approved
		updates := map[string]any{
			"status":     model.StatusApproved,
			"manager_by": identity.UserID, "manager_at": now, "manager_remark": req.Remark,
			"manager_signature_key": sigKey,
			// 公章快照：固化终审时点的 active 公章（按报告小区所属租户取章），后续换章不影响已归档报告
			"seal_file_key": s.activeSealKey(r.CommunityID),
		}
		if err := s.db.Model(r).Updates(updates).Error; err != nil {
			return nil, errs.ErrInternal
		}
		go s.archivePDF(r.ID)
		return gin.H{"status": model.StatusApproved}, nil
	}
}

// PDF 返回报告 PDF 字节与文件名；file_key 已归档则直接读归档文件，否则即时生成临时版。
// 访问控制（路由层不再挂权限点，由此处统一判定）：持 report:download 权限，
// 或为报告相关人（应签巡检员/指定主管/经理签字人）——巡检员有权查看自己参与的完整报告。
func (s *ReportService) PDF(c *gin.Context, id string) ([]byte, string, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return nil, "", be
	}
	if be := s.checkPDFAccess(c, r); be != nil {
		return nil, "", be
	}
	return s.pdfBytes(r)
}

// checkPDFAccess PDF 访问判定：超管 / report:download 权限点 / 报告相关人（三级签字名单任一）。
func (s *ReportService) checkPDFAccess(c *gin.Context, r *model.InspectionReport) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	if identity.SuperAdmin {
		return nil
	}
	if ok, err := authz.EnforceAny(identity.UserID, "report:download"); err == nil && ok {
		return nil
	}
	uid := identity.UserID
	if r.InspectorIDs.Contains(uid) || r.SupervisorIDs.Contains(uid) || r.ManagerIDs.Contains(uid) {
		return nil
	}
	return errs.ErrNoPerm.WithMsg("仅报告相关人或有下载权限的账号可查看报告")
}

// pdfBytes 取报告 PDF 字节：已归档读归档文件（按驱动读取，local 读盘 / 云存储 HTTP），否则即时生成临时版。
func (s *ReportService) pdfBytes(r *model.InspectionReport) ([]byte, string, *errs.Error) {
	filename := r.Title + ".pdf"
	if r.FileKey != "" {
		data, err := s.store.ReadFile(r.FileKey)
		if err != nil {
			logger.L.Warn("月报归档文件读取失败，回退即时生成", zap.String("report_id", r.ID), zap.Error(err))
		} else {
			return data, filename, nil
		}
	}
	data, err := pdf.GenerateMonthly(s.pdfData(r))
	if err != nil {
		logger.L.Warn("月报 PDF 即时生成失败", zap.Error(err), zap.String("report_id", r.ID))
		return nil, "", errs.ErrInternal
	}
	return data, filename, nil
}

// PDFTicketTTL PDF 预览 ticket 有效期（有效期内可重复取：pdf.js 可能对大文件分段拉取）。
const PDFTicketTTL = 5 * time.Minute

// PDFTicket 签发报告 PDF 预览 ticket（App web-view 无法带 Authorization 头，凭 ticket 走公开通道）。
// 访问判定与 PDF 接口一致（report:download 或报告相关人 + 小区数据范围）。
func (s *ReportService) PDFTicket(c *gin.Context, id string) (string, *errs.Error) {
	r, be := s.getWithScope(c, id)
	if be != nil {
		return "", be
	}
	if be := s.checkPDFAccess(c, r); be != nil {
		return "", be
	}
	ticket := uuid.NewString()
	if err := s.rdb.Set(c.Request.Context(), "pdftkt:"+ticket, r.ID, PDFTicketTTL).Err(); err != nil {
		return "", errs.ErrInternal
	}
	return ticket, nil
}

// PDFByTicket 凭 ticket 取报告 PDF（免登录公开通道；ticket 由 PDFTicket 签发，过期需重新打开）。
func (s *ReportService) PDFByTicket(c *gin.Context, id string, ticket string) ([]byte, string, *errs.Error) {
	if ticket == "" {
		return nil, "", errs.ErrUnauthorized
	}
	reportID, err := s.rdb.Get(c.Request.Context(), "pdftkt:"+ticket).Result()
	if err != nil || reportID != id {
		return nil, "", errs.ErrUnauthorized.WithMsg("预览凭证无效或已过期，请重新打开")
	}
	var r model.InspectionReport
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return nil, "", errs.ErrNotFound
	}
	return s.pdfBytes(&r)
}

// loadPhoto 按 file_key 读取照片字节与图片类型（local 读本地文件；云存储 HTTP 下载）。
func (s *ReportService) loadPhoto(fileKey string) ([]byte, string, error) {
	if f, err := uploadfile.ByRef(s.db, fileKey); err == nil {
		fileKey = f.StorageKey
	}
	var imgType string
	switch strings.ToLower(filepath.Ext(fileKey)) {
	case ".jpg", ".jpeg":
		imgType = "JPG"
	case ".png":
		imgType = "PNG"
	default:
		return nil, "", fmt.Errorf("不支持的照片格式: %s", fileKey)
	}
	if s.store.IsLocal() {
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
// 供模板升级后人工刷新存量报告：local 覆盖 file_key 对应本地文件；云存储重新保存并回写 file_key。
func (s *ReportService) RebuildPDF(reportID string) error {
	var r model.InspectionReport
	if err := s.db.First(&r, "id = ?", reportID).Error; err != nil {
		return err
	}
	data, err := pdf.GenerateMonthly(s.pdfData(&r))
	if err != nil {
		return err
	}
	if s.store.IsLocal() && r.FileKey != "" {
		if err := os.WriteFile(s.store.LocalPath(r.FileKey), data, 0o644); err != nil {
			return err
		}
		logger.L.Info("月报 PDF 重渲染完成", zap.String("report_id", reportID), zap.String("file_key", r.FileKey))
		return nil
	}
	key, url, err := s.store.SaveGenerated("reports", r.Title+".pdf", data)
	if err != nil {
		return err
	}
	systemsvc.RegisterGeneratedFile(s.db, s.store, r.Title+".pdf", "application/pdf", storage.MD5Hex(data), key, url)
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
	key, url, err := s.store.SaveGenerated("reports", r.Title+".pdf", data)
	if err != nil {
		logger.L.Warn("月报归档：保存失败", zap.Error(err), zap.String("report_id", reportID))
		return
	}
	systemsvc.RegisterGeneratedFile(s.db, s.store, r.Title+".pdf", "application/pdf", storage.MD5Hex(data), key, url)
	if err := s.db.Model(&r).Update("file_key", key).Error; err != nil {
		logger.L.Warn("月报归档：回写 file_key 失败", zap.Error(err), zap.String("report_id", reportID))
		return
	}
	logger.L.Info("月报 PDF 归档完成", zap.String("report_id", reportID), zap.String("file_key", key))
}

// notify 写站内消息 + App 推送（统一走 notify.Notifier，仿 OrderService.Notify）。
func (s *ReportService) notify(userID, msgType, title, content string, bizID *string) {
	_ = s.notifier.Send(userID, msgType, title, content, bizID)
}

// getWithScope 取报告并做数据权限校验（self 档用户放宽到本人相关报告：三级签字名单任一含本人）。
func (s *ReportService) getWithScope(c *gin.Context, id string) (*model.InspectionReport, *errs.Error) {
	var r model.InspectionReport
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, r.CommunityID); be != nil {
		if identity := middleware.CurrentIdentity(c); identity != nil && identity.ScopeSelf {
			uid := identity.UserID
			if r.InspectorIDs.Contains(uid) || r.SupervisorIDs.Contains(uid) || r.ManagerIDs.Contains(uid) {
				return &r, nil
			}
		}
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

// validPatrolType 巡查类型字典驱动校验（同 plan_service 写法）：
// 字典 patrol_type 存在该值时以启用状态为准；字典无此值（seed 未跑/新库初始化顺序）回落内置常量校验。
func (s *ReportService) validPatrolType(t string) bool {
	var status string
	err := s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = ? AND value = ?", "patrol_type", t).
		Limit(1).Pluck("status", &status).Error
	if err != nil || status == "" {
		return insmodel.ValidPatrolType(t)
	}
	return status == sysmodel.StatusEnabled
}

// patrolTypeLabel 巡查类型中文名：空=综合月报；非空取字典 patrol_type 的 label，查不到回落原值。
func (s *ReportService) patrolTypeLabel(t string) string {
	if t == "" {
		return "综合月报"
	}
	var label string
	if err := s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = ? AND value = ?", "patrol_type", t).
		Limit(1).Pluck("label", &label).Error; err == nil && label != "" {
		return label
	}
	return t
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

// signerItems 指定签字人名单 → 明细（补姓名与签署状态；signedBy 为实际签署人，任一签署即该级完成）。
func (s *ReportService) signerItems(ids types.IDArray, signedBy *string) []gin.H {
	items := make([]gin.H, 0, len(ids))
	for _, uid := range ids {
		items = append(items, gin.H{
			"user_id": uid, "name": s.userName(uid),
			"signed": signedBy != nil && *signedBy == uid,
		})
	}
	return items
}

// resolveSignKey 签字签名图：优先当前 active 签名资产（返回资产快照，防后续换签名影响历史报告）；
// 未配置时允许随请求传一次性签名文件（须为本人 scene=signature 上传，仅本次签字快照用，不写入签章资产）。
func (s *ReportService) resolveSignKey(uid, reqKey string) (sigKey, assetID string, be *errs.Error) {
	sigKey, assetID = s.signatureAssetOf(uid)
	if sigKey == "" && reqKey != "" {
		var cnt int64
		s.db.Model(&sysmodel.UploadFile{}).
			Where("id = ? AND scene = ? AND user_id = ?", reqKey, "signature", uid).Count(&cnt)
		if cnt > 0 {
			return reqKey, "", nil
		}
		return "", "", errs.ErrParam.WithMsg("签名文件无效，请重新手写签名")
	}
	if sigKey == "" {
		return "", "", errs.ErrSignatureMissing
	}
	return sigKey, assetID, nil
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

// activeSealKey 报告所属租户当前 active 公章资产 file_key（无则空串）。
// 租户按报告归属小区解析（CommunityTenantID）：私有化下与操作者同租户，SaaS 下必须盖报告归属公司的章。
func (s *ReportService) activeSealKey(communityID string) string {
	tenantID := middleware.CommunityTenantID(s.db, communityID)
	if tenantID == nil {
		return ""
	}
	var a sysmodel.SignAsset
	if err := s.db.Select("file_key").
		Where("tenant_id = ? AND asset_type = ? AND status = ?", *tenantID, sysmodel.SignAssetTypeCompanySeal, sysmodel.SignAssetStatusActive).
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
