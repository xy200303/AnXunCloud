// Package demo 演示数据播种（独立命令 cmd/seed-demo 调用）：与 server 主流程和系统预置 seed 完全解耦，
// 仅显式运行 seed-demo 命令时触发，不随服务启动执行。
// 内容：两家演示物业公司（租户）+ 小区/楼栋/编制/模板/点位/计划/任务/打卡/工单/公告，
// 覆盖工单六态与打卡正常/异常/逾期，用于演示与验收。幂等：演示租户已存在则整体跳过。
package demo

import (
	"bytes"
	"embed"
	"fmt"
	"hash/fnv"
	"time"

	"gorm.io/gorm"

	rptmodel "anxuncloud/internal/module/report/model"
	insmodel "anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	womodel "anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

//go:embed demoassets/*.jpg
var demoAssets embed.FS

// demoAssetFiles 内置演示照片（真实图片，随二进制分发，离线可用）。
var demoAssetFiles = []string{
	"corridor.jpg", "fireext.jpg", "pump.jpg", "meter.jpg",
	"garage.jpg", "gate.jpg", "garden.jpg", "lobby.jpg",
}

// demoAssetByLabel 检查项 → 照片偏好（尽量贴近场景，未命中按标签哈希分配）。
var demoAssetByLabel = map[string]string{
	"消防通道畅通无阻":  "corridor.jpg",
	"灭火器压力正常":   "fireext.jpg",
	"设备运行无异响":   "pump.jpg",
	"仪表读数在正常范围": "meter.jpg",
}

// DemoPassword 全部演示账号的统一密码（满足 8–32 位含字母数字策略）。
const DemoPassword = "Demo@12345"

// demoTenantACode 演示租户 A 的公司代码（幂等标记：存在即跳过）。
const demoTenantACode = "huaan"

type demoPhotoRef struct {
	item types.PhotoItem
	key  string
}

type demoSeeder struct {
	db         *gorm.DB
	store      *storage.Storage
	photoCache map[string]demoPhotoRef
	roleIDs    map[string]string
	pwdHash    string
	orderSeq   map[string]int // 工单号日内序号（uk_order_no 全局唯一，两个演示租户共享计数）
}

// Seed 写入演示数据。store 用于落演示打卡照片（仅本地存储驱动写盘，云存储跳过照片）。
func Seed(db *gorm.DB, store *storage.Storage) error {
	var count int64
	if err := db.Model(&sysmodel.Tenant{}).Where("code = ?", demoTenantACode).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	d := &demoSeeder{store: store, photoCache: map[string]demoPhotoRef{}, roleIDs: map[string]string{}, orderSeq: map[string]int{}}
	return db.Transaction(func(tx *gorm.DB) error {
		d.db = tx
		return d.run()
	})
}

func (d *demoSeeder) run() error {
	var roles []sysmodel.SysRole
	if err := d.db.Where("tenant_id IS NULL").Find(&roles).Error; err != nil {
		return err
	}
	for _, r := range roles {
		d.roleIDs[r.Code] = r.ID
	}
	hash, err := password.Hash(DemoPassword)
	if err != nil {
		return err
	}
	d.pwdHash = hash
	if err := d.seedTenantA(); err != nil {
		return err
	}
	return d.seedTenantB()
}

// ---------- 通用小工具 ----------

func strptr(s string) *string { return &s }

// createUser 创建演示账号（统一密码；roleIDs 为空则完全靠岗位绑定角色并集授权）。
func (d *demoSeeder) createUser(tenantID, username, name, phone string, roleIDs ...string) (string, error) {
	u := sysmodel.SysUser{
		TenantID: tenantID, Username: username, Password: d.pwdHash, Name: name,
		Phone: phone, RoleIDs: types.IDArray(roleIDs), Status: sysmodel.StatusEnabled,
		Remark: "演示账号（seed-demo 生成）",
	}
	if err := d.db.Create(&u).Error; err != nil {
		return "", err
	}
	return u.ID, nil
}

// addStaff 编制：一人多岗；楼管员可圈责任楼栋。
func (d *demoSeeder) addStaff(tenantID, projectID, userID string, posts []string, buildingIDs ...string) error {
	row := sysmodel.ProjectStaff{
		TenantID: &tenantID, ProjectID: projectID, UserID: userID,
		Posts: types.StringArray(posts), BuildingIDs: types.IDArray(buildingIDs),
		Status: sysmodel.StatusEnabled,
	}
	return d.db.Create(&row).Error
}

// photo 生成/复用演示打卡照片（真实图片内置于 demoassets，仅本地存储驱动落盘；返回照片元素与 file_key）。
func (d *demoSeeder) photo(tenantID, ownerID, label string) (types.PhotoItem, string) {
	if d.store == nil || !d.store.IsDev() {
		return types.PhotoItem{}, ""
	}
	if p, ok := d.photoCache[label]; ok {
		return p.item, p.key
	}
	data, err := demoAssetByLabel_(label)
	if err != nil {
		return types.PhotoItem{}, ""
	}
	key, url, data, md5, err := d.store.Save("checkin", ownerID, "jpg", bytes.NewReader(data))
	if err != nil {
		return types.PhotoItem{}, ""
	}
	rec := sysmodel.UploadFile{
		TenantID: &tenantID, FileKey: key, Scene: "checkin", UserID: ownerID,
		Size: int64(len(data)), MimeType: "image/jpeg", URL: url, WatermarkedURL: url,
		Name: label + ".jpg", MD5: md5, Storage: d.store.DriverName(),
	}
	if err := d.db.Create(&rec).Error; err != nil {
		return types.PhotoItem{}, ""
	}
	ref := demoPhotoRef{item: types.PhotoItem{Item: label, URL: url, WatermarkedURL: url, Required: true}, key: key}
	d.photoCache[label] = ref
	return ref.item, ref.key
}

// demoAssetByLabel_ 按标签取内置演示照片字节。
func demoAssetByLabel_(label string) ([]byte, error) {
	name, ok := demoAssetByLabel[label]
	if !ok {
		h := fnv.New32a()
		_, _ = h.Write([]byte(label))
		name = demoAssetFiles[h.Sum32()%uint32(len(demoAssetFiles))]
	}
	return demoAssets.ReadFile("demoassets/" + name)
}

type demoTemplateItem struct {
	name, requirement, photoReq string
}

// createTemplate 检查项模板 + 项行，返回模板 ID 与必拍项名称列表。
func (d *demoSeeder) createTemplate(tenantID, name string, items []demoTemplateItem) (string, []string, error) {
	tpl := insmodel.CheckTemplate{
		TenantID: &tenantID, Name: name, Sort: 1,
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
		if err := d.db.Create(&row).Error; err != nil {
			return "", nil, err
		}
		if it.photoReq == types.PhotoReqRequired {
			required = append(required, it.name)
		}
	}
	return tpl.ID, required, nil
}

type demoPoint struct {
	name, typ, credential string
	buildingIdx           int // -1 = 不挂楼栋
	lng, lat              float64
	fence                 int
}

// createPoint 点位（每个点位都分配二维码编号，与 point_service 一致：P+6 位序列，无分隔符）。
func (d *demoSeeder) createPoint(tenantID, communityID string, buildingIDs []string, p demoPoint, templateID string, requiredPhotos []string, sort int) (string, error) {
	var seq int64
	if err := d.db.Raw("SELECT nextval('qrcode_no_seq')").Scan(&seq).Error; err != nil {
		return "", err
	}
	pt := insmodel.InspectionPoint{
		TenantID: &tenantID, CommunityID: communityID, Name: p.name, Type: p.typ,
		QRCodeNo: fmt.Sprintf("P%06d", seq), Longitude: p.lng, Latitude: p.lat, FenceRadius: p.fence,
		Credential: p.credential, RequireFence: true,
		RequiredPhotoItems: types.StringArray(requiredPhotos),
		Sort:               sort, Status: sysmodel.StatusEnabled, Remark: "演示点位（seed-demo 生成）",
	}
	if templateID != "" {
		pt.TemplateID = &templateID
	}
	if p.buildingIdx >= 0 && p.buildingIdx < len(buildingIDs) {
		pt.BuildingID = &buildingIDs[p.buildingIdx]
	}
	if err := d.db.Create(&pt).Error; err != nil {
		return "", err
	}
	return pt.ID, nil
}

// ---------- 租户 A：华安物业（完整演示） ----------

func (d *demoSeeder) seedTenantA() error {
	tenant := sysmodel.Tenant{
		Code: demoTenantACode, Name: "华安物业管理有限公司",
		ContactName: "王建国", ContactPhone: "027-87651234",
		Status: sysmodel.StatusEnabled, Remark: "演示租户（seed-demo 生成）",
	}
	if err := d.db.Create(&tenant).Error; err != nil {
		return err
	}
	tid := tenant.ID
	if err := systemsvc.CopyPostTemplatesToTenant(d.db, tid); err != nil {
		return err
	}

	// 账号（统一密码 Demo@12345；除租户管理员外不显式挂角色，演示岗位绑定角色的实时并集）
	adminID, err := d.createUser(tid, "huaan_admin", "王建国", "13800001001", d.roleIDs[sysmodel.TenantAdminCode])
	if err != nil {
		return err
	}
	managerID, err := d.createUser(tid, "ha_manager", "张伟", "13800001002")
	if err != nil {
		return err
	}
	safetyID, err := d.createUser(tid, "ha_safety", "李强", "13800001003")
	if err != nil {
		return err
	}
	xj01ID, err := d.createUser(tid, "ha_xj01", "陈刚", "13800001004")
	if err != nil {
		return err
	}
	xj02ID, err := d.createUser(tid, "ha_xj02", "赵敏", "13800001005")
	if err != nil {
		return err
	}
	engID, err := d.createUser(tid, "ha_eng", "周建国", "13800001006")
	if err != nil {
		return err
	}
	repairID, err := d.createUser(tid, "ha_repair", "吴永强", "13800001007")
	if err != nil {
		return err
	}
	serviceID, err := d.createUser(tid, "ha_service", "刘芳", "13800001008")
	if err != nil {
		return err
	}
	bmID, err := d.createUser(tid, "ha_bm", "孙丽", "13800001009")
	if err != nil {
		return err
	}
	fdID, err := d.createUser(tid, "ha_fd", "钱婷", "13800001010")
	if err != nil {
		return err
	}

	// 小区 + 楼栋
	community := sysmodel.Community{
		TenantID: tid, Name: "锦绣华庭", Address: "武汉市洪山区光谷大道 88 号",
		ManagerID: &managerID, WoTriageEnabled: true,
		Status: sysmodel.StatusEnabled, Remark: "演示小区（seed-demo 生成）",
	}
	if err := d.db.Create(&community).Error; err != nil {
		return err
	}
	cid := community.ID
	var buildingIDs []string
	for i, name := range []string{"1 栋", "2 栋", "3 栋"} {
		b := insmodel.Building{
			TenantID: &tid, CommunityID: cid, Name: name, Type: "building",
			Sort: i + 1, Status: sysmodel.StatusEnabled,
		}
		if err := d.db.Create(&b).Error; err != nil {
			return err
		}
		buildingIDs = append(buildingIDs, b.ID)
	}

	// 编制（一人多岗、责任楼栋均在编制体现）
	staff := []struct {
		uid       string
		posts     []string
		buildings []string
	}{
		{managerID, []string{sysmodel.PostProjectManager}, nil},
		{safetyID, []string{"safety_supervisor"}, nil},
		{xj01ID, []string{sysmodel.PostInspector}, nil},
		{xj02ID, []string{sysmodel.PostInspector}, nil},
		{engID, []string{"engineering_supervisor"}, nil},
		{repairID, []string{sysmodel.PostRepairman}, nil},
		{serviceID, []string{"service_supervisor"}, nil},
		{bmID, []string{"building_manager"}, buildingIDs[:2]},
		{fdID, []string{"receptionist"}, nil},
	}
	for _, s := range staff {
		if err := d.addStaff(tid, cid, s.uid, s.posts, s.buildings...); err != nil {
			return err
		}
	}

	// 检查项模板
	tplSafetyID, safetyRequired, err := d.createTemplate(tid, "安全巡查通用模板", []demoTemplateItem{
		{"消防通道畅通无阻", "通道无杂物堆放，安全出口标识清晰、应急照明正常", types.PhotoReqRequired},
		{"灭火器压力正常", "指针在绿色区域，铅封完好，在有效期内", types.PhotoReqRequired},
		{"无违规用电", "无私拉电线、无大功率违规电器", types.PhotoReqOptional},
		{"门禁与监控运行正常", "门禁刷卡正常，监控画面清晰无遮挡", types.PhotoReqNone},
	})
	if err != nil {
		return err
	}
	tplEquipID, equipRequired, err := d.createTemplate(tid, "设备设施专项模板", []demoTemplateItem{
		{"设备运行无异响", "运行声音平稳，无异常振动", types.PhotoReqRequired},
		{"仪表读数在正常范围", "压力表/温度表/电压表读数在额定区间内", types.PhotoReqRequired},
		{"无跑冒滴漏", "管路、阀门、泵体无渗漏", types.PhotoReqOptional},
	})
	if err != nil {
		return err
	}

	// 点位（坐标取武汉光谷一带，彼此偏移数十米）
	pointDefs := []struct {
		demoPoint
		template string
		required []string
	}{
		{demoPoint{"消防控制室", "fire_control", insmodel.CredentialQRCode, 0, 114.39850, 30.50520, 80}, tplSafetyID, safetyRequired},
		{demoPoint{"配电房", "power_room", insmodel.CredentialQRCode, 0, 114.39862, 30.50532, 80}, tplEquipID, equipRequired},
		{demoPoint{"水泵房", "pump_room", insmodel.CredentialQRCode, 1, 114.39831, 30.50541, 80}, tplEquipID, equipRequired},
		{demoPoint{"1 栋电梯机房", "elevator", insmodel.CredentialQRCode, 0, 114.39855, 30.50508, 100}, tplEquipID, equipRequired},
		{demoPoint{"地下车库出入口", "garage", insmodel.CredentialQRCode, -1, 114.39890, 30.50555, 100}, tplSafetyID, safetyRequired},
		{demoPoint{"东门岗亭", "common", insmodel.CredentialAny, -1, 114.39920, 30.50530, 150}, tplSafetyID, safetyRequired},
		{demoPoint{"园区北门", "common", insmodel.CredentialNone, -1, 114.39840, 30.50580, 150}, tplSafetyID, safetyRequired},
	}
	var pointIDs []string
	pointMeta := map[string]demoPoint{}
	for i, pd := range pointDefs {
		id, err := d.createPoint(tid, cid, buildingIDs, pd.demoPoint, pd.template, pd.required, i+1)
		if err != nil {
			return err
		}
		pointIDs = append(pointIDs, id)
		pointMeta[id] = pd.demoPoint
	}

	// 计划
	planSafety := insmodel.InspectionPlan{
		TenantID: &tid, CommunityID: cid, Name: "每日安全巡查", PatrolType: insmodel.PatrolSafety,
		PointIDs: types.IDArray(pointIDs), CycleType: "daily", CycleConfig: types.JSONMap{"interval": 1},
		InspectorIDs: types.IDArray{xj01ID, xj02ID},
		StartDate:    daysAgo(7), TimeWindow: "08:00-20:00",
		Status: sysmodel.StatusEnabled, Remark: "演示计划（seed-demo 生成）",
	}
	if err := d.db.Create(&planSafety).Error; err != nil {
		return err
	}
	equipPoints := []string{pointIDs[1], pointIDs[2], pointIDs[3]}
	planEquip := insmodel.InspectionPlan{
		TenantID: &tid, CommunityID: cid, Name: "设备设施周检", PatrolType: insmodel.PatrolEquipment,
		PointIDs: types.IDArray(equipPoints), CycleType: "weekly", CycleConfig: types.JSONMap{"weekdays": []int{1}},
		InspectorIDs: types.IDArray{repairID},
		StartDate:    daysAgo(7), TimeWindow: "09:00-17:00",
		Status: sysmodel.StatusEnabled, Remark: "演示计划（seed-demo 生成）",
	}
	if err := d.db.Create(&planEquip).Error; err != nil {
		return err
	}

	// 任务：昨日 xj01 已完成（含 1 条异常打卡）、昨日 xj02 逾期未巡、今日两条待巡
	taskDone := insmodel.InspectionTask{
		TenantID: &tid, PlanID: planSafety.ID, CommunityID: cid, InspectorID: xj01ID,
		PatrolType: insmodel.PatrolSafety, TaskDate: daysAgo(1), Status: insmodel.TaskDone,
		TotalPoints: len(pointIDs), DonePoints: len(pointIDs),
		StartedAt: at(yesterdayAt(9, 2)), FinishedAt: at(yesterdayAt(10, 47)),
	}
	if err := d.db.Create(&taskDone).Error; err != nil {
		return err
	}
	taskOverdue := insmodel.InspectionTask{
		TenantID: &tid, PlanID: planSafety.ID, CommunityID: cid, InspectorID: xj02ID,
		PatrolType: insmodel.PatrolSafety, TaskDate: daysAgo(1), Status: insmodel.TaskOverdue,
		TotalPoints: len(pointIDs),
	}
	if err := d.db.Create(&taskOverdue).Error; err != nil {
		return err
	}
	for _, uid := range []string{xj01ID, xj02ID} {
		t := insmodel.InspectionTask{
			TenantID: &tid, PlanID: planSafety.ID, CommunityID: cid, InspectorID: uid,
			PatrolType: insmodel.PatrolSafety, TaskDate: daysAgo(0), Status: insmodel.TaskPending,
			TotalPoints: len(pointIDs),
		}
		if err := d.db.Create(&t).Error; err != nil {
			return err
		}
	}

	// 昨日 xj01 的 7 条打卡（地下车库一条异常 → 转工单 WO2）
	var abnormalCheckinID string
	for i, pid := range pointIDs {
		p := pointMeta[pid]
		checkinTime := yesterdayAt(9, 5+i*14)
		abnormal := p.typ == "garage"
		rec := insmodel.CheckinRecord{
			TenantID: &tid, TaskID: taskDone.ID, PointID: pid, InspectorID: xj01ID, CommunityID: cid,
			CheckinTime: checkinTime, ClientTime: &checkinTime,
			Longitude: floatptr(p.lng + 0.00003), Latitude: floatptr(p.lat + 0.00002),
			DistanceToPoint: floatptr(3.6), CheckinType: insmodel.CredentialQRCode,
			Result: insmodel.ResultNormal, AuditStatus: insmodel.AuditAutoPass,
			CreatedAt: checkinTime,
		}
		if p.credential == insmodel.CredentialNone {
			rec.CheckinType = "fence"
		}
		if abnormal {
			rec.Result = insmodel.ResultAbnormal
			rec.Remark = "车库 B 区一盏照明灯不亮，已拍照报修"
		}
		items, ok := demoTemplateItemsOf(p.typ, tplSafetyID, tplEquipID)
		if !ok {
			return fmt.Errorf("演示点位模板缺失: %s", p.name)
		}
		for _, label := range items.requiredPhotos {
			ph, _ := d.photo(tid, xj01ID, label)
			if ph.URL != "" {
				rec.Photos = append(rec.Photos, ph)
			}
		}
		if err := d.db.Create(&rec).Error; err != nil {
			return err
		}
		for j, it := range items.items {
			row := insmodel.CheckinRecordItem{
				RecordID: rec.ID, Name: it.name, Requirement: strptr(it.requirement),
				PhotoRequired: it.photoReq, Pass: true, Sort: j + 1, CreatedAt: checkinTime,
			}
			if it.photoReq == types.PhotoReqRequired {
				_, key := d.photo(tid, xj01ID, it.name)
				if key != "" {
					row.Photos = types.StringArray{key}
				}
			}
			if abnormal && it.name == "门禁与监控运行正常" {
				row.Pass = false
				row.Note = "B 区一盏照明灯损坏"
			}
			if err := d.db.Create(&row).Error; err != nil {
				return err
			}
		}
		if abnormal {
			abnormalCheckinID = rec.ID
		}
	}

	// 工单（六态全覆盖 + 已作废）
	if err := d.seedOrdersA(tid, cid, pointIDs, abnormalCheckinID, orderPeopleA{
		manager: managerID, eng: engID, repair: repairID, service: serviceID,
		bm: bmID, fd: fdID, xj01: xj01ID,
	}); err != nil {
		return err
	}

	// 月度报告：上月已三级签字归档
	if err := d.seedReportsA(tid, cid, managerID, safetyID, xj01ID, xj02ID); err != nil {
		return err
	}

	// 公告
	notice := sysmodel.SysNotice{
		TenantID: &tid, Title: "关于开展夏季消防安全专项检查的通知",
		Content:    "各岗位请注意：即日起至本月底开展夏季消防安全专项检查，重点检查消防通道、灭火器有效期与电动车违规充电情况，请各巡检员按每日安全巡查计划逐项落实，发现异常立即拍照上报并转工单处理。",
		Status:     1, PublishAt: at(daysAgo(2)), CreatedBy: &adminID, CreatedByName: "王建国",
	}
	return d.db.Create(&notice).Error
}

type orderPeopleA struct {
	manager, eng, repair, service, bm, fd, xj01 string
}

func (d *demoSeeder) seedOrdersA(tid, cid string, pointIDs []string, abnormalCheckinID string, p orderPeopleA) error {
	garagePoint := pointIDs[4]
	type logStep struct {
		action, operator, detail string
		at                       time.Time
	}
	create := func(o womodel.WorkOrder, logs []logStep) error {
		if err := d.db.Create(&o).Error; err != nil {
			return err
		}
		for _, l := range logs {
			row := womodel.WorkOrderLog{OrderID: o.ID, Action: l.action, OperatorID: l.operator, Detail: l.detail, CreatedAt: l.at}
			if err := d.db.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	}
	d3 := daysAgo(3)
	d1 := daysAgo(1)

	// WO1 已闭环：前台代录 → 分诊 → 派单 → 完工 → 验收通过
	wo1 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(d3), CommunityID: cid,
		Title: "水泵房地面渗水维修", Description: "前台接业主电话反映水泵房门口地面有渗水，请工程部核实处理。",
		Source: womodel.SourceFrontdesk, Category: "给排水", ReporterID: p.fd,
		AssigneeID: &p.repair, DispatcherID: &p.eng, Priority: "normal", Status: womodel.OrderClosed,
		TriageBy: &p.service, TriageAt: at(d3.Add(30 * time.Minute)), TriageNote: "属实，转工程维修",
		DispatchAt: at(d3.Add(time.Hour)), AcceptAt: at(d3.Add(time.Hour)),
		FinishNote: "更换老化密封圈，渗水已止，观察 24 小时无复发。", FinishAt: at(d1.Add(-2 * time.Hour)),
		ConfirmBy: &p.fd, ConfirmAt: at(d1), ConfirmNote: "现场复核无渗水，验收通过",
		CreatedAt: d3, UpdatedAt: d1,
	}
	if err := create(wo1, []logStep{
		{womodel.ActionCreate, p.fd, "前台代录工单", d3},
		{womodel.ActionTriagePass, p.service, "分诊通过：属实，转工程维修", d3.Add(30 * time.Minute)},
		{womodel.ActionDispatch, p.eng, "派单给吴永强", d3.Add(time.Hour)},
		{womodel.ActionFinish, p.repair, "完工：更换老化密封圈", d1.Add(-2 * time.Hour)},
		{womodel.ActionConfirmPass, p.fd, "验收通过", d1},
	}); err != nil {
		return err
	}

	// WO2 处理中：巡检异常转单（关联异常打卡与点位）
	wo2 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(d1), CommunityID: cid, PointID: &garagePoint,
		CheckinID: &abnormalCheckinID,
		Title:     "地下车库 B 区照明灯损坏", Description: "巡检发现车库 B 区一盏照明灯不亮，影响车辆通行安全，需更换。",
		Source: womodel.SourceInspection, Category: "强电", ReporterID: p.xj01,
		AssigneeID: &p.repair, DispatcherID: &p.eng, Priority: "high", Status: womodel.OrderProcessing,
		DispatchAt: at(yesterdayAt(11, 20)), AcceptAt: at(yesterdayAt(11, 20)),
		Items: types.OrderItemArray{{Name: "门禁与监控运行正常", Remark: "B 区一盏照明灯损坏"}},
		CreatedAt: yesterdayAt(11, 0), UpdatedAt: yesterdayAt(11, 20),
	}
	if err := create(wo2, []logStep{
		{womodel.ActionCreate, p.xj01, "巡检异常转工单", yesterdayAt(11, 0)},
		{womodel.ActionDispatch, p.eng, "派单给吴永强", yesterdayAt(11, 20)},
	}); err != nil {
		return err
	}

	// WO3 待验收：楼管员主动上报，已完工待报单方验收
	wo3 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(d1), CommunityID: cid,
		Title: "1 栋大堂门禁刷卡无响应", Description: "1 栋大堂门禁读卡器刷卡无反应，多户业主反映，请尽快检修。",
		Source: womodel.SourceActive, Category: "弱电智能化", ReporterID: p.bm,
		AssigneeID: &p.repair, DispatcherID: &p.eng, Priority: "urgent", Status: womodel.OrderPendingConfirm,
		TriageBy: &p.service, TriageAt: at(yesterdayAt(14, 5)), TriageNote: "影响业主出行，加急",
		DispatchAt: at(yesterdayAt(14, 30)), AcceptAt: at(yesterdayAt(14, 30)),
		FinishNote: "读卡器排线松脱，重新插拔固定后恢复正常。", FinishAt: at(yesterdayAt(17, 40)),
		CreatedAt: yesterdayAt(14, 0), UpdatedAt: yesterdayAt(17, 40),
	}
	if err := create(wo3, []logStep{
		{womodel.ActionCreate, p.bm, "楼管员主动上报", yesterdayAt(14, 0)},
		{womodel.ActionTriagePass, p.service, "分诊通过：影响业主出行，加急", yesterdayAt(14, 5)},
		{womodel.ActionDispatch, p.eng, "派单给吴永强", yesterdayAt(14, 30)},
		{womodel.ActionFinish, p.repair, "完工：读卡器排线修复", yesterdayAt(17, 40)},
	}); err != nil {
		return err
	}

	// WO4 待派单：主动上报，分诊已通过
	wo4 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(daysAgo(0)), CommunityID: cid,
		Title: "2 栋电梯运行异响", Description: "2 栋西梯上行至 7 层附近有明显异响，建议维保单位检查曳引系统。",
		Source: womodel.SourceActive, Category: "电梯", ReporterID: p.bm,
		Priority: "high", Status: womodel.OrderPendingDispatch,
		TriageBy: &p.service, TriageAt: at(todayAt(9, 10)), TriageNote: "属实，转工程安排电梯维保",
		CreatedAt: todayAt(8, 50), UpdatedAt: todayAt(9, 10),
	}
	if err := create(wo4, []logStep{
		{womodel.ActionCreate, p.bm, "楼管员主动上报", todayAt(8, 50)},
		{womodel.ActionTriagePass, p.service, "分诊通过：属实，转工程安排电梯维保", todayAt(9, 10)},
	}); err != nil {
		return err
	}

	// WO5 待分诊：前台代录刚进池
	wo5 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(daysAgo(0)), CommunityID: cid,
		Title: "3 栋楼道堆放杂物", Description: "业主来电反映 3 栋 12 层楼道堆放纸箱杂物，存在消防隐患，请安排清理。",
		Source: womodel.SourceFrontdesk, ReporterID: p.fd,
		Priority: "normal", Status: womodel.OrderReported,
		CreatedAt: todayAt(9, 40), UpdatedAt: todayAt(9, 40),
	}
	if err := create(wo5, []logStep{
		{womodel.ActionCreate, p.fd, "前台代录工单", todayAt(9, 40)},
	}); err != nil {
		return err
	}

	// WO6 已作废：重复报单分诊驳回
	wo6 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(d3), CommunityID: cid,
		Title: "北门路灯不亮（重复报单）", Description: "业主反映园区北门路灯不亮。",
		Source: womodel.SourceFrontdesk, ReporterID: p.fd,
		Priority: "normal", Status: womodel.OrderClosedInvalid,
		TriageBy: &p.service, TriageAt: at(d3.Add(time.Hour)), TriageNote: "与 WX 工单重复，驳回",
		RejectReason: "与既有工单重复",
		CreatedAt:    d3, UpdatedAt: d3.Add(time.Hour),
	}
	return create(wo6, []logStep{
		{womodel.ActionCreate, p.fd, "前台代录工单", d3},
		{womodel.ActionTriageReject, p.service, "分诊驳回：与既有工单重复", d3.Add(time.Hour)},
	})
}

// demoTemplateItemsOf 演示点位对应的模板项快照源（安全类点位 → 安全模板，设备类 → 设备模板）。
func demoTemplateItemsOf(pointType, tplSafetyID, tplEquipID string) (struct {
	items          []demoTemplateItem
	requiredPhotos []string
}, bool,
) {
	safety := []demoTemplateItem{
		{"消防通道畅通无阻", "通道无杂物堆放，安全出口标识清晰、应急照明正常", types.PhotoReqRequired},
		{"灭火器压力正常", "指针在绿色区域，铅封完好，在有效期内", types.PhotoReqRequired},
		{"无违规用电", "无私拉电线、无大功率违规电器", types.PhotoReqOptional},
		{"门禁与监控运行正常", "门禁刷卡正常，监控画面清晰无遮挡", types.PhotoReqNone},
	}
	equip := []demoTemplateItem{
		{"设备运行无异响", "运行声音平稳，无异常振动", types.PhotoReqRequired},
		{"仪表读数在正常范围", "压力表/温度表/电压表读数在额定区间内", types.PhotoReqRequired},
		{"无跑冒滴漏", "管路、阀门、泵体无渗漏", types.PhotoReqOptional},
	}
	out := struct {
		items          []demoTemplateItem
		requiredPhotos []string
	}{}
	switch pointType {
	case "power_room", "pump_room", "elevator":
		if tplEquipID == "" {
			return out, false
		}
		out.items = equip
		out.requiredPhotos = []string{"设备运行无异响", "仪表读数在正常范围"}
	default:
		if tplSafetyID == "" {
			return out, false
		}
		out.items = safety
		out.requiredPhotos = []string{"消防通道畅通无阻", "灭火器压力正常"}
	}
	return out, true
}

// ---------- 租户 B：金源物业（验证隔离 + 抢单模式） ----------

func (d *demoSeeder) seedTenantB() error {
	tenant := sysmodel.Tenant{
		Code: "jinyuan", Name: "金源物业服务集团",
		ContactName: "林秀英", ContactPhone: "027-59386666",
		Status: sysmodel.StatusEnabled, Remark: "演示租户（seed-demo 生成）",
	}
	if err := d.db.Create(&tenant).Error; err != nil {
		return err
	}
	tid := tenant.ID
	if err := systemsvc.CopyPostTemplatesToTenant(d.db, tid); err != nil {
		return err
	}

	adminID, err := d.createUser(tid, "jinyuan_admin", "林秀英", "13900002001", d.roleIDs[sysmodel.TenantAdminCode])
	if err != nil {
		return err
	}
	managerID, err := d.createUser(tid, "jy_manager", "黄志强", "13900002002")
	if err != nil {
		return err
	}
	xjID, err := d.createUser(tid, "jy_xj01", "郑凯", "13900002003")
	if err != nil {
		return err
	}
	repairID, err := d.createUser(tid, "jy_repair", "冯建华", "13900002004")
	if err != nil {
		return err
	}
	serviceID, err := d.createUser(tid, "jy_service", "何敏", "13900002005")
	if err != nil {
		return err
	}

	// 抢单模式开启（与租户 A 的分诊+派单模式形成对照）
	community := sysmodel.Community{
		TenantID: tid, Name: "金源世纪城", Address: "武汉市江夏区藏龙大道 12 号",
		ManagerID: &managerID, WoTriageEnabled: true, WoGrabEnabled: true,
		Status: sysmodel.StatusEnabled, Remark: "演示小区（seed-demo 生成）",
	}
	if err := d.db.Create(&community).Error; err != nil {
		return err
	}
	cid := community.ID

	for _, s := range []struct {
		uid   string
		posts []string
	}{
		{managerID, []string{sysmodel.PostProjectManager}},
		{xjID, []string{sysmodel.PostInspector}},
		{repairID, []string{sysmodel.PostRepairman}},
		{serviceID, []string{"service_supervisor"}},
	} {
		if err := d.addStaff(tid, cid, s.uid, s.posts); err != nil {
			return err
		}
	}

	tplID, required, err := d.createTemplate(tid, "安全巡查通用模板", []demoTemplateItem{
		{"消防通道畅通无阻", "通道无杂物堆放，安全出口标识清晰", types.PhotoReqRequired},
		{"灭火器压力正常", "指针在绿色区域，在有效期内", types.PhotoReqRequired},
		{"无违规充电", "无电动车违规入户/飞线充电", types.PhotoReqOptional},
	})
	if err != nil {
		return err
	}

	pointDefs := []demoPoint{
		{"消防控制室", "fire_control", insmodel.CredentialQRCode, -1, 114.41020, 30.46110, 80},
		{"配电房", "power_room", insmodel.CredentialQRCode, -1, 114.41035, 30.46125, 80},
		{"南门岗亭", "common", insmodel.CredentialAny, -1, 114.41050, 30.46090, 150},
		{"中心花园", "common", insmodel.CredentialNone, -1, 114.41010, 30.46150, 200},
	}
	var pointIDs []string
	pointMeta := map[string]demoPoint{}
	for i, pd := range pointDefs {
		id, err := d.createPoint(tid, cid, nil, pd, tplID, required, i+1)
		if err != nil {
			return err
		}
		pointIDs = append(pointIDs, id)
		pointMeta[id] = pd
	}

	plan := insmodel.InspectionPlan{
		TenantID: &tid, CommunityID: cid, Name: "每日安全巡查", PatrolType: insmodel.PatrolSafety,
		PointIDs: types.IDArray(pointIDs), CycleType: "daily", CycleConfig: types.JSONMap{"interval": 1},
		InspectorIDs: types.IDArray{xjID},
		StartDate:    daysAgo(7), TimeWindow: "08:00-18:00",
		Status: sysmodel.StatusEnabled, Remark: "演示计划（seed-demo 生成）",
	}
	if err := d.db.Create(&plan).Error; err != nil {
		return err
	}

	taskDone := insmodel.InspectionTask{
		TenantID: &tid, PlanID: plan.ID, CommunityID: cid, InspectorID: xjID,
		PatrolType: insmodel.PatrolSafety, TaskDate: daysAgo(1), Status: insmodel.TaskDone,
		TotalPoints: len(pointIDs), DonePoints: len(pointIDs),
		StartedAt: at(yesterdayAt(8, 40)), FinishedAt: at(yesterdayAt(9, 35)),
	}
	if err := d.db.Create(&taskDone).Error; err != nil {
		return err
	}
	taskToday := insmodel.InspectionTask{
		TenantID: &tid, PlanID: plan.ID, CommunityID: cid, InspectorID: xjID,
		PatrolType: insmodel.PatrolSafety, TaskDate: daysAgo(0), Status: insmodel.TaskPending,
		TotalPoints: len(pointIDs),
	}
	if err := d.db.Create(&taskToday).Error; err != nil {
		return err
	}

	for i, pid := range pointIDs {
		p := pointMeta[pid]
		checkinTime := yesterdayAt(8, 45+i*12)
		rec := insmodel.CheckinRecord{
			TenantID: &tid, TaskID: taskDone.ID, PointID: pid, InspectorID: xjID, CommunityID: cid,
			CheckinTime: checkinTime, ClientTime: &checkinTime,
			Longitude: floatptr(p.lng + 0.00002), Latitude: floatptr(p.lat + 0.00002),
			DistanceToPoint: floatptr(2.8), CheckinType: insmodel.CredentialQRCode,
			Result: insmodel.ResultNormal, AuditStatus: insmodel.AuditAutoPass,
			CreatedAt: checkinTime,
		}
		if p.credential == insmodel.CredentialNone {
			rec.CheckinType = "fence"
		}
		for _, label := range required {
			ph, _ := d.photo(tid, xjID, label)
			if ph.URL != "" {
				rec.Photos = append(rec.Photos, ph)
			}
		}
		if err := d.db.Create(&rec).Error; err != nil {
			return err
		}
	}

	// 两条工单：待派单池（抢单模式演示）+ 处理中
	d0 := daysAgo(0)
	wo1 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(d0), CommunityID: cid,
		Title: "南门道闸杆起降卡顿", Description: "早高峰南门道闸杆起降明显卡顿，影响车辆通行。",
		Source: womodel.SourceActive, Category: "弱电智能化", ReporterID: xjID,
		Priority: "high", Status: womodel.OrderPendingDispatch,
		TriageBy: &serviceID, TriageAt: at(todayAt(8, 20)), TriageNote: "属实，转工程",
		CreatedAt: todayAt(8, 5), UpdatedAt: todayAt(8, 20),
	}
	if err := d.db.Create(&wo1).Error; err != nil {
		return err
	}
	for _, l := range []struct {
		action, op, detail string
		at                 time.Time
	}{
		{womodel.ActionCreate, xjID, "巡检员主动上报", todayAt(8, 5)},
		{womodel.ActionTriagePass, serviceID, "分诊通过：属实，转工程", todayAt(8, 20)},
	} {
		row := womodel.WorkOrderLog{OrderID: wo1.ID, Action: l.action, OperatorID: l.op, Detail: l.detail, CreatedAt: l.at}
		if err := d.db.Create(&row).Error; err != nil {
			return err
		}
	}
	wo2 := womodel.WorkOrder{
		TenantID: &tid, OrderNo: d.orderNo(daysAgo(1)), CommunityID: cid,
		Title: "中心花园休闲椅螺丝松动", Description: "中心花园两张休闲椅螺丝松动，存在安全隐患。",
		Source: womodel.SourceActive, Category: "公共设施", ReporterID: xjID,
		AssigneeID: &repairID, Priority: "low", Status: womodel.OrderProcessing,
		DispatchAt: at(yesterdayAt(10, 10)), AcceptAt: at(yesterdayAt(10, 10)),
		CreatedAt: yesterdayAt(9, 50), UpdatedAt: yesterdayAt(10, 10),
	}
	if err := d.db.Create(&wo2).Error; err != nil {
		return err
	}
	for _, l := range []struct {
		action, op, detail string
		at                 time.Time
	}{
		{womodel.ActionCreate, xjID, "巡检员主动上报", yesterdayAt(9, 50)},
		{womodel.ActionGrab, repairID, "抢单成功", yesterdayAt(10, 10)},
	} {
		row := womodel.WorkOrderLog{OrderID: wo2.ID, Action: l.action, OperatorID: l.op, Detail: l.detail, CreatedAt: l.at}
		if err := d.db.Create(&row).Error; err != nil {
			return err
		}
	}

	// 月度报告：上月待巡检员确认（与租户 A 的已归档形成对照）
	if err := d.seedReportsB(tid, cid, managerID, xjID); err != nil {
		return err
	}

	notice := sysmodel.SysNotice{
		TenantID: &tid, Title: "中秋节期间值班安排通知",
		Content:    "中秋节期间项目服务中心实行值班制：前台每日 8:00–18:00 有人值守，工程与秩序条线按排班表执行巡查，遇紧急情况请第一时间上报项目经理。",
		Status:     1, PublishAt: at(daysAgo(1)), CreatedBy: &adminID, CreatedByName: "林秀英",
	}
	return d.db.Create(&notice).Error
}

// ---------- 时间/编号小工具 ----------

func daysAgo(n int) time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()-n, 0, 0, 0, 0, time.Local)
}

func yesterdayAt(hour, min int) time.Time {
	d := daysAgo(1)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, time.Local)
}

func todayAt(hour, min int) time.Time {
	d := daysAgo(0)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, time.Local)
}

func at(t time.Time) *time.Time { return &t }

func floatptr(f float64) *float64 { return &f }

// orderNo 演示工单号：WX+yyyyMMdd+3 位日内序号（与 OrderService.GenOrderNo 同格式；全局共享计数器保证不撞 uk_order_no）。
func (d *demoSeeder) orderNo(day time.Time) string {
	key := day.Format("20060102")
	d.orderSeq[key]++
	return fmt.Sprintf("WX%s%03d", key, d.orderSeq[key])
}

// ---------- 月度报告演示 ----------

// seedReportsA 租户 A 上月报告：三级签字全部完成、已归档（approved）。
func (d *demoSeeder) seedReportsA(tid, cid, managerID, safetyID, xj01ID, xj02ID string) error {
	period := lastMonthPeriod()
	svAt := monthDayAt(1, 14, 0)
	mgAt := monthDayAt(2, 10, 0)
	// 照片快照（photo 走缓存，保证 stats.records 附图 key 可访问）
	_, key1 := d.photo(tid, xj01ID, "消防通道畅通无阻")
	_, key2 := d.photo(tid, xj01ID, "灭火器压力正常")
	var photoKeys []string
	if key1 != "" && key2 != "" {
		photoKeys = []string{key1, key2}
	}
	lm, _ := time.ParseInLocation("2006-01", period, time.Local)
	rec := func(day, hour int, name, point, result string) map[string]any {
		return map[string]any{
			"checkin_time":   timefmt.T(time.Date(lm.Year(), lm.Month(), day, hour, 5, 0, 0, time.Local)),
			"inspector_name": name, "point_name": point, "checkin_type": "qrcode",
			"distance": 3.2, "result": result, "is_suspect": false,
			"audit_status": insmodel.AuditAutoPass, "photo_keys": photoKeys,
		}
	}
	records := []any{
		rec(3, 9, "陈刚", "消防控制室", insmodel.ResultNormal),
		rec(11, 10, "赵敏", "东门岗亭", insmodel.ResultNormal),
		rec(18, 9, "陈刚", "地下车库出入口", insmodel.ResultAbnormal),
	}
	report := rptmodel.InspectionReport{
		TenantID: &tid, CommunityID: cid, Period: period,
		Title:  demoReportTitle("锦绣华庭", period),
		Status: rptmodel.StatusApproved,
		Stats:  demoReportStats(period, 62, 59, 3, 434, 416, 4, 1, 6, 5, records),
		InspectorIDs: types.IDArray{xj01ID, xj02ID},
		InspectorSigned: types.SignArray{
			{UserID: xj01ID, Name: "陈刚", SignedAt: timefmt.T(monthDayAt(1, 9, 10))},
			{UserID: xj02ID, Name: "赵敏", SignedAt: timefmt.T(monthDayAt(1, 9, 25))},
		},
		SupervisorIDs: types.IDArray{safetyID},
		SupervisorBy:  &safetyID, SupervisorAt: &svAt, SupervisorRemark: "数据属实，同意",
		ManagerIDs:    types.IDArray{managerID},
		ManagerBy:     &managerID, ManagerAt: &mgAt, ManagerRemark: "审核通过，归档",
	}
	return d.db.Create(&report).Error
}

// seedReportsB 租户 B 上月报告：待巡检员确认（pending_inspector，签字流程第一级）。
func (d *demoSeeder) seedReportsB(tid, cid, managerID, xjID string) error {
	period := lastMonthPeriod()
	report := rptmodel.InspectionReport{
		TenantID: &tid, CommunityID: cid, Period: period,
		Title:  demoReportTitle("金源世纪城", period),
		Status: rptmodel.StatusPendingInspector,
		Stats:  demoReportStats(period, 31, 29, 2, 124, 116, 1, 0, 4, 3, []any{}),
		InspectorIDs:    types.IDArray{xjID},
		InspectorSigned: types.SignArray{},
		// 项目无安全主管编制 → 主管级名单为空（该级自动跳过），经理级为项目经理
		SupervisorIDs: types.IDArray{},
		ManagerIDs:    types.IDArray{managerID},
	}
	return d.db.Create(&report).Error
}

// lastMonthPeriod 上个月期间（YYYY-MM）。
func lastMonthPeriod() string {
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	return first.AddDate(0, -1, 0).Format("2006-01")
}

// monthDayAt 当前月某日时刻（报告签字时间用，月报次月初签署）。
func monthDayAt(day, hour, min int) time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), day, hour, min, 0, 0, time.Local)
}

// pct1 百分比（1 位小数，与 report_service.pct 同规则）。
func pct1(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(int(float64(a)/float64(b)*1000+0.5)) / 10
}

// demoReportTitle 报告标题（与 report_service.reportTitle 同格式）。
func demoReportTitle(commName, period string) string {
	t, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return commName + period + "月度巡检工作报告"
	}
	return fmt.Sprintf("%s%d年%d月月度巡检工作报告", commName, t.Year(), int(t.Month()))
}

// demoReportStats 与 ReportService.buildStats 同结构的统计快照（daily 逐日平铺任务/异常）。
func demoReportStats(period string, taskTotal, taskDone, taskOverdue, shouldPts, donePts, abnormal, suspect, woCreated, woClosed int64, records []any) types.JSONMap {
	t, _ := time.ParseInLocation("2006-01", period, time.Local)
	days := int64(time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.Local).Day())
	perDay, doneDay := taskTotal/days, taskDone/days
	daily := make([]any, 0, days)
	abLeft := abnormal
	for day := int64(1); day <= days; day++ {
		var ab int64
		if abLeft > 0 && day%7 == 3 {
			ab = 1
			abLeft--
		}
		daily = append(daily, map[string]any{
			"date": fmt.Sprintf("%s-%02d", period, day),
			"task_total": perDay, "task_done": doneDay, "abnormal": ab,
		})
	}
	return types.JSONMap{
		"task_total": taskTotal, "task_done": taskDone, "task_overdue": taskOverdue,
		"should_points": shouldPts, "done_points": donePts,
		"coverage_rate":  pct1(donePts, shouldPts),
		"abnormal_count": abnormal, "suspect_count": suspect,
		"wo_created": woCreated, "wo_closed": woClosed, "wo_unclosed": woCreated - woClosed,
		"wo_close_rate": pct1(woClosed, woCreated),
		"daily": daily, "records": records,
	}
}
