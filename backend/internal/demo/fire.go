// 消防专项 + 两班倒巡更演示数据（《专项巡检与专项检查报告设计方案》§3.2/§3.3、§5 第 7 条）：
// 锦绣华庭消防箱/灭火器点位（绑带 ai_hint 的专项检查模板）、两班倒巡更计划（近 3 天执行 + 今日进行中）、
// 消防设施月度专项计划（当月任务已完成，含 2 处异常转工程条线工单）、当月专项检查报告（三级签字归档）。
// 主种子路径（seedTenantA）与回填命令（seed-demo -fire → SeedFireDemo）共用 seedFireSpecial。
package demo

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	insmodel "anxuncloud/internal/module/inspection/model"
	rptmodel "anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	womodel "anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// 演示数据命名（幂等哨兵：专项计划存在即整体跳过）。
const (
	firePlanPatrolName = "日常保安巡更（两班倒）"
	firePlanMonthName  = "消防设施月度专项检查"
	fireTplCabinetName = "消防箱检查模板"
	fireTplExtName     = "灭火器检查模板"
	fireFloorsPerBld   = 6 // 每栋楼层数（每层楼 1 个消防箱点位）
)

// firePeople 消防演示涉及的账号。
type firePeople struct{ manager, eng, repair, xj01, xj02 string }

// fireTplItem 专项检查模板项（比 demoTemplateItem 多 ai_hint 识别要点）。
type fireTplItem struct{ name, requirement, photoReq, aiHint string }

// fireCabinetItems 消防箱检查模板项（§3.3：必拍项带 ai_hint）。
var fireCabinetItems = []fireTplItem{
	{"消防枪头在位", "打开箱门确认消防枪头在位、无缺失", types.PhotoReqRequired, "画面中应可见消防枪头"},
	{"水带齐全完好", "水带齐全、卷盘完好，无破损霉变", types.PhotoReqRequired, "画面中应可见卷盘/水带"},
	{"手报按钮正常", "手动报警按钮外观完好、无破损", types.PhotoReqRequired, "手动报警按钮外观无破损"},
	{"箱体与通道无遮挡", "箱门可正常开启，箱体前方通道无堆物遮挡", types.PhotoReqOptional, ""},
}

// fireExtItems 灭火器检查模板项。
var fireExtItems = []fireTplItem{
	{"压力表指针在绿区", "压力表指针位于绿色区域，铅封完好", types.PhotoReqRequired, "指针位于绿色区域"},
	{"在有效期内", "铭牌生产日期/有效期清晰，灭火器在有效期内", types.PhotoReqRequired, "读取铭牌生产日期/有效期并判断是否过期"},
	{"瓶体无锈蚀变形摆放无遮挡", "瓶体无锈蚀、无变形，摆放位置醒目无遮挡", types.PhotoReqOptional, ""},
}

// SeedFireDemo 消防专项+两班倒演示数据回填（老库升级用，seed-demo -fire）：
// 为已存在的演示租户（华安/锦绣华庭）补消防点位/模板/两班倒巡更/月度专项/当月专项报告，不动其他数据。
// 幂等：专项计划「消防设施月度专项检查」已存在则整体跳过；演示租户或小区不存在则无事可做。
func SeedFireDemo(db *gorm.DB, store *storage.Storage) error {
	var tenant sysmodel.Tenant
	if err := db.Select("id").Where("code = ?", demoTenantACode).First(&tenant).Error; err != nil {
		return nil
	}
	tid := tenant.ID
	var comm sysmodel.Community
	if err := db.Select("id").Where("tenant_id = ? AND name = ?", tid, "锦绣华庭").First(&comm).Error; err != nil {
		return nil
	}
	cid := comm.ID
	var cnt int64
	if err := db.Model(&insmodel.InspectionPlan{}).Where("community_id = ? AND name = ?", cid, firePlanMonthName).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	// 账号（主种子路径创建的演示账号，用户名即幂等键）
	p := firePeople{}
	for uname, dst := range map[string]*string{
		"ha_manager": &p.manager, "ha_eng": &p.eng, "ha_repair": &p.repair,
		"ha_xj01": &p.xj01, "ha_xj02": &p.xj02,
	} {
		var u sysmodel.SysUser
		if err := db.Select("id").Where("tenant_id = ? AND username = ?", tid, uname).First(&u).Error; err != nil {
			return fmt.Errorf("演示账号缺失: %s", uname)
		}
		*dst = u.ID
	}
	// 楼栋（1/2/3 栋，按 sort）
	var buildings []insmodel.Building
	if err := db.Select("id").Where("community_id = ? AND type = ?", cid, "building").Order("sort").Find(&buildings).Error; err != nil {
		return err
	}
	if len(buildings) < 3 {
		return fmt.Errorf("演示楼栋不足 3 栋，无法落消防点位")
	}
	buildingIDs := []string{buildings[0].ID, buildings[1].ID, buildings[2].ID}
	// 巡逻路线点位（现有非消防点位，按 sort 即巡逻顺序）
	var points []insmodel.InspectionPoint
	if err := db.Where("community_id = ? AND type NOT IN ?", cid, []string{"fire_cabinet", "fire_extinguisher"}).
		Order("sort").Find(&points).Error; err != nil {
		return err
	}
	if len(points) == 0 {
		return fmt.Errorf("演示巡逻路线点位缺失")
	}
	var routePointIDs []string
	routeMeta := map[string]demoPoint{}
	for _, pt := range points {
		routePointIDs = append(routePointIDs, pt.ID)
		routeMeta[pt.ID] = demoPoint{
			name: pt.Name, typ: pt.Type, credential: pt.Credential, buildingIdx: -1,
			lng: pt.Longitude, lat: pt.Latitude, fence: pt.FenceRadius,
		}
	}
	// 安全/设备模板 ID（仅作存在性校验，项快照由 demoTemplateItemsOf 提供）
	tplSafetyID := templateIDByName(db, tid, "安全巡查通用模板")
	tplEquipID := templateIDByName(db, tid, "设备设施专项模板")

	d := &demoSeeder{db: db, store: store, photoCache: map[string]demoPhotoRef{}, orderSeq: map[string]int{}}
	return db.Transaction(func(tx *gorm.DB) error {
		d.db = tx
		return d.seedFireSpecial(tid, cid, buildingIDs, routePointIDs, routeMeta, tplSafetyID, tplEquipID, p)
	})
}

// templateIDByName 按名称查模板 ID（查不到返回空串，由调用方决定成败）。
func templateIDByName(db *gorm.DB, tid, name string) string {
	var tpl insmodel.CheckTemplate
	if err := db.Select("id").Where("tenant_id = ? AND name = ?", tid, name).First(&tpl).Error; err != nil {
		return ""
	}
	return tpl.ID
}

// seedFireSpecial 消防专项+两班倒全量演示数据（主种子/回填共用；调用方保证幂等）。
func (d *demoSeeder) seedFireSpecial(tid, cid string, buildingIDs, routePointIDs []string,
	routeMeta map[string]demoPoint, tplSafetyID, tplEquipID string, p firePeople) error {

	// 1) 消防检查模板（point_type 绑定 + ai_hint）
	tplCabinetID, cabinetReq, err := d.createFireTemplate(tid, fireTplCabinetName, "fire_cabinet", fireCabinetItems)
	if err != nil {
		return err
	}
	tplExtID, extReq, err := d.createFireTemplate(tid, fireTplExtName, "fire_extinguisher", fireExtItems)
	if err != nil {
		return err
	}

	// 2) 消防点位：每栋每层楼 1 个消防箱 + 每栋 2 个灭火器点（坐标沿小区楼栋位置微调）
	bldBase := [][2]float64{{114.39855, 30.50508}, {114.39872, 30.50545}, {114.39830, 30.50562}}
	var firePointIDs []string
	fireMeta := map[string]demoPoint{}
	sort := 8 // 现有 7 个巡逻点位之后
	for bi, base := range bldBase {
		for f := 1; f <= fireFloorsPerBld; f++ {
			dp := demoPoint{
				name: fmt.Sprintf("%d栋%d层消防箱", bi+1, f), typ: "fire_cabinet",
				credential: insmodel.CredentialQRCode, buildingIdx: bi,
				lng: base[0] + float64(f)*0.00001, lat: base[1] + float64(f)*0.00001, fence: 80,
			}
			id, err := d.createPoint(tid, cid, buildingIDs, dp, tplCabinetID, cabinetReq, sort)
			if err != nil {
				return err
			}
			firePointIDs = append(firePointIDs, id)
			fireMeta[id] = dp
			sort++
		}
		for _, suf := range []struct {
			name        string
			dlng, dlat  float64
		}{{"大堂", -0.00004, -0.00003}, {"顶层", 0.00005, 0.00006}} {
			dp := demoPoint{
				name: fmt.Sprintf("%d栋%s灭火器点", bi+1, suf.name), typ: "fire_extinguisher",
				credential: insmodel.CredentialQRCode, buildingIdx: bi,
				lng: base[0] + suf.dlng, lat: base[1] + suf.dlat, fence: 80,
			}
			id, err := d.createPoint(tid, cid, buildingIDs, dp, tplExtID, extReq, sort)
			if err != nil {
				return err
			}
			firePointIDs = append(firePointIDs, id)
			fireMeta[id] = dp
			sort++
		}
	}

	// 3) 计划：两班倒巡更（安全巡查，轮次×巡检员生成任务）+ 消防设施月度专项（按点位类型圈选）
	planPatrol := insmodel.InspectionPlan{
		TenantID: &tid, CommunityID: cid, Name: firePlanPatrolName, PatrolType: insmodel.PatrolSafety,
		PointIDs: types.IDArray(routePointIDs), CycleType: "daily",
		CycleConfig: types.JSONMap{
			"interval": 1,
			"rounds": []any{
				map[string]any{"name": "白班", "window": "07:00-19:00"},
				map[string]any{"name": "夜班", "window": "19:00-07:00"},
			},
			"daily_min_rounds": 2,
		},
		InspectorIDs: types.IDArray{p.xj01, p.xj02},
		StartDate:    daysAgo(3),
		Status:       sysmodel.StatusEnabled, Remark: "演示计划（seed-demo 生成）",
	}
	if err := d.db.Create(&planPatrol).Error; err != nil {
		return err
	}
	planFire := insmodel.InspectionPlan{
		TenantID: &tid, CommunityID: cid, Name: firePlanMonthName, PatrolType: "fire",
		CycleType: "monthly", CycleConfig: types.JSONMap{"days": []int{1}},
		SelectionMode: insmodel.SelectionByPointTypes, PointTypes: types.StringArray{"fire_cabinet", "fire_extinguisher"},
		InspectorIDs:  types.IDArray{p.xj01},
		StartDate:     monthDayAt(1, 0, 0), TimeWindow: "09:00-17:00",
		Status: sysmodel.StatusEnabled, Remark: "演示计划（seed-demo 生成）",
	}
	if err := d.db.Create(&planFire).Error; err != nil {
		return err
	}

	// 4) 两班倒近 3 天执行 + 今日任务（白班 doing / 夜班 pending，含 1 条夜班漏巡）
	if err := d.seedPatrolRounds(tid, cid, planPatrol.ID, routePointIDs, routeMeta, tplSafetyID, tplEquipID,
		[]string{p.xj01, p.xj02}); err != nil {
		return err
	}

	// 5) 当月消防专项任务（全部完成，2 处异常）+ 异常工单（工程条线 1 闭环 1 处理中）+ 当月专项检查报告
	return d.seedFireMonth(tid, cid, planFire.ID, firePointIDs, fireMeta, p)
}

// createFireTemplate 专项检查模板（point_type 绑定 + ai_hint），返回模板 ID 与必拍项名称列表。
func (d *demoSeeder) createFireTemplate(tenantID, name, pointType string, items []fireTplItem) (string, []string, error) {
	tpl := insmodel.CheckTemplate{
		TenantID: &tenantID, Name: name, PointType: pointType, Sort: 10,
		Status: sysmodel.StatusEnabled, Remark: "演示模板（seed-demo 生成）",
	}
	if err := d.db.Create(&tpl).Error; err != nil {
		return "", nil, err
	}
	var required []string
	for i, it := range items {
		row := insmodel.CheckTemplateItem{
			TemplateID: tpl.ID, Name: it.name, Requirement: strptr(it.requirement),
			Required: true, PhotoRequired: it.photoReq, Sort: i + 1,
		}
		if it.aiHint != "" {
			row.AIHint = strptr(it.aiHint)
		}
		if err := d.db.Create(&row).Error; err != nil {
			return "", nil, err
		}
		if it.photoReq == types.PhotoReqRequired {
			required = append(required, it.name)
		}
	}
	return tpl.ID, required, nil
}

// seedPatrolRounds 两班倒轮次任务与打卡：近 3 天每天 白班/夜班 × 双巡检员
// （白班全 done；夜班 3 天中 1 条漏巡 overdue，其余 done）；今日白班 doing（已打部分点）、夜班 pending。
func (d *demoSeeder) seedPatrolRounds(tid, cid, planID string, routePointIDs []string,
	routeMeta map[string]demoPoint, tplSafetyID, tplEquipID string, inspectors []string) error {
	rounds := []struct {
		name, window string
		startHour    int
	}{
		{"白班", "07:00-19:00", 7},
		{"夜班", "19:00-07:00", 19},
	}
	newTask := func(uid string, date time.Time, rd struct {
		name, window string
		startHour    int
	}) insmodel.InspectionTask {
		return insmodel.InspectionTask{
			TenantID: &tid, PlanID: planID, CommunityID: cid, InspectorID: uid,
			PatrolType: insmodel.PatrolSafety, TaskDate: date,
			RoundName: rd.name, TimeWindow: rd.window,
			PointIDs: types.IDArray(routePointIDs), TotalPoints: len(routePointIDs),
		}
	}
	var recs []insmodel.CheckinRecord
	collect := func(taskID, uid string, startT time.Time, upto int) error {
		for pi := 0; pi < upto; pi++ {
			pid := routePointIDs[pi]
			pm := routeMeta[pid]
			ct := startT.Add(time.Duration(5+pi*13) * time.Minute)
			rec := insmodel.CheckinRecord{
				TenantID: &tid, TaskID: taskID, PointID: pid, InspectorID: uid, CommunityID: cid,
				CheckinTime: ct, ClientTime: &ct,
				Longitude: floatptr(pm.lng + 0.00003), Latitude: floatptr(pm.lat + 0.00002),
				DistanceToPoint: floatptr(3.5), CheckinType: insmodel.CredentialQRCode,
				Result: insmodel.ResultNormal, AuditStatus: insmodel.AuditAutoPass, CreatedAt: ct,
			}
			if pm.credential == insmodel.CredentialNone {
				rec.CheckinType = "fence"
			}
			tpl, ok := demoTemplateItemsOf(pm.typ, tplSafetyID, tplEquipID)
			if !ok {
				return fmt.Errorf("演示点位模板缺失: %s", pm.name)
			}
			for _, label := range tpl.requiredPhotos {
				ph, _ := d.photo(tid, uid, label)
				if ph.URL != "" {
					rec.Photos = append(rec.Photos, ph)
				}
			}
			recs = append(recs, rec)
		}
		return nil
	}

	// 近 3 天：白班全 done；夜班仅「昨天夜班赵敏」漏巡 overdue，其余 done
	for dayOff := 3; dayOff >= 1; dayOff-- {
		date := daysAgo(dayOff)
		for ri, rd := range rounds {
			for ii, uid := range inspectors {
				task := newTask(uid, date, rd)
				if dayOff == 1 && ri == 1 && ii == len(inspectors)-1 {
					task.Status = insmodel.TaskOverdue
					if err := d.db.Create(&task).Error; err != nil {
						return err
					}
					continue
				}
				startT := dayAt(dayOff, rd.startHour, 5+ii*20)
				endT := startT.Add(100 * time.Minute)
				task.Status = insmodel.TaskDone
				task.DonePoints = len(routePointIDs)
				task.StartedAt = &startT
				task.FinishedAt = &endT
				if err := d.db.Create(&task).Error; err != nil {
					return err
				}
				if err := collect(task.ID, uid, startT, len(routePointIDs)); err != nil {
					return err
				}
			}
		}
	}

	// 今日：白班 doing（陈刚打了 3 个点、赵敏打了 1 个），夜班 pending
	for i, part := range []struct {
		done, startMin int
	}{{3, 5}, {1, 25}} {
		uid := inspectors[i]
		startT := todayAt(rounds[0].startHour, part.startMin)
		task := newTask(uid, daysAgo(0), rounds[0])
		task.Status = insmodel.TaskDoing
		task.DonePoints = part.done
		task.StartedAt = &startT
		if err := d.db.Create(&task).Error; err != nil {
			return err
		}
		if err := collect(task.ID, uid, startT, part.done); err != nil {
			return err
		}
	}
	for _, uid := range inspectors {
		task := newTask(uid, daysAgo(0), rounds[1])
		task.Status = insmodel.TaskPending
		if err := d.db.Create(&task).Error; err != nil {
			return err
		}
	}

	if err := d.db.CreateInBatches(&recs, 200).Error; err != nil {
		return err
	}
	// 逐项结果快照（全部合格；照片挂必拍项）
	var items []insmodel.CheckinRecordItem
	for _, rec := range recs {
		tpl, _ := demoTemplateItemsOf(routeMeta[rec.PointID].typ, tplSafetyID, tplEquipID)
		for j, it := range tpl.items {
			row := insmodel.CheckinRecordItem{
				RecordID: rec.ID, Name: it.name, Requirement: strptr(it.requirement),
				PhotoRequired: it.photoReq, Pass: true, Sort: j + 1, CreatedAt: rec.CheckinTime,
			}
			if it.photoReq == types.PhotoReqRequired {
				if _, key := d.photo(tid, rec.InspectorID, it.name); key != "" {
					row.Photos = types.StringArray{key}
				}
			}
			items = append(items, row)
		}
	}
	return d.db.CreateInBatches(&items, 200).Error
}

// seedFireMonth 当月消防专项：任务全部完成（24 点逐点打卡，2 处故意异常）→ 异常工单转工程条线
// （1 闭环 1 处理中）→ 当月专项检查报告（approved，三级签字齐全；主管级=工程主管，对应 patrol_report_line.fire 槽位）。
func (d *demoSeeder) seedFireMonth(tid, cid, planID string, firePointIDs []string,
	fireMeta map[string]demoPoint, p firePeople) error {

	startT := monthDayAt(1, 9, 0)
	endT := monthDayAt(1, 11, 40)
	task := insmodel.InspectionTask{
		TenantID: &tid, PlanID: planID, CommunityID: cid, InspectorID: p.xj01,
		PatrolType: "fire", TaskDate: monthDayAt(1, 0, 0),
		PointIDs: types.IDArray(firePointIDs),
		Status:   insmodel.TaskDone, TotalPoints: len(firePointIDs), DonePoints: len(firePointIDs),
		StartedAt: &startT, FinishedAt: &endT,
	}
	if err := d.db.Create(&task).Error; err != nil {
		return err
	}

	// 2 处故意异常（整改清单样例）+ 逐项 AI 初判断演示值
	type abSpec struct{ itemName, note, remark string }
	abs := map[string]abSpec{
		"2栋3层消防箱":   {"消防枪头在位", "消防枪头缺失，需补充", "消防枪头缺失，已拍照上报转工单"},
		"1栋大堂灭火器点": {"在有效期内", "铭牌显示有效期至 2026-06，已过期", "灭火器已过有效期，已拍照上报转工单"},
	}
	aiDemo := map[string]map[string][2]string{ // 点位 → 检查项 → {verdict, reason}
		"2栋3层消防箱":   {"消防枪头在位": {insmodel.AIVerdictReview, "未在画面中识别到消防枪头"}},
		"1栋大堂灭火器点": {"在有效期内": {insmodel.AIVerdictReview, "铭牌显示有效期至 2026-06，已过期"}},
		"1栋1层消防箱":   {"消防枪头在位": {insmodel.AIVerdictPass, "画面中可见消防枪头，安装在位"}},
	}

	abRec := map[string]string{} // 点位名 → 异常打卡记录 ID
	abPt := map[string]string{}  // 点位名 → 点位 ID
	for i, pid := range firePointIDs {
		pm := fireMeta[pid]
		ct := startT.Add(time.Duration(5+i*6) * time.Minute)
		items, reqLabels := fireCabinetItems, []string(nil)
		if pm.typ == "fire_extinguisher" {
			items = fireExtItems
		}
		for _, it := range items {
			if it.photoReq == types.PhotoReqRequired {
				reqLabels = append(reqLabels, it.name)
			}
		}
		ab, isAb := abs[pm.name]
		rec := insmodel.CheckinRecord{
			TenantID: &tid, TaskID: task.ID, PointID: pid, InspectorID: p.xj01, CommunityID: cid,
			CheckinTime: ct, ClientTime: &ct,
			Longitude: floatptr(pm.lng + 0.00003), Latitude: floatptr(pm.lat + 0.00002),
			DistanceToPoint: floatptr(3.2), CheckinType: insmodel.CredentialQRCode,
			Result: insmodel.ResultNormal, AuditStatus: insmodel.AuditAutoPass, CreatedAt: ct,
		}
		if isAb {
			rec.Result = insmodel.ResultAbnormal
			rec.Remark = ab.remark
		}
		for _, label := range reqLabels {
			ph, _ := d.photo(tid, p.xj01, label)
			if ph.URL != "" {
				rec.Photos = append(rec.Photos, ph)
			}
		}
		if err := d.db.Create(&rec).Error; err != nil {
			return err
		}
		for j, it := range items {
			row := insmodel.CheckinRecordItem{
				RecordID: rec.ID, Name: it.name, Requirement: strptr(it.requirement),
				PhotoRequired: it.photoReq, Pass: true, Sort: j + 1, CreatedAt: ct,
			}
			if it.aiHint != "" {
				row.AIHint = strptr(it.aiHint)
			}
			if it.photoReq == types.PhotoReqRequired {
				if _, key := d.photo(tid, p.xj01, it.name); key != "" {
					row.Photos = types.StringArray{key}
				}
			}
			if isAb && it.name == ab.itemName {
				row.Pass = false
				row.Note = ab.note
			}
			if v, ok := aiDemo[pm.name][it.name]; ok {
				row.AIVerdict = strptr(v[0])
				row.AIReason = strptr(v[1])
			}
			if err := d.db.Create(&row).Error; err != nil {
				return err
			}
		}
		if isAb {
			abRec[pm.name] = rec.ID
			abPt[pm.name] = pid
		}
	}

	// 异常工单（视同已分诊，工程条线闭环：周建国派单 → 吴永强处理 → 陈刚验收）
	cabPoint, cabRec := abPt["2栋3层消防箱"], abRec["2栋3层消防箱"]
	extPoint, extRec := abPt["1栋大堂灭火器点"], abRec["1栋大堂灭火器点"]
	woF1Items := types.OrderItemArray{{Name: "消防枪头在位", Remark: "消防枪头缺失，需补充"}}
	if _, key := d.photo(tid, p.xj01, "消防枪头在位"); key != "" {
		woF1Items[0].BeforePhotos = types.StringArray{key}
	}
	woF1 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(monthDayAt(1, 0, 0)), CommunityID: cid,
		PointID: &cabPoint, CheckinID: &cabRec,
		Title: "2栋3层消防箱枪头缺失", Description: "消防专项检查发现 2栋3层消防箱内消防枪头缺失，需补充同型号枪头。",
		Source: womodel.SourceInspection, Category: "消防设施", ReporterID: p.xj01,
		Photos:     d.photosOf(tid, p.xj01, "消防枪头在位"),
		Items:      woF1Items,
		AssigneeID: &p.repair, DispatcherID: &p.eng, Priority: "high", Status: womodel.OrderClosed,
		DispatchAt: at(monthDayAt(1, 14, 0)), AcceptAt: at(monthDayAt(1, 14, 0)),
		FinishNote: "已补充同型号消防枪头并复位，箱内器材齐全。", FinishAt: at(monthDayAt(2, 10, 30)),
		ConfirmBy:  &p.xj01, ConfirmAt: at(monthDayAt(3, 9, 0)), ConfirmNote: "现场复核枪头在位，验收通过",
		CreatedAt:  monthDayAt(1, 11, 50), UpdatedAt: monthDayAt(3, 9, 0),
	}
	if err := d.createOrderWithLogs(woF1, []womodel.WorkOrderLog{
		{Action: womodel.ActionCreate, OperatorID: p.xj01, Detail: "巡检异常上报，自动生成工单（视同已分诊）", CreatedAt: monthDayAt(1, 11, 50)},
		{Action: womodel.ActionDispatch, OperatorID: p.eng, Detail: "派单给吴永强", CreatedAt: monthDayAt(1, 14, 0)},
		{Action: womodel.ActionFinish, OperatorID: p.repair, Detail: "完工：补充消防枪头", CreatedAt: monthDayAt(2, 10, 30)},
		{Action: womodel.ActionConfirmPass, OperatorID: p.xj01, Detail: "验收通过", CreatedAt: monthDayAt(3, 9, 0)},
	}); err != nil {
		return err
	}
	woF2Items := types.OrderItemArray{{Name: "在有效期内", Remark: "铭牌显示有效期至 2026-06，已过期"}}
	if _, key := d.photo(tid, p.xj01, "在有效期内"); key != "" {
		woF2Items[0].BeforePhotos = types.StringArray{key}
	}
	woF2 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(monthDayAt(1, 0, 0)), CommunityID: cid,
		PointID: &extPoint, CheckinID: &extRec,
		Title: "1栋大堂灭火器过期更换", Description: "消防专项检查发现 1栋大堂灭火器已过有效期（铭牌 2026-06 到期），需更换。",
		Source: womodel.SourceInspection, Category: "消防设施", ReporterID: p.xj01,
		Photos:     d.photosOf(tid, p.xj01, "在有效期内"),
		Items:      woF2Items,
		AssigneeID: &p.repair, DispatcherID: &p.eng, Priority: "high", Status: womodel.OrderProcessing,
		DispatchAt: at(monthDayAt(1, 14, 10)), AcceptAt: at(monthDayAt(1, 14, 10)),
		CreatedAt:  monthDayAt(1, 11, 55), UpdatedAt: monthDayAt(1, 14, 10),
	}
	if err := d.createOrderWithLogs(woF2, []womodel.WorkOrderLog{
		{Action: womodel.ActionCreate, OperatorID: p.xj01, Detail: "巡检异常上报，自动生成工单（视同已分诊）", CreatedAt: monthDayAt(1, 11, 55)},
		{Action: womodel.ActionDispatch, OperatorID: p.eng, Detail: "派单给吴永强", CreatedAt: monthDayAt(1, 14, 10)},
	}); err != nil {
		return err
	}

	// 当月专项检查报告（stats 快照与落库数据一致；PDF 明细按报告期实时查询，无需预生成 file_key）
	period := curMonthPeriod()
	_, keyGun := d.photo(tid, p.xj01, "消防枪头在位")
	_, keyGauge := d.photo(tid, p.xj01, "压力表指针在绿区")
	var photoKeys []string
	if keyGun != "" && keyGauge != "" {
		photoKeys = []string{keyGun, keyGauge}
	}
	fireRecView := func(hour, min int, point, result string) map[string]any {
		return map[string]any{
			"checkin_time": timefmt.T(monthDayAt(1, hour, min)), "inspector_name": "陈刚",
			"point_name": point, "checkin_type": "qrcode", "distance": 3.1,
			"result": result, "is_suspect": false,
			"audit_status": insmodel.AuditAutoPass, "photo_keys": photoKeys,
		}
	}
	records := []any{
		fireRecView(9, 5, "1栋1层消防箱", insmodel.ResultNormal),
		fireRecView(9, 47, "2栋3层消防箱", insmodel.ResultAbnormal),
		fireRecView(10, 53, "1栋大堂灭火器点", insmodel.ResultAbnormal),
	}
	total := int64(len(firePointIDs))
	stats := types.JSONMap{
		"task_total": int64(1), "task_done": int64(1), "task_overdue": int64(0),
		"should_points": total, "done_points": total, "coverage_rate": float64(100),
		"abnormal_count": int64(2), "suspect_count": int64(0),
		"wo_created": int64(2), "wo_closed": int64(1), "wo_unclosed": int64(1),
		"wo_close_rate": pct1(1, 2),
		"daily": []any{map[string]any{
			"date": period + "-01", "task_total": int64(1), "task_done": int64(1), "abnormal": int64(2),
		}},
		"records": records,
	}
	svAt := monthDayAt(2, 14, 0)
	mgAt := monthDayAt(3, 10, 0)
	report := rptmodel.InspectionReport{
		TenantID: &tid, CommunityID: cid, Period: period, PatrolType: "fire", PlanID: &planID,
		Title:  fireReportTitle("锦绣华庭", period),
		Status: rptmodel.StatusApproved, Stats: stats,
		InspectorIDs: types.IDArray{p.xj01},
		InspectorSigned: types.SignArray{
			{UserID: p.xj01, Name: "陈刚", SignedAt: timefmt.T(monthDayAt(2, 9, 10))},
		},
		SupervisorIDs: types.IDArray{p.eng},
		SupervisorBy:  &p.eng, SupervisorAt: &svAt, SupervisorRemark: "数据属实，2 处异常已转工程条线整改",
		ManagerIDs:    types.IDArray{p.manager},
		ManagerBy:     &p.manager, ManagerAt: &mgAt, ManagerRemark: "审核通过，归档",
	}
	return d.db.Create(&report).Error
}

// curMonthPeriod 当月期间（YYYY-MM；消防专项演示挂当月，任何时候执行都是「本月」）。
func curMonthPeriod() string { return time.Now().Format("2006-01") }

// fireReportTitle 消防专项检查报告标题（与 report_service.specialReportTitle 对「消防设施专项」的拼法一致）。
func fireReportTitle(commName, period string) string {
	t, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return commName + period + "消防设施专项检查报告"
	}
	return fmt.Sprintf("%s%d年%d月消防设施专项检查报告", commName, t.Year(), int(t.Month()))
}

// dayAt n 天前某时刻。
func dayAt(n, hour, min int) time.Time {
	d := daysAgo(n)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, time.Local)
}
