package service

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	eqsvc "anxuncloud/internal/module/equipment/service"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

type AppTaskService struct{ db *gorm.DB }

func NewAppTaskService(db *gorm.DB) *AppTaskService { return &AppTaskService{db: db} }

func dateOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func (s *AppTaskService) patrolTypeLabels(values ...string) map[string]string {
	labels := map[string]string{}
	if len(values) == 0 {
		return labels
	}
	var rows []sysmodel.SysDictData
	s.db.Select("value", "label").Where("type_code = ? AND value IN ?", "patrol_type", values).Find(&rows)
	for i := range rows {
		labels[rows[i].Value] = rows[i].Label
	}
	return labels
}

func (s *AppTaskService) TodayTasks(inspectorID string) (gin.H, *errs.Error) {
	return s.tasksByDate(inspectorID, time.Now().Format("2006-01-02"))
}

func (s *AppTaskService) HistoryTasks(inspectorID, date string) (gin.H, *errs.Error) {
	if _, err := time.ParseInLocation("2006-01-02", date, time.Local); err != nil {
		return nil, errs.ErrParam.WithMsg("date 格式应为 YYYY-MM-DD")
	}
	return s.tasksByDate(inspectorID, date)
}

func (s *AppTaskService) tasksByDate(inspectorID, date string) (gin.H, *errs.Error) {
	var tasks []insmodel.InspectionTask
	if err := s.db.Where("inspector_id = ? AND task_date = ?", inspectorID, date).
		Order("CASE WHEN status = 'doing' THEN 0 ELSE 1 END, id ASC").Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if date == time.Now().Format("2006-01-02") {
		var carry []insmodel.InspectionTask
		if err := s.db.Where("inspector_id = ? AND task_date < ? AND due_date >= ? AND status IN ?",
			inspectorID, date, date, []string{insmodel.TaskPending, insmodel.TaskDoing}).
			Order("id ASC").Find(&carry).Error; err != nil {
			return nil, errs.ErrInternal
		}
		tasks = append(tasks, carry...)
	}
	typeSet := map[string]bool{}
	typeValues := make([]string, 0, 4)
	for i := range tasks {
		if v := tasks[i].PatrolType; v != "" && !typeSet[v] {
			typeSet[v] = true
			typeValues = append(typeValues, v)
		}
	}
	typeLabels := s.patrolTypeLabels(typeValues...)
	planIDs, commIDs := []string{}, []string{}
	seenP, seenC := map[string]bool{}, map[string]bool{}
	for i := range tasks {
		t := &tasks[i]
		if !seenP[t.PlanID] {
			seenP[t.PlanID] = true
			planIDs = append(planIDs, t.PlanID)
		}
		if !seenC[t.CommunityID] {
			seenC[t.CommunityID] = true
			commIDs = append(commIDs, t.CommunityID)
		}
	}
	planNames := map[string]string{}
	if len(planIDs) > 0 {
		var plans []insmodel.InspectionPlan
		s.db.Unscoped().Select("id", "name", "deleted_at").Where("id IN ?", planIDs).Find(&plans)
		for i := range plans {
			name := plans[i].Name
			if plans[i].DeletedAt.Valid {
				name += "（已删除）"
			}
			planNames[plans[i].ID] = name
		}
	}
	commNames := map[string]string{}
	if len(commIDs) > 0 {
		var comms []sysmodel.Community
		s.db.Select("id", "name").Where("id IN ?", commIDs).Find(&comms)
		for i := range comms {
			commNames[comms[i].ID] = comms[i].Name
		}
	}
	taskIDs := make([]string, 0, len(tasks))
	for i := range tasks {
		taskIDs = append(taskIDs, tasks[i].ID)
	}
	blockedCounts, err := TaskBlockedPointCounts(s.db, taskIDs)
	if err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(tasks))
	totalPts, donePts := 0, 0
	for i := range tasks {
		t := &tasks[i]
		blocked := blockedCounts[t.ID]
		executableTotal := t.TotalPoints - blocked
		totalPts += executableTotal
		donePts += t.DonePoints
		items = append(items, gin.H{
			"id": t.ID, "plan_name": planNames[t.PlanID], "community_name": commNames[t.CommunityID],
			"patrol_type":       t.PatrolType,
			"patrol_type_label": typeLabels[t.PatrolType],
			"task_date":         t.TaskDate.Format("2006-01-02"), "time_window": t.TimeWindow, "round_name": t.RoundName, "status": t.Status,
			"due_date":     dateOrEmpty(t.DueDate),
			"total_points": executableTotal, "done_points": t.DonePoints, "blocked_points": blocked,
			"progress":   progressOf(t.DonePoints, executableTotal),
			"started_at": timefmt.TP(t.StartedAt),
		})
	}
	return gin.H{
		"date": date, "total_points": totalPts, "done_points": donePts,
		"progress": progressOf(donePts, totalPts), "tasks": items,
	}, nil
}

func (s *AppTaskService) TaskDetail(inspectorID, taskID string) (gin.H, *errs.Error) {
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", taskID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if task.InspectorID != inspectorID {
		return nil, errs.ErrTaskNotOwned
	}
	var plan insmodel.InspectionPlan
	if err := s.db.Unscoped().First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var checkins []insmodel.CheckinRecord
	s.db.Where("task_id = ? AND superseded_by IS NULL", task.ID).Find(&checkins)
	byPoint := map[string]*insmodel.CheckinRecord{}
	for i := range checkins {
		byPoint[checkins[i].PointID] = &checkins[i]
	}
	pointIDs := insmodel.TaskPointIDs(&task)
	blocks, err := ActivePointConfigBlocks(s.db, []string(pointIDs))
	if err != nil {
		return nil, errs.ErrInternal
	}
	blockedPoints := 0
	for _, pointID := range pointIDs {
		if _, blocked := blocks[pointID]; blocked {
			if _, done := byPoint[pointID]; !done {
				blockedPoints++
			}
		}
	}
	executableTotal := task.TotalPoints - blockedPoints
	var ptRows []insmodel.InspectionPoint
	if len(pointIDs) > 0 {
		s.db.Where("id IN ?", []string(pointIDs)).Find(&ptRows)
	}
	ptByID := make(map[string]*insmodel.InspectionPoint, len(ptRows))
	buildingIDSet := map[string]struct{}{}
	for i := range ptRows {
		ptByID[ptRows[i].ID] = &ptRows[i]
		if ptRows[i].BuildingID != nil && *ptRows[i].BuildingID != "" {
			buildingIDSet[*ptRows[i].BuildingID] = struct{}{}
		}
	}
	buildingNames := map[string]string{}
	if len(buildingIDSet) > 0 {
		bIDs := make([]string, 0, len(buildingIDSet))
		for id := range buildingIDSet {
			bIDs = append(bIDs, id)
		}
		var bs []insmodel.Building
		s.db.Select("id", "name").Where("id IN ?", bIDs).Find(&bs)
		for _, b := range bs {
			buildingNames[b.ID] = b.Name
		}
	}
	points := make([]gin.H, 0, len(pointIDs))
	for i, pid := range pointIDs {
		pt, ok := ptByID[pid]
		if !ok {
			continue
		}
		buildingName := ""
		if pt.BuildingID != nil {
			buildingName = buildingNames[*pt.BuildingID]
		}
		var myCheckin any
		if ck, ok := byPoint[pid]; ok {
			dist := any(nil)
			if ck.DistanceToPoint != nil {
				dist = int(*ck.DistanceToPoint)
			}
			alt := any(nil)
			if ck.Altitude != nil {
				alt = *ck.Altitude
			}
			acc := any(nil)
			if ck.Accuracy != nil {
				acc = *ck.Accuracy
			}
			myCheckin = gin.H{
				"id": ck.ID, "checkin_time": timefmt.T(ck.CheckinTime),
				"checkin_type": ck.CheckinType, "distance_to_point": dist,
				"altitude": alt, "accuracy": acc,
				"result": ck.Result, "is_suspect": ck.IsSuspect,
				"audit_status": ck.AuditStatus, "audit_remark": ck.AuditRemark,
				"locked": ck.LockedAt != nil,
			}
		}
		pointStatus, blockReason := "", ""
		if myCheckin == nil {
			if reason, blocked := blocks[pid]; blocked {
				pointStatus, blockReason = "blocked_by_config", reason
			}
		}
		points = append(points, gin.H{
			"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
			"sort": i + 1, "credential": pt.Credential, "require_fence": pt.RequireFence, "qrcode_no": pt.QRCodeNo,
			"nfc_id":    pt.NfcID,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
			"my_checkin": myCheckin, "status": pointStatus, "block_reason": blockReason,
		})
	}
	tplSets := insmodel.LoadTaskTemplateSets(s.db, &task, []string(pointIDs))
	for idx := range points {
		pid, _ := points[idx]["point_id"].(string)
		set := tplSets[pid]
		ci := []gin.H{}
		if set != nil {
			for _, it := range set.Items {
				guide := ""
				if it.Guide != nil {
					guide = *it.Guide
				}
				ci = append(ci, gin.H{
					"name": it.Name, "standard": it.Standard, "standard_no": it.StandardNo,
					"photo_required": it.PhotoRequired, "note_required": it.NoteRequired, "guide": guide,
					"value_type": it.ValueType, "unit": it.Unit, "value_min": it.ValueMin, "value_max": it.ValueMax,
					"enum_options": it.EnumOptions, "ai_enabled": it.AIEnabled, "ai_prompt_kind": it.AIPromptKind,
					"tags": it.Tags, "template_id": it.TemplateID, "item_code": it.Code, "template_name": it.TemplateName,
				})
			}
		}
		points[idx]["check_items"] = ci
	}
	s.injectEquipmentItems(task.ID, task.TaskDate.Format("2006-01-02"), points)
	return gin.H{
		"id": task.ID, "plan_name": plan.Name, "community_name": s.commName(task.CommunityID),
		"patrol_type":       task.PatrolType,
		"patrol_type_label": s.patrolTypeLabels(task.PatrolType)[task.PatrolType],
		"task_date":         task.TaskDate.Format("2006-01-02"), "time_window": insmodel.TaskTimeWindow(&task),
		"due_date":   dateOrEmpty(task.DueDate),
		"round_name": task.RoundName,
		"status":     task.Status, "total_points": executableTotal, "done_points": task.DonePoints, "blocked_points": blockedPoints,
		"progress": progressOf(task.DonePoints, executableTotal), "points": points,
	}, nil
}

func (s *AppTaskService) injectEquipmentItems(taskID, taskDate string, points []gin.H) {
	pointIDs := make([]string, 0, len(points))
	for idx := range points {
		if pid, _ := points[idx]["point_id"].(string); pid != "" {
			pointIDs = append(pointIDs, pid)
		}
	}
	if len(pointIDs) == 0 {
		return
	}
	byPoint, err := eqsvc.LoadPointEquipment(s.db, pointIDs)
	if err != nil {
		logger.L.Warn("台账合成检查项注入：查询关联设备失败，跳过注入", zap.Error(err))
		return
	}
	if len(byPoint) == 0 {
		return
	}
	eqIDs := make([]string, 0, 8)
	for _, eqs := range byPoint {
		for i := range eqs {
			eqIDs = append(eqIDs, eqs[i].ID)
		}
	}
	pendingSet := eqsvc.PendingMaintenanceSet(s.db, eqIDs)
	globalWarn := eqsvc.GlobalWarnDays(s.db)
	now := time.Now()
	spotEnabled := eqsvc.CfgBool(s.db, "equipment.spotcheck_enabled", true)
	globalRatio := eqsvc.CfgInt(s.db, "equipment.spotcheck_ratio", 2)
	spotSalt := eqsvc.CfgString(s.db, "equipment.spotcheck_salt", "")
	var rules map[string]eqsvc.TypeRule
	var verified map[string]time.Time
	if spotEnabled {
		rules = eqsvc.NewEquipmentService(s.db).TypeRules()
		verified = eqsvc.LastVerifiedMap(s.db, eqIDs)
	}
	for idx := range points {
		pid, _ := points[idx]["point_id"].(string)
		eqs := byPoint[pid]
		if len(eqs) == 0 {
			continue
		}
		judges := eqsvc.JudgeDevices(eqs, pendingSet, globalWarn, now)
		items, _ := points[idx]["check_items"].([]gin.H)
		copied := make([]gin.H, len(items), len(items)+len(judges)*2)
		for i, it := range items {
			m := gin.H{}
			for k, v := range it {
				m[k] = v
			}
			copied[i] = m
		}
		for _, j := range judges {
			copied = append(copied, gin.H{
				"name": eqsvc.SyntheticItemName(j), "standard": "", "value_type": insmodel.ValueOK,
				"photo_required": types.PhotoReqNone, "item_code": "builtin_" + insmodel.BuiltinEquipmentValidity,
				"builtin_kind": insmodel.BuiltinEquipmentValidity, "equipment_id": j.EquipmentID,
				"equipment_no": j.Code, "auto_judge": j.View(), "ai_enabled": false,
			})
			if spotEnabled && j.State == eqsvc.DueNormal {
				lv, ok := verified[j.EquipmentID]
				var lvPtr *time.Time
				if ok {
					lvPtr = &lv
				}
				ratio := eqsvc.SpotRatioFor(rules[j.Type], globalRatio, lvPtr, now)
				triggered := eqsvc.SpotTriggered(taskID, pid, j.EquipmentID, taskDate, spotSalt, ratio)
				if triggered {
					copied = append(copied, gin.H{
						"name": eqsvc.SpotItemPrefix + j.Name + "(" + j.Code + ")", "standard": "拍 1 张瓶体标签/钢印照片",
						"value_type": insmodel.ValueOK, "photo_required": types.PhotoReqRequired,
						"item_code": "builtin_" + insmodel.BuiltinEquipmentDateSpot, "builtin_kind": insmodel.BuiltinEquipmentDateSpot,
						"equipment_id": j.EquipmentID, "equipment_no": j.Code, "auto_judge": j.View(), "ai_enabled": true,
					})
				}
			}
		}
		points[idx]["check_items"] = copied
	}
}

func (s *AppTaskService) commName(id string) string {
	var c sysmodel.Community
	if s.db.Select("name").First(&c, "id = ?", id).Error == nil {
		return c.Name
	}
	return ""
}

func progressOf(done, total int) int {
	if total == 0 {
		return 0
	}
	return done * 100 / total
}
