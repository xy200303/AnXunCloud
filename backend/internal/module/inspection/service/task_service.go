package service

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	womodel "anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// TaskService 任务监控与打卡记录检索服务（管理后台）。
type TaskService struct {
	db *gorm.DB
}

func NewTaskService(db *gorm.DB) *TaskService { return &TaskService{db: db} }

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
			Select("task_id").Where("result = ?", model.ResultAbnormal))
	case "suspect":
		db = db.Where("id IN (?)", s.db.Model(&model.CheckinRecord{}).
			Select("task_id").Where("is_suspect"))
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
		Where("task_id IN ?", ids).Group("task_id").Scan(&rows)
	for _, r := range rows {
		out[r.TaskID] = taskCounters{abnormal: r.Abnormal, suspect: r.Suspect}
	}
	return out
}

func (s *TaskService) toItem(t *model.InspectionTask, cnt taskCounters) gin.H {
	var plan model.InspectionPlan
	planName, timeWindow := "", ""
	if s.db.Select("name", "time_window").First(&plan, "id = ?", t.PlanID).Error == nil {
		planName, timeWindow = plan.Name, plan.TimeWindow
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
		"task_date": t.TaskDate.Format("2006-01-02"), "time_window": timeWindow,
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
	if be := middleware.CheckCommunity(c, t.CommunityID); be != nil {
		return nil, be
	}
	var plan model.InspectionPlan
	if err := s.db.First(&plan, "id = ?", t.PlanID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	// 任务下全部打卡记录，按点位归集
	var checkins []model.CheckinRecord
	s.db.Where("task_id = ?", t.ID).Find(&checkins)
	byPoint := map[string]*model.CheckinRecord{}
	for i := range checkins {
		byPoint[checkins[i].PointID] = &checkins[i]
	}
	points := make([]gin.H, 0, len(plan.PointIDs))
	for i, pid := range plan.PointIDs {
		var pt model.InspectionPoint
		if s.db.First(&pt, "id = ?", pid).Error != nil {
			continue
		}
		buildingName := ""
		if pt.BuildingID != nil {
			var b model.Building
			if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
				buildingName = b.Name
			}
		}
		entry := gin.H{
			"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
			"sort": i + 1, "checkin_mode": pt.CheckinMode,
			"status": "pending", "checkin": nil,
		}
		if ck, ok := byPoint[pid]; ok {
			entry["status"] = "done"
			entry["checkin"] = checkinBrief(s.db, ck)
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
			"id": t.ID, "plan_name": plan.Name, "community_name": commName,
			"inspector_id": t.InspectorID, "inspector_name": inspectorName,
			"task_date": t.TaskDate.Format("2006-01-02"), "time_window": plan.TimeWindow,
			"status": t.Status, "total_points": t.TotalPoints, "done_points": t.DonePoints,
			"progress": progress, "started_at": timefmt.TP(t.StartedAt), "finished_at": timefmt.TP(t.FinishedAt),
		},
		"points": points,
	}, nil
}

// checkinBrief 打卡摘要（任务明细内嵌），附带关联工单号。
func checkinBrief(db *gorm.DB, ck *model.CheckinRecord) gin.H {
	var orderNo *string
	var wo womodel.WorkOrder
	if db.Select("order_no").Where("checkin_id = ?", ck.ID).First(&wo).Error == nil {
		orderNo = &wo.OrderNo
	}
	distance := any(nil)
	if ck.DistanceToPoint != nil {
		distance = int(*ck.DistanceToPoint)
	}
	return gin.H{
		"id": ck.ID, "checkin_time": timefmt.T(ck.CheckinTime), "client_time": timefmt.TP(ck.ClientTime),
		"checkin_type": ck.CheckinType, "distance_to_point": distance,
		"longitude": ck.Longitude, "latitude": ck.Latitude,
		"result": ck.Result, "is_suspect": ck.IsSuspect, "suspect_reason": ck.SuspectReason,
		"remark": ck.Remark, "photos": ck.Photos, "work_order_no": orderNo,
		"check_items": ck.CheckItems,
		"audit_status": ck.AuditStatus, "audit_by": ck.AuditBy,
		"audit_at": timefmt.TP(ck.AuditAt), "audit_remark": ck.AuditRemark,
		"ai_verdict": ck.AIVerdict, "ai_reason": ck.AIReason,
	}
}

// ========== 打卡记录检索（管理后台） ==========

// CheckinList 打卡记录分页检索。
func (s *TaskService) CheckinList(c *gin.Context, q *dto.CheckinListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.CheckinRecord{})
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
	if q.AuditStatus != "" {
		db = db.Where("audit_status = ?", q.AuditStatus)
	}
	var be *errs.Error
	if db, be = timeRangeOn(db, "checkin_time", q.StartTime, q.EndTime); be != nil {
		return nil, be
	}
	db = middleware.ApplyCommunityFilter(db, c, "checkin_record.community_id")
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
		r := &rows[i]
		list = append(list, gin.H{
			"id": r.ID, "task_id": r.TaskID, "point_id": r.PointID,
			"point_name":     pointName(s.db, r.PointID),
			"community_name": commName(s.db, r.CommunityID),
			"inspector_id": r.InspectorID, "inspector_name": userName(s.db, r.InspectorID),
			"checkin_time": timefmt.T(r.CheckinTime), "checkin_type": r.CheckinType,
			"distance_to_point": distanceOrNil(r), "result": r.Result,
			"is_suspect": r.IsSuspect, "photo_count": len(r.Photos),
			"audit_status": r.AuditStatus,
		})
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// CheckinDetail 打卡记录详情。
func (s *TaskService) CheckinDetail(c *gin.Context, id string) (gin.H, *errs.Error) {
	// 分区表按 id 查询带宽时间范围即可（此处全分区扫描由主键序列保证唯一）
	var r model.CheckinRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, r.CommunityID); be != nil {
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
	var orderNo *string
	var wo womodel.WorkOrder
	if s.db.Select("order_no").Where("checkin_id = ?", r.ID).First(&wo).Error == nil {
		orderNo = &wo.OrderNo
	}
	// 照片带 EXIF 校验结论
	photos := make([]gin.H, 0, len(r.Photos))
	for _, p := range r.Photos {
		photos = append(photos, gin.H{
			"item": p.Item, "url": p.URL, "watermarked_url": p.WatermarkedURL,
			"exif_check": exifCheck(&r, p),
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
		"is_offline_sync": r.IsOfflineSync,
		"is_suspect": r.IsSuspect, "suspect_reason": r.SuspectReason,
		"photos": photos, "check_items": r.CheckItems, "work_order_no": orderNo,
		"audit_status": r.AuditStatus, "audit_by": r.AuditBy,
		"audit_at": timefmt.TP(r.AuditAt), "audit_remark": r.AuditRemark,
		"ai_verdict": r.AIVerdict, "ai_reason": r.AIReason,
		"created_at": timefmt.T(r.CreatedAt),
	}, nil
}

// exifCheck 构造照片 EXIF 校验结论（无 EXIF 时间时 passed 为 nil）。
func exifCheck(r *model.CheckinRecord, p types.PhotoItem) gin.H {
	if p.ExifTime == "" {
		return gin.H{"shot_at": nil, "deviation_seconds": nil, "passed": nil}
	}
	shot, err := timefmt.Parse(p.ExifTime)
	if err != nil {
		return gin.H{"shot_at": p.ExifTime, "deviation_seconds": nil, "passed": nil}
	}
	dev := int(shot.Sub(r.CheckinTime).Seconds())
	if dev < 0 {
		dev = -dev
	}
	return gin.H{"shot_at": p.ExifTime, "deviation_seconds": dev, "passed": dev <= 300}
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

func distanceOrNil(r *model.CheckinRecord) any {
	if r.DistanceToPoint == nil {
		return nil
	}
	return int(*r.DistanceToPoint)
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
