// Package service 统计报表业务逻辑：覆盖率/及时率/绩效/导出 + 工作台总览。
package service

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"property-inspection/internal/middleware"
	insmodel "property-inspection/internal/module/inspection/model"
	"property-inspection/internal/module/stats/dto"
	sysmodel "property-inspection/internal/module/system/model"
	womodel "property-inspection/internal/module/workorder/model"
	"property-inspection/internal/pkg/errs"
	"property-inspection/internal/pkg/response"
	"property-inspection/internal/pkg/storage"
	"property-inspection/internal/pkg/timefmt"
)

// StatsService 统计报表服务。
type StatsService struct {
	db    *gorm.DB
	store *storage.Storage
}

func NewStatsService(db *gorm.DB, store *storage.Storage) *StatsService {
	return &StatsService{db: db, store: store}
}

// parseRange 解析并校验日期范围（跨度 ≤366 天）。
func parseRange(start, end string) (time.Time, time.Time, *errs.Error) {
	s, err := time.ParseInLocation("2006-01-02", start, time.Local)
	if err != nil {
		return s, s, errs.ErrParam.WithMsg("start_date 格式应为 YYYY-MM-DD")
	}
	e, err := time.ParseInLocation("2006-01-02", end, time.Local)
	if err != nil {
		return s, s, errs.ErrParam.WithMsg("end_date 格式应为 YYYY-MM-DD")
	}
	if e.Before(s) {
		return s, s, errs.ErrParam.WithMsg("end_date 不能早于 start_date")
	}
	if e.Sub(s) > 366*24*time.Hour {
		return s, s, errs.ErrParam.WithMsg("日期跨度不能超过 366 天")
	}
	return s, e, nil
}

// taskScope 带数据权限与日期范围的任务查询。
func (s *StatsService) taskScope(c *gin.Context, q *dto.ReportQuery) *gorm.DB {
	db := s.db.Model(&insmodel.InspectionTask{}).
		Where("task_date >= ? AND task_date <= ?", q.StartDate, q.EndDate)
	if q.CommunityID > 0 {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	return middleware.ApplyCommunityFilter(db, c, "inspection_task.community_id")
}

// pct 百分比（保留 1 位小数）。
func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	v := float64(part) * 100 / float64(total)
	return float64(int(v*10+0.5)) / 10
}

// Coverage 巡检覆盖率报表。
func (s *StatsService) Coverage(c *gin.Context, q *dto.ReportQuery) (gin.H, *errs.Error) {
	if _, _, be := parseRange(q.StartDate, q.EndDate); be != nil {
		return nil, be
	}
	scope := s.taskScope(c, q)

	var sum struct {
		Should int64
		Done   int64
	}
	scope.Session(&gorm.Session{}).Select("COALESCE(SUM(total_points),0) AS should, COALESCE(SUM(done_points),0) AS done").Scan(&sum)

	// 异常/疑似打卡数
	ck := s.db.Model(&insmodel.CheckinRecord{}).
		Where("checkin_time >= ? AND checkin_time < ?", q.StartDate, q.EndDate+" 23:59:59")
	if q.CommunityID > 0 {
		ck = ck.Where("community_id = ?", q.CommunityID)
	}
	ck = middleware.ApplyCommunityFilter(ck, c, "checkin_record.community_id")
	var ab struct {
		Abnormal int64
		Suspect  int64
	}
	ck.Select("COUNT(*) FILTER (WHERE result = 'abnormal') AS abnormal, COUNT(*) FILTER (WHERE is_suspect) AS suspect").Scan(&ab)

	// 按日
	type dayRow struct {
		Date   string
		Should int64
		Done   int64
	}
	var days []dayRow
	scope.Session(&gorm.Session{}).
		Select("to_char(task_date, 'YYYY-MM-DD') AS date, SUM(total_points) AS should, SUM(done_points) AS done").
		Group("date").Order("date").Scan(&days)
	daily := make([]gin.H, 0, len(days))
	for _, d := range days {
		daily = append(daily, gin.H{"date": d.Date, "should_points": d.Should, "done_points": d.Done, "coverage_rate": pct(d.Done, d.Should)})
	}
	// 按小区
	type commRow struct {
		CommunityID string
		Should      int64
		Done        int64
	}
	var commRows []commRow
	scope.Session(&gorm.Session{}).
		Select("community_id, SUM(total_points) AS should, SUM(done_points) AS done").
		Group("community_id").Scan(&commRows)
	byComm := make([]gin.H, 0, len(commRows))
	for _, r := range commRows {
		byComm = append(byComm, gin.H{
			"community_id": r.CommunityID, "community_name": s.commName(r.CommunityID),
			"should_points": r.Should, "done_points": r.Done, "coverage_rate": pct(r.Done, r.Should),
		})
	}
	return gin.H{
		"summary": gin.H{
			"should_points": sum.Should, "done_points": sum.Done,
			"coverage_rate": pct(sum.Done, sum.Should),
			"abnormal_count": ab.Abnormal, "suspect_count": ab.Suspect,
		},
		"daily": daily, "by_community": byComm,
	}, nil
}

// Timeliness 巡检及时率报表（及时 = 任务在 time_window 截止前完成）。
func (s *StatsService) Timeliness(c *gin.Context, q *dto.ReportQuery) (gin.H, *errs.Error) {
	if _, _, be := parseRange(q.StartDate, q.EndDate); be != nil {
		return nil, be
	}
	scope := s.taskScope(c, q)
	// 及时判定：finished_at <= task_date + time_window 结束时刻；SQL 用 split_part 取时段结束
	onTimeExpr := `CASE WHEN status = 'done' AND finished_at IS NOT NULL AND
		finished_at <= (task_date + (split_part((SELECT time_window FROM inspection_plan WHERE id = inspection_task.plan_id), '-', 2))::interval) THEN 1 ELSE 0 END`
	var sum struct {
		Total   int64
		OnTime  int64
		Overdue int64
	}
	scope.Session(&gorm.Session{}).
		Select("COUNT(*) AS total, COALESCE(SUM("+onTimeExpr+"),0) AS on_time, COUNT(*) FILTER (WHERE status = 'overdue') AS overdue").
		Scan(&sum)

	type dayRow struct {
		Date   string
		Total  int64
		OnTime int64
	}
	var days []dayRow
	scope.Session(&gorm.Session{}).
		Select("to_char(task_date, 'YYYY-MM-DD') AS date, COUNT(*) AS total, COALESCE(SUM("+onTimeExpr+"),0) AS on_time").
		Group("date").Order("date").Scan(&days)
	daily := make([]gin.H, 0, len(days))
	for _, d := range days {
		daily = append(daily, gin.H{"date": d.Date, "total_tasks": d.Total, "on_time_tasks": d.OnTime, "timeliness_rate": pct(d.OnTime, d.Total)})
	}
	type commRow struct {
		CommunityID string
		Total       int64
		OnTime      int64
	}
	var commRows []commRow
	scope.Session(&gorm.Session{}).
		Select("community_id, COUNT(*) AS total, COALESCE(SUM("+onTimeExpr+"),0) AS on_time").
		Group("community_id").Scan(&commRows)
	byComm := make([]gin.H, 0, len(commRows))
	for _, r := range commRows {
		byComm = append(byComm, gin.H{
			"community_id": r.CommunityID, "community_name": s.commName(r.CommunityID),
			"total_tasks": r.Total, "on_time_tasks": r.OnTime, "timeliness_rate": pct(r.OnTime, r.Total),
		})
	}
	return gin.H{
		"summary": gin.H{
			"total_tasks": sum.Total, "on_time_tasks": sum.OnTime,
			"timeliness_rate": pct(sum.OnTime, sum.Total), "overdue_tasks": sum.Overdue,
		},
		"daily": daily, "by_community": byComm,
	}, nil
}

// Performance 巡检员绩效报表。
func (s *StatsService) Performance(c *gin.Context, q *dto.PerformanceQuery) (*response.Page, *errs.Error) {
	if _, _, be := parseRange(q.StartDate, q.EndDate); be != nil {
		return nil, be
	}
	scope := s.taskScope(c, &q.ReportQuery)
	type row struct {
		InspectorID   string
		TotalTasks    int64
		DoneTasks     int64
		ShouldPoints  int64
		DonePoints    int64
		AvgDurationMs float64
	}
	query := scope.Session(&gorm.Session{}).
		Select(`inspector_id,
			COUNT(*) AS total_tasks,
			COUNT(*) FILTER (WHERE status = 'done') AS done_tasks,
			COALESCE(SUM(total_points),0) AS should_points,
			COALESCE(SUM(done_points),0) AS done_points,
			COALESCE(AVG(EXTRACT(EPOCH FROM (finished_at - started_at)) * 1000) FILTER (WHERE status = 'done' AND started_at IS NOT NULL AND finished_at IS NOT NULL), 0) AS avg_duration_ms`).
		Group("inspector_id")
	var rows []row
	query.Scan(&rows)

	// 异常发现数与疑似数（按打卡记录统计）
	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		var ab struct {
			Abnormal int64
			Suspect  int64
		}
		ck := s.db.Model(&insmodel.CheckinRecord{}).
			Where("inspector_id = ? AND checkin_time >= ? AND checkin_time < ?", r.InspectorID, q.StartDate, q.EndDate+" 23:59:59")
		ck = middleware.ApplyCommunityFilter(ck, c, "checkin_record.community_id")
		ck.Select("COUNT(*) FILTER (WHERE result = 'abnormal') AS abnormal, COUNT(*) FILTER (WHERE is_suspect) AS suspect").Scan(&ab)
		// 所辖小区名
		var commNames []string
		s.db.Model(&insmodel.InspectionTask{}).Distinct().
			Joins("JOIN community ON community.id = inspection_task.community_id").
			Where("inspector_id = ? AND task_date >= ? AND task_date <= ?", r.InspectorID, q.StartDate, q.EndDate).
			Pluck("community.name", &commNames)
		items = append(items, gin.H{
			"inspector_id": r.InspectorID, "inspector_name": s.userName(r.InspectorID),
			"community_names": commNames,
			"total_tasks": r.TotalTasks, "done_tasks": r.DoneTasks,
			"should_points": r.ShouldPoints, "done_points": r.DonePoints,
			"coverage_rate": pct(r.DonePoints, r.ShouldPoints),
			"avg_duration_min": int(r.AvgDurationMs / 60000),
			"abnormal_found": ab.Abnormal, "suspect_count": ab.Suspect,
		})
	}
	// 内存排序（列排序）
	sortBy := q.SortBy
	if sortBy == "" {
		sortBy = "coverage_rate"
	}
	desc := q.SortOrder != "asc"
	less := func(i, j int) bool {
		vi := sortVal(items[i], sortBy)
		vj := sortVal(items[j], sortBy)
		if desc {
			return vi > vj
		}
		return vi < vj
	}
	sortSlice(items, less)
	offset, limit := q.Normalize()
	total := int64(len(items))
	end := offset + limit
	if offset > len(items) {
		offset = len(items)
	}
	if end > len(items) {
		end = len(items)
	}
	return &response.Page{List: items[offset:end], Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func sortVal(item gin.H, key string) float64 {
	switch v := item[key].(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

func sortSlice(items []gin.H, less func(i, j int) bool) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// Export 同步生成报表 Excel，返回下载地址（小报表场景；异步任务+消息通知为后续扩展）。
func (s *StatsService) Export(c *gin.Context, req *dto.ExportReq) (gin.H, *errs.Error) {
	if req.Format == "pdf" {
		return nil, errs.ErrParam.WithMsg("PDF 月报将在后续版本提供，当前请使用 excel")
	}
	if _, _, be := parseRange(req.StartDate, req.EndDate); be != nil {
		return nil, be
	}
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	q := &dto.ReportQuery{StartDate: req.StartDate, EndDate: req.EndDate, CommunityID: req.CommunityID}

	writeRow := func(row int, vals ...any) {
		for i, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}
	var fileTitle string
	switch req.ReportType {
	case "coverage", "monthly":
		fileTitle = "巡检覆盖率报表"
		data, be := s.Coverage(c, q)
		if be != nil {
			return nil, be
		}
		sum := data["summary"].(gin.H)
		writeRow(1, "巡检覆盖率报表", req.StartDate+" ~ "+req.EndDate)
		writeRow(2, "应检点位", sum["should_points"], "已检点位", sum["done_points"], "覆盖率%", sum["coverage_rate"], "异常数", sum["abnormal_count"], "疑似作弊数", sum["suspect_count"])
		writeRow(3)
		writeRow(4, "日期", "应检点位", "已检点位", "覆盖率%")
		for i, d := range data["daily"].([]gin.H) {
			writeRow(5+i, d["date"], d["should_points"], d["done_points"], d["coverage_rate"])
		}
	case "timeliness":
		fileTitle = "巡检及时率报表"
		data, be := s.Timeliness(c, q)
		if be != nil {
			return nil, be
		}
		sum := data["summary"].(gin.H)
		writeRow(1, "巡检及时率报表", req.StartDate+" ~ "+req.EndDate)
		writeRow(2, "任务总数", sum["total_tasks"], "及时完成", sum["on_time_tasks"], "及时率%", sum["timeliness_rate"], "逾期任务", sum["overdue_tasks"])
		writeRow(3)
		writeRow(4, "日期", "任务总数", "及时完成", "及时率%")
		for i, d := range data["daily"].([]gin.H) {
			writeRow(5+i, d["date"], d["total_tasks"], d["on_time_tasks"], d["timeliness_rate"])
		}
	case "operation_log", "login_log":
		// 日志导出（前端日志管理页导出按钮）
		title, lbe := s.exportLogs(f, sheet, writeRow, req)
		if lbe != nil {
			return nil, lbe
		}
		fileTitle = title
	case "performance":
		fileTitle = "巡检员绩效报表"
		pq := &dto.PerformanceQuery{ReportQuery: *q}
		pq.Page, pq.PageSize = 1, 10000
		page, be := s.Performance(c, pq)
		if be != nil {
			return nil, be
		}
		writeRow(1, "巡检员绩效报表", req.StartDate+" ~ "+req.EndDate)
		writeRow(2, "巡检员", "任务数", "完成数", "覆盖率%", "平均耗时(分)", "异常发现", "疑似作弊")
		for i, item := range page.List.([]gin.H) {
			writeRow(3+i, item["inspector_name"], item["total_tasks"], item["done_tasks"], item["coverage_rate"], item["avg_duration_min"], item["abnormal_found"], item["suspect_count"])
		}
	}
	var buf []byte
	{
		b, err := f.WriteToBuffer()
		if err != nil {
			return nil, errs.ErrInternal
		}
		buf = b.Bytes()
	}
	fileName := fmt.Sprintf("%s_%s_%s.xlsx", fileTitle, req.StartDate, req.EndDate)
	_, url, err := s.store.SaveGenerated("stats", fileName, buf)
	if err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{
		"export_id":    fmt.Sprintf("exp%s", time.Now().Format("20060102150405")),
		"status":       "done",
		"download_url": url,
		"expire_at":    time.Now().Add(24 * time.Hour).Format(timefmt.Layout),
	}, nil
}

// Dashboard 工作台总览（数据权限自动按所辖小区过滤）。
func (s *StatsService) Dashboard(c *gin.Context, communityID int64) (gin.H, *errs.Error) {
	today := time.Now().Format("2006-01-02")
	taskQ := s.db.Model(&insmodel.InspectionTask{}).Where("task_date = ?", today)
	if communityID > 0 {
		taskQ = taskQ.Where("community_id = ?", communityID)
	}
	taskQ = middleware.ApplyCommunityFilter(taskQ, c, "inspection_task.community_id")
	var todaySum struct {
		Total   int64
		Done    int64
		Doing   int64
		Overdue int64
	}
	taskQ.Session(&gorm.Session{}).
		Select("COALESCE(SUM(total_points),0) AS total, COALESCE(SUM(done_points),0) AS done, COUNT(*) FILTER (WHERE status = 'doing') AS doing, COUNT(*) FILTER (WHERE status = 'overdue') AS overdue").
		Scan(&todaySum)

	woQ := s.db.Model(&womodel.WorkOrder{}).Where("status IN ?", []string{womodel.OrderPending, womodel.OrderAssigned, womodel.OrderProcessing})
	woQ = middleware.ApplyCommunityFilter(woQ, c, "work_order.community_id")
	var pendingWO int64
	woQ.Session(&gorm.Session{}).Count(&pendingWO)

	// 近 7 天趋势
	trend := make([]gin.H, 0, 7)
	for i := 6; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i)
		dayStr := day.Format("2006-01-02")
		var ds struct {
			Total int64
			Done  int64
		}
		dq := s.db.Model(&insmodel.InspectionTask{}).Where("task_date = ?", dayStr)
		dq = middleware.ApplyCommunityFilter(dq, c, "inspection_task.community_id")
		dq.Session(&gorm.Session{}).Select("COALESCE(SUM(total_points),0) AS total, COALESCE(SUM(done_points),0) AS done").Scan(&ds)
		trend = append(trend, gin.H{"date": dayStr, "total": ds.Total, "done": ds.Done, "rate": pct(ds.Done, ds.Total)})
	}
	// 小区排行（今日）
	type rankRow struct {
		CommunityID string
		Total       int64
		Done        int64
	}
	var ranks []rankRow
	rankQ := s.db.Model(&insmodel.InspectionTask{}).Where("task_date = ?", today)
	rankQ = middleware.ApplyCommunityFilter(rankQ, c, "inspection_task.community_id")
	rankQ.Session(&gorm.Session{}).
		Select("community_id, SUM(total_points) AS total, SUM(done_points) AS done").
		Group("community_id").Scan(&ranks)
	rankList := make([]gin.H, 0, len(ranks))
	for _, r := range ranks {
		rankList = append(rankList, gin.H{
			"community_id": r.CommunityID, "community_name": s.commName(r.CommunityID),
			"total": r.Total, "done": r.Done, "rate": pct(r.Done, r.Total),
		})
	}
	// 最新工单（前 5）
	var orders []womodel.WorkOrder
	latestQ := s.db.Model(&womodel.WorkOrder{})
	latestQ = middleware.ApplyCommunityFilter(latestQ, c, "work_order.community_id")
	latestQ.Session(&gorm.Session{}).Order("id DESC").Limit(5).Find(&orders)
	latest := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		latest = append(latest, gin.H{
			"id": o.ID, "order_no": o.OrderNo, "title": o.Title,
			"community_name": s.commName(o.CommunityID),
			"priority": o.Priority, "status": o.Status, "created_at": timefmt.T(o.CreatedAt),
		})
	}
	// 今日执行动态（最近 10 条打卡）
	var cks []insmodel.CheckinRecord
	ckQ := s.db.Model(&insmodel.CheckinRecord{}).Where("checkin_time >= ?", today)
	ckQ = middleware.ApplyCommunityFilter(ckQ, c, "checkin_record.community_id")
	ckQ.Session(&gorm.Session{}).Order("checkin_time DESC").Limit(10).Find(&cks)
	timeline := make([]gin.H, 0, len(cks))
	for _, ck := range cks {
		action := "完成打卡"
		if ck.Result == insmodel.ResultAbnormal {
			action = "上报异常：" + ck.Remark
		}
		timeline = append(timeline, gin.H{
			"time": timefmt.T(ck.CheckinTime), "inspector_name": s.userName(ck.InspectorID),
			"task_id": ck.TaskID, "task_name": s.pointName(ck.PointID), "action": action,
		})
	}
	return gin.H{
		"today_completion": gin.H{"total": todaySum.Total, "done": todaySum.Done, "rate": pct(todaySum.Done, todaySum.Total)},
		"doing_tasks":          todaySum.Doing,
		"pending_workorders":   pendingWO,
		"overdue_tasks":        todaySum.Overdue,
		"trend_7d":             trend,
		"community_rank":       rankList,
		"latest_workorders":    latest,
		"task_timeline":        timeline,
	}, nil
}

func (s *StatsService) commName(id string) string {
	var c sysmodel.Community
	if s.db.Select("name").First(&c, "id = ?", id).Error == nil {
		return c.Name
	}
	return ""
}

func (s *StatsService) userName(id string) string {
	var u sysmodel.SysUser
	if s.db.Select("name").First(&u, "id = ?", id).Error == nil {
		return u.Name
	}
	return ""
}

func (s *StatsService) pointName(id string) string {
	var name string
	s.db.Table("inspection_point").Select("name").Where("id = ?", id).Scan(&name)
	return name
}

// exportLogs 导出操作/登录日志（时间范围内，上限 5 万行）。
func (s *StatsService) exportLogs(f *excelize.File, sheet string, writeRow func(int, ...any), req *dto.ExportReq) (string, *errs.Error) {
	start := req.StartDate + " 00:00:00"
	end := req.EndDate + " 23:59:59"
	const logExportMax = 50000
	if req.ReportType == "operation_log" {
		var rows []sysmodel.SysOperationLog
		if err := s.db.Where("created_at >= ? AND created_at <= ?", start, end).
			Order("created_at DESC").Limit(logExportMax).Find(&rows).Error; err != nil {
			return "", errs.ErrInternal
		}
		writeRow(1, "操作日志", start+" ~ "+end)
		writeRow(2, "时间", "操作人", "模块", "动作", "方法", "路径", "IP", "状态", "耗时ms")
		for i, r := range rows {
			writeRow(3+i, timefmt.T(r.CreatedAt), r.Username, r.Module, r.Action, r.Method, r.Path, r.IP, r.Status, r.CostMs)
		}
		return "操作日志", nil
	}
	var rows []sysmodel.SysLoginLog
	if err := s.db.Where("created_at >= ? AND created_at <= ?", start, end).
		Order("created_at DESC").Limit(logExportMax).Find(&rows).Error; err != nil {
		return "", errs.ErrInternal
	}
	writeRow(1, "登录日志", start+" ~ "+end)
	writeRow(2, "时间", "账号", "渠道", "IP", "状态", "说明")
	for i, r := range rows {
		writeRow(3+i, timefmt.T(r.CreatedAt), r.Username, r.Channel, r.IP, r.Status, r.Msg)
	}
	return "登录日志", nil
}
