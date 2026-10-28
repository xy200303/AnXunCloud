package service

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

func (s *AppTaskService) PointByCode(inspectorID, code string) (gin.H, *errs.Error) {
	var pt insmodel.InspectionPoint
	code = strings.TrimSpace(code)
	if err := s.db.Where("qrcode_no = ? OR nfc_id = ?", code, code).First(&pt).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	var user sysmodel.SysUser
	if err := s.db.Select("id", "role_id", "tenant_id").First(&user, "id = ?", inspectorID).Error; err != nil {
		return nil, errs.ErrUnauthorized
	}
	var role sysmodel.SysRole
	if user.RoleID != nil {
		s.db.First(&role, "id = ?", *user.RoleID)
	}
	scopeAll := role.ScopeLevel == sysmodel.ScopeTenant
	isSuper := role.Code == sysmodel.SuperAdminCode
	if !isSuper && pt.TenantID != nil && user.TenantID != "" && *pt.TenantID != user.TenantID {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	if !scopeAll {
		projectIDs, err := middleware.StaffProjectIDs(s.db, inspectorID)
		if err != nil {
			return nil, errs.ErrInternal
		}
		if !types.IDArray(projectIDs).Contains(pt.CommunityID) {
			return nil, errs.ErrDataScope
		}
	}
	today := time.Now().Format("2006-01-02")
	var tasks []insmodel.InspectionTask
	s.db.Where("inspector_id = ? AND (task_date = ? OR (task_date < ? AND due_date >= ?))", inspectorID, today, today, today).Find(&tasks)
	matched := make([]gin.H, 0, 1)
	for i := range tasks {
		t := &tasks[i]
		var plan insmodel.InspectionPlan
		if s.db.Unscoped().Select("id", "name", "point_ids").First(&plan, "id = ?", t.PlanID).Error != nil {
			continue
		}
		if !insmodel.TaskPointIDs(t).Contains(pt.ID) {
			continue
		}
		checked := s.db.Where("task_id = ? AND point_id = ? AND superseded_by IS NULL", t.ID, pt.ID).
			First(&insmodel.CheckinRecord{}).Error == nil
		matched = append(matched, gin.H{"task_id": t.ID, "plan_name": plan.Name, "status": t.Status, "checked": checked})
	}
	buildingName := ""
	if pt.BuildingID != nil {
		var b insmodel.Building
		if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
			buildingName = b.Name
		}
	}
	return gin.H{
		"point": gin.H{
			"id": pt.ID, "name": pt.Name, "qrcode_no": pt.QRCodeNo, "nfc_id": pt.NfcID,
			"community_id": pt.CommunityID, "community_name": s.commName(pt.CommunityID),
			"building_name": buildingName,
			"credential":    pt.Credential, "require_fence": pt.RequireFence,
			"longitude": pt.Longitude, "latitude": pt.Latitude, "fence_radius": pt.FenceRadius,
		},
		"tasks": matched,
	}, nil
}

func (s *AppTaskService) PublicPoint(code string) (gin.H, *errs.Error) {
	var pt insmodel.InspectionPoint
	if err := s.db.Where("qrcode_no = ?", strings.TrimSpace(code)).First(&pt).Error; err != nil {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	if pt.Status != "enabled" {
		return nil, errs.ErrNotFound.WithMsg("未找到相关点位信息")
	}
	buildingName := ""
	if pt.BuildingID != nil {
		var b insmodel.Building
		if s.db.Select("name").First(&b, "id = ?", *pt.BuildingID).Error == nil {
			buildingName = b.Name
		}
	}
	since := time.Now().AddDate(0, 0, -30)
	var total30, abnormal30 int64
	s.db.Model(&insmodel.CheckinRecord{}).Where("point_id = ? AND checkin_time >= ? AND superseded_by IS NULL", pt.ID, since).Count(&total30)
	s.db.Model(&insmodel.CheckinRecord{}).
		Where("point_id = ? AND checkin_time >= ? AND result = ? AND superseded_by IS NULL", pt.ID, since, insmodel.ResultAbnormal).Count(&abnormal30)
	var recs []insmodel.CheckinRecord
	s.db.Where("point_id = ? AND superseded_by IS NULL", pt.ID).Order("checkin_time DESC").Limit(5).Find(&recs)
	recent := make([]gin.H, 0, len(recs))
	for i := range recs {
		r := &recs[i]
		name := ""
		var u sysmodel.SysUser
		if s.db.Select("name").First(&u, "id = ?", r.InspectorID).Error == nil {
			name = maskName(u.Name)
		}
		recent = append(recent, gin.H{
			"checkin_time": r.CheckinTime.Format("2006-01-02 15:04"),
			"result":       r.Result,
			"checkin_type": r.CheckinType,
			"inspector":    name,
		})
	}
	return gin.H{
		"point": gin.H{
			"name": pt.Name, "qrcode_no": pt.QRCodeNo, "type": pt.Type,
			"community_name": s.commName(pt.CommunityID), "building_name": buildingName,
		},
		"stats":  gin.H{"total_30d": total30, "abnormal_30d": abnormal30},
		"recent": recent,
	}, nil
}

func maskName(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) <= 1 {
		return name
	}
	return string(r[0]) + "*"
}

const nearbyLimit = 20

func (s *AppTaskService) NearbyPoints(inspectorID string, lng, lat float64) (gin.H, *errs.Error) {
	if lng < -180 || lng > 180 || lat < -90 || lat > 90 || (lng == 0 && lat == 0) {
		return nil, errs.ErrParam.WithMsg("定位坐标非法")
	}
	today := time.Now().Format("2006-01-02")
	var tasks []insmodel.InspectionTask
	if err := s.db.Where("inspector_id = ? AND status IN ? AND (task_date = ? OR (task_date < ? AND due_date >= ?))",
		inspectorID, []string{insmodel.TaskPending, insmodel.TaskDoing, insmodel.TaskOverdue}, today, today, today).Find(&tasks).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(tasks) == 0 {
		return gin.H{"list": []gin.H{}}, nil
	}
	planIDs := make([]string, 0, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		planIDs = append(planIDs, t.PlanID)
		taskIDs = append(taskIDs, t.ID)
	}
	planNames := map[string]string{}
	{
		var plans []insmodel.InspectionPlan
		s.db.Unscoped().Select("id", "name").Where("id IN ?", planIDs).Find(&plans)
		for _, p := range plans {
			planNames[p.ID] = p.Name
		}
	}
	pointIDSet := map[string]struct{}{}
	taskPointIDs := make(map[string][]string, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		if _, ok := planNames[t.PlanID]; !ok {
			continue
		}
		ids := insmodel.TaskPointIDs(t)
		taskPointIDs[t.ID] = []string(ids)
		for _, pid := range ids {
			pointIDSet[pid] = struct{}{}
		}
	}
	ptIDs := make([]string, 0, len(pointIDSet))
	for pid := range pointIDSet {
		ptIDs = append(ptIDs, pid)
	}
	ptByID := map[string]*insmodel.InspectionPoint{}
	buildingIDSet := map[string]struct{}{}
	if len(ptIDs) > 0 {
		var pts []insmodel.InspectionPoint
		s.db.Where("id IN ?", ptIDs).Find(&pts)
		for i := range pts {
			ptByID[pts[i].ID] = &pts[i]
			if pts[i].BuildingID != nil && *pts[i].BuildingID != "" {
				buildingIDSet[*pts[i].BuildingID] = struct{}{}
			}
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
	checkedSet := map[string]bool{}
	{
		var recs []insmodel.CheckinRecord
		s.db.Select("task_id", "point_id").Where("task_id IN ? AND superseded_by IS NULL", taskIDs).Find(&recs)
		for _, r := range recs {
			checkedSet[r.TaskID+"|"+r.PointID] = true
		}
	}
	type entry struct {
		row  gin.H
		dist float64
	}
	entries := make([]entry, 0, len(pointIDSet))
	for i := range tasks {
		t := &tasks[i]
		for _, pid := range taskPointIDs[t.ID] {
			pt, ok := ptByID[pid]
			if !ok || (pt.Longitude == 0 && pt.Latitude == 0) {
				continue
			}
			d := haversineMeters(lng, lat, pt.Longitude, pt.Latitude)
			buildingName := ""
			if pt.BuildingID != nil {
				buildingName = buildingNames[*pt.BuildingID]
			}
			entries = append(entries, entry{row: gin.H{
				"task_id": t.ID, "plan_name": planNames[t.PlanID], "patrol_type": t.PatrolType,
				"point_id": pt.ID, "point_name": pt.Name, "building_name": buildingName,
				"distance":   int(d + 0.5),
				"checked":    checkedSet[t.ID+"|"+pid],
				"credential": pt.Credential, "require_fence": pt.RequireFence,
				"task_status": t.Status,
			}, dist: d})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		ci, cj := entries[i].row["checked"].(bool), entries[j].row["checked"].(bool)
		if ci != cj {
			return !ci
		}
		return entries[i].dist < entries[j].dist
	})
	out := make([]gin.H, 0, nearbyLimit)
	for i := 0; i < len(entries) && i < nearbyLimit; i++ {
		out = append(out, entries[i].row)
	}
	return gin.H{"list": out}, nil
}

func haversineMeters(lng1, lat1, lng2, lat2 float64) float64 {
	const earthR = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthR * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
