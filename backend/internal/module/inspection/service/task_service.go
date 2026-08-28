package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// TaskService 任务监控与打卡记录检索服务（管理后台）。
type TaskService struct {
	db       *gorm.DB
	store    *storage.Storage
	notifier *notify.Notifier
}

func NewTaskService(db *gorm.DB, store *storage.Storage, notifier *notify.Notifier) *TaskService {
	return &TaskService{db: db, store: store, notifier: notifier}
}

// taskCounters 任务异常/疑似计数。
type taskCounters struct {
	abnormal int64
	suspect  int64
}

// List 任务监控列表（数据权限过滤 + 状态 Tab 扩展筛选）。
func (s *TaskService) List(c *gin.Context, q *dto.TaskListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.InspectionTask{})
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.InspectorID != "" {
		db = db.Where("inspector_id = ?", q.InspectorID)
	}
	if q.PlanID != "" {
		db = db.Where("plan_id = ?", q.PlanID)
	}
	if q.PatrolType != "" {
		db = db.Where("patrol_type = ?", q.PatrolType)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 日期：task_date 与范围二选一，默认今天
	if q.TaskDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", q.TaskDate, time.Local); err != nil {
			return nil, errs.ErrParam.WithMsg("task_date 格式应为 YYYY-MM-DD")
		}
		db = db.Where("task_date = ?", q.TaskDate)
	} else if q.StartDate != "" || q.EndDate != "" {
		if q.StartDate != "" {
			db = db.Where("task_date >= ?", q.StartDate)
		}
		if q.EndDate != "" {
			db = db.Where("task_date <= ?", q.EndDate)
		}
	} else {
		db = db.Where("task_date = ?", time.Now().Format("2006-01-02"))
	}
	// Tab 扩展筛选
	switch q.Filter {
	case "missing":
		db = db.Where("done_points < total_points AND status <> ?", model.TaskPending)
	case "abnormal":
		db = db.Where("id IN (?)", s.db.Model(&model.CheckinRecord{}).
			Select("task_id").Where("result = ? AND superseded_by IS NULL", model.ResultAbnormal))
	case "suspect":
		db = db.Where("id IN (?)", s.db.Model(&model.CheckinRecord{}).
			Select("task_id").Where("is_suspect AND superseded_by IS NULL"))
	}
	db = middleware.ApplyCommunityFilter(db, c, "inspection_task.community_id")
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var tasks []model.InspectionTask
	offset, limit := q.Normalize()
	if err := db.Order("task_date DESC, id DESC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	counters := s.loadCounters(tasks)
	list := make([]gin.H, 0, len(tasks))
	for i := range tasks {
		list = append(list, s.toItem(&tasks[i], counters[tasks[i].ID]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// loadCounters 批量统计任务异常/疑似数。
func (s *TaskService) loadCounters(tasks []model.InspectionTask) map[string]taskCounters {
	out := map[string]taskCounters{}
	if len(tasks) == 0 {
		return out
	}
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	type row struct {
		TaskID   string
		Abnormal int64
		Suspect  int64
	}
	var rows []row
	s.db.Model(&model.CheckinRecord{}).
		Select("task_id, COUNT(*) FILTER (WHERE result = 'abnormal') AS abnormal, COUNT(*) FILTER (WHERE is_suspect) AS suspect").
		Where("task_id IN ? AND superseded_by IS NULL", ids).Group("task_id").Scan(&rows)
	for _, r := range rows {
		out[r.TaskID] = taskCounters{abnormal: r.Abnormal, suspect: r.Suspect}
	}
	return out
}

func (s *TaskService) toItem(t *model.InspectionTask, cnt taskCounters) gin.H {
	var plan model.InspectionPlan
	planName, timeWindow := "", t.TimeWindow // 时段取任务快照优先（轮次任务），空回落计划
	// Unscoped：计划已删除时仍需展示其名称（历史任务可追溯），标注「已删除」
	if s.db.Unscoped().Select("name", "time_window", "deleted_at").First(&plan, "id = ?", t.PlanID).Error == nil {
		planName = plan.Name
		if timeWindow == "" {
			timeWindow = plan.TimeWindow
		}
		if plan.DeletedAt.Valid {
			planName += "（已删除）"
		}
	}
	commName, inspectorName := "", ""
	var comm sysmodel.Community
	if s.db.Select("name").First(&comm, "id = ?", t.CommunityID).Error == nil {
		commName = comm.Name
	}
	var u sysmodel.SysUser
	if s.db.Select("name").First(&u, "id = ?", t.InspectorID).Error == nil {
		inspectorName = u.Name
	}
	progress := 0
	if t.TotalPoints > 0 {
		progress = t.DonePoints * 100 / t.TotalPoints
	}
	return gin.H{
		"id": t.ID, "plan_id": t.PlanID, "plan_name": planName,
		"community_id": t.CommunityID, "community_name": commName,
		"inspector_id": t.InspectorID, "inspector_name": inspectorName,
		"patrol_type": t.PatrolType,
		"task_date": t.TaskDate.Format("2006-01-02"), "time_window": timeWindow,
		"round_name": t.RoundName,
		"status": t.Status, "total_points": t.TotalPoints, "done_points": t.DonePoints,
		"progress": progress, "abnormal_count": cnt.abnormal, "suspect_count": cnt.suspect,
		"missing_count": t.TotalPoints - t.DonePoints,
		"started_at": timefmt.TP(t.StartedAt), "finished_at": timefmt.TP(t.FinishedAt),
	}
}

// Detail 任务明细：按路线顺序返回每点位打卡状态（支撑任务监控页）。
func (s *TaskService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var t model.InspectionTask
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, t.CommunityID); be != nil {
		return nil, be
	}
	// 计划可能已删除（删计划只级联清理未开始任务）：Unscoped 读取保留名称与路线，保证历史任务可查
	var plan model.InspectionPlan
	if err := s.db.Unscoped().First(&plan, "id = ?", t.PlanID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	planName := plan.Name
	if plan.DeletedAt.Valid {
		planName += "（已删除）"
	}
	// 任务下全部打卡记录，按点位归集（过滤已被覆盖的旧记录）
	var checkins []model.CheckinRecord
	s.db.Where("task_id = ? AND superseded_by IS NULL", t.ID).Find(&checkins)
	byPoint := map[string]*model.CheckinRecord{}
	for i := range checkins {
		byPoint[checkins[i].PointID] = &checkins[i]
	}
	// 任务点位名单：任务快照优先（by_point_types 圈选/轮次任务均落快照），空回落计划名单（存量任务）
	pointIDs := model.TaskPointIDs(&t, &plan)
	// 项级进度派生：过程草稿按点位聚合（done=已有结论[含手动项]，recognizing=AI 识别中，failed=识别失败待重拍）；
	// 总项数取点位模板当前项数（观测层数据，模板中途变更只影响展示，不影响正式记录）
	var drafts []model.CheckinItemDraft
	s.db.Select("point_id", "ai_status").Where("task_id = ?", t.ID).Find(&drafts)
	type draftAgg struct{ done, recognizing, failed int }
	aggByPoint := map[string]*draftAgg{}
	for _, d := range drafts {
		a := aggByPoint[d.PointID]
		if a == nil {
			a = &draftAgg{}
			aggByPoint[d.PointID] = a
		}
		switch d.AIStatus {
		case model.ItemDraftPending:
			a.recognizing++
		case model.ItemDraftFailed:
			a.failed++
		default:
			a.done++
		}
	}
	// 全量状态聚合（汇总条数据源，不随分页变化）
	stats := gin.H{"total": len(pointIDs), "done": 0, "doing": 0, "pending": 0, "normal": 0, "abnormal": 0, "suspect": 0}
	for _, pid := range pointIDs {
		if ck, ok := byPoint[pid]; ok {
			stats["done"] = stats["done"].(int) + 1
			switch ck.Result {
			case model.ResultNormal:
				stats["normal"] = stats["normal"].(int) + 1
			case model.ResultAbnormal:
				stats["abnormal"] = stats["abnormal"].(int) + 1
			}
			if ck.IsSuspect {
				stats["suspect"] = stats["suspect"].(int) + 1
			}
		} else if aggByPoint[pid] != nil {
			stats["doing"] = stats["doing"].(int) + 1
		} else {
			stats["pending"] = stats["pending"].(int) + 1
		}
	}
	// 点位时间线分页：单任务可达数百点位，全量序列化会拖垮详情页
	page, pageSize := 1, 50
	if v, err := strconv.Atoi(c.Query("points_page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("points_page_size")); err == nil && v > 0 && v <= 200 {
		pageSize = v
	}
	total := len(pointIDs)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageIDs := pointIDs[start:end]
	// 本页点位/楼栋/模板项数批量加载（避免逐点位 N+1 查询）
	var pagePoints []model.InspectionPoint
	s.db.Where("id IN ?", pageIDs).Find(&pagePoints)
	ptByID := make(map[string]*model.InspectionPoint, len(pagePoints))
	buildingIDSet, tplIDSet := map[string]bool{}, map[string]bool{}
	for i := range pagePoints {
		ptByID[pagePoints[i].ID] = &pagePoints[i]
		if pagePoints[i].BuildingID != nil {
			buildingIDSet[*pagePoints[i].BuildingID] = true
		}
		if pagePoints[i].TemplateID != nil && *pagePoints[i].TemplateID != "" {
			tplIDSet[*pagePoints[i].TemplateID] = true
		}
	}
	buildingNames := map[string]string{}
	if len(buildingIDSet) > 0 {
		bIDs := make([]string, 0, len(buildingIDSet))
		for id := range buildingIDSet {
			bIDs = append(bIDs, id)
		}
		var buildings []model.Building
		s.db.Select("id", "name").Where("id IN ?", bIDs).Find(&buildings)
		for _, b := range buildings {
			buildingNames[b.ID] = b.Name
		}
	}
	itemTotals := map[string]int{}
	if len(tplIDSet) > 0 {
		tIDs := make([]string, 0, len(tplIDSet))
		for id := range tplIDSet {
			tIDs = append(tIDs, id)
		}
		type tplCnt struct {
			TemplateID string
			Cnt        int
		}
		var cnts []tplCnt
		s.db.Model(&model.CheckTemplateItem{}).Select("template_id, COUNT(*) AS cnt").
			Where("template_id IN ?", tIDs).Group("template_id").Scan(&cnts)
		for _, r := range cnts {
			itemTotals[r.TemplateID] = r.Cnt
		}
	}
	points := make([]gin.H, 0, len(pageIDs))
	for i, pid := range pageIDs {
		pt, ok := ptByID[pid]
		if !ok {
			continue
		}
		buildingName := ""
		if pt.BuildingID != nil {
			buildingName = buildingNames[*pt.BuildingID]
		}
		itemTotal := 0
		if pt.TemplateID != nil {
			itemTotal = itemTotals[*pt.TemplateID]
		}
		entry := gin.H{
			"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
			"sort": start + i + 1, "credential": pt.Credential, "require_fence": pt.RequireFence,
			"status": "pending", "checkin": nil, "item_total": itemTotal,
		}
		if ck, ok2 := byPoint[pid]; ok2 {
			entry["status"] = "done"
			entry["checkin"] = checkinBrief(s.db, ck)
		} else if a := aggByPoint[pid]; a != nil {
			// 巡检中：有过程草稿但尚未提交
			entry["status"] = "doing"
			entry["item_done"] = a.done
			entry["item_recognizing"] = a.recognizing
			entry["item_failed"] = a.failed
		}
		points = append(points, entry)
	}
	progress := 0
	if t.TotalPoints > 0 {
		progress = t.DonePoints * 100 / t.TotalPoints
	}
	commName, inspectorName := "", ""
	var comm sysmodel.Community
	if s.db.Select("name").First(&comm, "id = ?", t.CommunityID).Error == nil {
		commName = comm.Name
	}
	var u sysmodel.SysUser
	if s.db.Select("name").First(&u, "id = ?", t.InspectorID).Error == nil {
		inspectorName = u.Name
	}
	return gin.H{
		"task": gin.H{
			"id": t.ID, "plan_name": planName, "community_name": commName,
			"inspector_id": t.InspectorID, "inspector_name": inspectorName,
			"patrol_type": t.PatrolType,
			"task_date": t.TaskDate.Format("2006-01-02"), "time_window": model.TaskTimeWindow(&t, &plan),
			"round_name": t.RoundName,
			"status": t.Status, "total_points": t.TotalPoints, "done_points": t.DonePoints,
			"progress": progress, "started_at": timefmt.TP(t.StartedAt), "finished_at": timefmt.TP(t.FinishedAt),
		},
		"points":       points,
		"points_total": total,
		"points_page":  page,
		"points_size":  pageSize,
		"stats":        stats,
	}, nil
}

// checkinBrief 打卡摘要（任务明细内嵌）。
func checkinBrief(db *gorm.DB, ck *model.CheckinRecord) gin.H {
	distance := any(nil)
	if ck.DistanceToPoint != nil {
		distance = int(*ck.DistanceToPoint)
	}
	return gin.H{
		"id": ck.ID, "checkin_time": timefmt.T(ck.CheckinTime), "client_time": timefmt.TP(ck.ClientTime),
		"checkin_type": ck.CheckinType, "distance_to_point": distance,
		"longitude": ck.Longitude, "latitude": ck.Latitude,
		"result": ck.Result, "is_suspect": ck.IsSuspect, "suspect_reason": ck.SuspectReason,
		"remark": ck.Remark, "photos": RecordFlatPhotos(db, ck.ID),
		"check_items": briefItemViews(db, ck.ID),
		"audit_status": ck.AuditStatus, "audit_by": ck.AuditBy,
		"audit_at": timefmt.TP(ck.AuditAt), "audit_remark": ck.AuditRemark,
		"ai_verdict": ck.AIVerdict, "ai_reason": ck.AIReason,
	}
}

// briefItemViews 任务明细内嵌的逐项结果（photos 为 file_key，结构同旧 JSONB 快照；requirement 可空）。
func briefItemViews(db *gorm.DB, recordID string) []gin.H {
	var items []model.CheckinRecordItem
	db.Where("record_id = ?", recordID).Order("sort ASC").Find(&items)
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"name": it.Name, "pass": it.Pass, "note": it.Note,
			"photos": it.Photos, "requirement": it.Requirement,
		})
	}
	return out
}

// ========== 打卡记录检索（管理后台） ==========

// applyCheckinFilters 打卡记录检索的公共过滤条件（含小区数据权限，不含审核状态）。
// 固定过滤 superseded_by 非空的旧记录（覆盖修改后仅最新记录参与列表/计数）。
func (s *TaskService) applyCheckinFilters(c *gin.Context, q *dto.CheckinListQuery) (*gorm.DB, *errs.Error) {
	db := s.db.Model(&model.CheckinRecord{}).Where("superseded_by IS NULL")
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.PointID != "" {
		db = db.Where("point_id = ?", q.PointID)
	}
	if q.InspectorID != "" {
		db = db.Where("inspector_id = ?", q.InspectorID)
	}
	if q.TaskID != "" {
		db = db.Where("task_id = ?", q.TaskID)
	}
	if q.Result != "" {
		db = db.Where("result = ?", q.Result)
	}
	if q.CheckinType != "" {
		db = db.Where("checkin_type = ?", q.CheckinType)
	}
	if v, ok, _ := bind.BoolFilter(q.IsSuspect); ok {
		db = db.Where("is_suspect = ?", v)
	}
	if v, ok, _ := bind.BoolFilter(q.ForceSubmit); ok {
		db = db.Where("force_submit = ?", v)
	}
	if q.AIVerdict != "" {
		// 支持逗号多值（如 review,error 查"AI 存疑"合集）
		if verdicts := strings.Split(q.AIVerdict, ","); len(verdicts) > 1 {
			db = db.Where("ai_verdict IN ?", verdicts)
		} else {
			db = db.Where("ai_verdict = ?", q.AIVerdict)
		}
	}
	var be *errs.Error
	if db, be = timeRangeOn(db, "checkin_time", q.StartTime, q.EndTime); be != nil {
		return nil, be
	}
	return middleware.ApplyCommunityFilter(db, c, "checkin_record.community_id"), nil
}

// CheckinAuditCounts 各审核状态计数（列表页 tab 徽章；与列表共用过滤条件，不含 audit_status 本身）。
func (s *TaskService) CheckinAuditCounts(c *gin.Context, q *dto.CheckinListQuery) (gin.H, *errs.Error) {
	db, be := s.applyCheckinFilters(c, q)
	if be != nil {
		return nil, be
	}
	var rows []struct {
		Status string
		Cnt    int64
	}
	if err := db.Session(&gorm.Session{}).Select("audit_status AS status, count(*) AS cnt").
		Group("audit_status").Scan(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := gin.H{"auto_pass": int64(0), "pending": int64(0), "pass": int64(0), "rejected": int64(0)}
	for _, r := range rows {
		out[r.Status] = r.Cnt
	}
	return out, nil
}

// CheckinList 打卡记录分页检索。
func (s *TaskService) CheckinList(c *gin.Context, q *dto.CheckinListQuery) (*response.Page, *errs.Error) {
	db, be := s.applyCheckinFilters(c, q)
	if be != nil {
		return nil, be
	}
	if q.AuditStatus != "" {
		// 支持逗号多值（如 pass,rejected 查"已审核"合集）
		if statuses := strings.Split(q.AuditStatus, ","); len(statuses) > 1 {
			db = db.Where("audit_status IN ?", statuses)
		} else {
			db = db.Where("audit_status = ?", q.AuditStatus)
		}
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
	flows := map[string]types.FlowStepArray{} // community_id → 打卡审核链（当前环节名展示用）
	for i := range rows {
		r := &rows[i]
		list = append(list, gin.H{
			"id": r.ID, "task_id": r.TaskID, "point_id": r.PointID,
			"point_name":     pointName(s.db, r.PointID),
			"community_name": commName(s.db, r.CommunityID),
			"inspector_id": r.InspectorID, "inspector_name": userName(s.db, r.InspectorID),
			"checkin_time": timefmt.T(r.CheckinTime), "checkin_type": r.CheckinType,
			"distance_to_point": distanceOrNil(r), "result": r.Result,
			"is_suspect": r.IsSuspect, "photo_count": RecordPhotoCount(s.db, r.ID),
			"audit_status": r.AuditStatus, "audit_step": r.AuditStep,
			"current_step_name": s.currentStepName(flows, r),
			"ai_verdict":        r.AIVerdict, "ai_reason": r.AIReason,
			"force_submit":      r.ForceSubmit,
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// currentStepName 待审核记录的当前审批环节名（pending 才有值；供列表"待审核（环节名）"展示）。
func (s *TaskService) currentStepName(flows map[string]types.FlowStepArray, r *model.CheckinRecord) string {
	if r.AuditStatus != model.AuditPending {
		return ""
	}
	flow, ok := flows[r.CommunityID]
	if !ok {
		flow = communitysvc.ResolveFlow(s.db, r.CommunityID, sysmodel.FlowCheckinReview)
		flows[r.CommunityID] = flow
	}
	if idx := int(r.AuditStep); idx < len(flow) {
		return flow[idx].Name
	}
	return ""
}

// CheckinDetail 打卡记录详情。
func (s *TaskService) CheckinDetail(c *gin.Context, id string) (gin.H, *errs.Error) {
	// 分区表按 id 查询带宽时间范围即可（此处全分区扫描由主键序列保证唯一）
	var r model.CheckinRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, r.CommunityID); be != nil {
		return nil, be
	}
	planName := ""
	var t model.InspectionTask
	if s.db.Select("plan_id").First(&t, "id = ?", r.TaskID).Error == nil {
		var plan model.InspectionPlan
		if s.db.Select("name").First(&plan, "id = ?", t.PlanID).Error == nil {
			planName = plan.Name
		}
	}
	// 照片带 EXIF 校验结论（v21 起从逐项照片聚合）
	flatPhotos := RecordFlatPhotos(s.db, r.ID)
	photos := make([]gin.H, 0, len(flatPhotos))
	for _, p := range flatPhotos {
		exif, _ := p["exif_time"].(string)
		photos = append(photos, gin.H{
			"item": p["item"], "url": p["url"], "watermarked_url": p["watermarked_url"],
			"exif_check": exifCheck(&r, exif),
		})
	}
	// 逐项结果带照片 URL（photos 存 file_key；优先水印图）；v18 起读 checkin_record_item 快照表
	var recItems []model.CheckinRecordItem
	s.db.Where("record_id = ?", r.ID).Order("sort ASC").Find(&recItems)
	checkItems := make([]gin.H, 0, len(recItems))
	for _, ci := range recItems {
		checkItems = append(checkItems, gin.H{
			"name": ci.Name, "pass": ci.Pass, "note": ci.Note,
			"photos": ci.Photos, "photo_urls": ItemPhotoURLs(s.db, ci.Photos),
			"requirement": ci.Requirement, "ai_hint": ci.AIHint,
			"judge_type": ci.JudgeType, "judge_config": ci.JudgeConfig,
			"ai_verdict": ci.AIVerdict, "ai_reason": ci.AIReason, "ai_reading": ci.AIReading,
		})
	}
	return gin.H{
		"id": r.ID, "task_id": r.TaskID, "plan_name": planName,
		"point_id": r.PointID, "point_name": pointName(s.db, r.PointID),
		"community_name": commName(s.db, r.CommunityID),
		"inspector_id": r.InspectorID, "inspector_name": userName(s.db, r.InspectorID),
		"checkin_time": timefmt.T(r.CheckinTime), "client_time": timefmt.TP(r.ClientTime),
		"checkin_type": r.CheckinType, "longitude": r.Longitude, "latitude": r.Latitude,
		"distance_to_point": distanceOrNil(&r), "result": r.Result, "remark": r.Remark,
		"is_suspect": r.IsSuspect, "suspect_reason": r.SuspectReason,
		"photos": photos, "check_items": checkItems,
		"audit_status": r.AuditStatus, "audit_by": r.AuditBy, "audit_by_name": userNamePtr(s.db, r.AuditBy),
		"audit_at": timefmt.TP(r.AuditAt), "audit_remark": r.AuditRemark,
		"ai_verdict": r.AIVerdict, "ai_reason": r.AIReason,
		"ai_quality_pass": r.AIQualityPass, "ai_quality_issue": r.AIQualityIssue,
		"force_submit":    r.ForceSubmit,
		"created_at": timefmt.T(r.CreatedAt),
	}, nil
}

// exifCheck 构造照片 EXIF 校验结论（无 EXIF 时间时 passed 为 nil）。
// exifCheck 照片 EXIF 拍摄时间与打卡时间偏差校验（exifTime 为 timefmt 格式字符串，空=无 EXIF）。
func exifCheck(r *model.CheckinRecord, exifTime string) gin.H {
	if exifTime == "" {
		return gin.H{"shot_at": nil, "deviation_seconds": nil, "passed": nil}
	}
	shot, err := timefmt.Parse(exifTime)
	if err != nil {
		return gin.H{"shot_at": exifTime, "deviation_seconds": nil, "passed": nil}
	}
	dev := int(shot.Sub(r.CheckinTime).Seconds())
	if dev < 0 {
		dev = -dev
	}
	return gin.H{"shot_at": exifTime, "deviation_seconds": dev, "passed": dev <= 300}
}

func pointName(db *gorm.DB, id string) string {
	var p model.InspectionPoint
	if db.Select("name").First(&p, "id = ?", id).Error == nil {
		return p.Name
	}
	return ""
}

func commName(db *gorm.DB, id string) string {
	var c sysmodel.Community
	if db.Select("name").First(&c, "id = ?", id).Error == nil {
		return c.Name
	}
	return ""
}

func userName(db *gorm.DB, id string) string {
	var u sysmodel.SysUser
	if db.Select("name").First(&u, "id = ?", id).Error == nil {
		return u.Name
	}
	return ""
}

// userNamePtr 可空 ID 转用户名（nil 或查不到返回空串）。
func userNamePtr(db *gorm.DB, id *string) string {
	if id == nil || *id == "" {
		return ""
	}
	return userName(db, *id)
}

func distanceOrNil(r *model.CheckinRecord) any {
	if r.DistanceToPoint == nil {
		return nil
	}
	return int(*r.DistanceToPoint)
}

// Remind 任务催办：给执行人发送站内提醒（已完成任务不可催办；App 管理端一键催办）。
// 操作者须在巡查汇报线名单内（催办归口汇报线主管；超管/租户管理员默认放行）。
func (s *TaskService) Remind(c *gin.Context, id string) *errs.Error {
	var t model.InspectionTask
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, t.CommunityID); be != nil {
		return be
	}
	// 催办归口该任务巡查业务线的汇报线主管（维度槽位 → 通用槽位回落；超管/租户管理员默认放行）
	slot := communitysvc.ResolveReportLineSlot(s.db, t.CommunityID, t.PatrolType)
	if !communitysvc.SlotAuthorized(s.db, t.CommunityID, slot, middleware.CurrentIdentity(c)) {
		return errs.ErrNotInSlot.WithMsg("当前用户不在本项目该巡查业务线的审核名单内")
	}
	if t.Status == model.TaskDone {
		return errs.ErrParam.WithMsg("任务已完成，无需催办")
	}
	planName := ""
	var plan model.InspectionPlan
	if s.db.Unscoped().Select("name").First(&plan, "id = ?", t.PlanID).Error == nil {
		planName = plan.Name
	}
	if err := s.notifier.Send(t.InspectorID, "task",
		"巡检任务催办",
		fmt.Sprintf("你的巡检任务「%s」（%s）还未完成，当前进度 %d/%d 个点位，请尽快执行。", planName, t.TaskDate.Format("2006-01-02"), t.DonePoints, t.TotalPoints),
		&t.ID); err != nil {
		return errs.ErrInternal
	}
	return nil
}

// timeRangeOn 对指定字段追加时间范围过滤。
func timeRangeOn(db *gorm.DB, column, start, end string) (*gorm.DB, *errs.Error) {
	if start != "" {
		t, err := timefmt.Parse(start)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("start_time 格式应为 YYYY-MM-DD HH:mm:ss")
		}
		db = db.Where(column+" >= ?", t)
	}
	if end != "" {
		t, err := timefmt.Parse(end)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("end_time 格式应为 YYYY-MM-DD HH:mm:ss")
		}
		db = db.Where(column+" <= ?", t)
	}
	return db, nil
}
