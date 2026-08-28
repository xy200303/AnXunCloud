// Package demo 演示数据播种（独立命令 cmd/seed-demo 调用）：与 server 主流程和系统预置 seed 完全解耦，
// 仅显式运行 seed-demo 命令时触发，不随服务启动执行。
// 内容：两家演示物业公司（租户）+ 小区/楼栋/编制/模板/点位/计划/任务/打卡/公告。
// 租户 A（华安物业/锦绣华庭）按甲方真实月度计划组织：小区分 A/B 区，消防设施（消火栓及灭火器）月检，
// 黄辉负责 B 区、杨诗负责 A 区+B 区 16/17 栋，约 3500 点位，月度计划按 LPT 贪心分摊到每日，月底前巡完。
// 幂等：演示租户已存在则整体跳过。
package demo

import (
	"bytes"
	"embed"
	"fmt"
	"hash/fnv"
	"time"

	"gorm.io/gorm"

	insmodel "anxuncloud/internal/module/inspection/model"
	inssvc "anxuncloud/internal/module/inspection/service"
	rptmodel "anxuncloud/internal/module/report/model"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/password"
	"anxuncloud/internal/pkg/storage"
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
	"消防通道畅通无阻": "corridor.jpg",
	"灭火器压力正常":  "fireext.jpg",
	"设备运行无异响":  "pump.jpg",
	"仪表读数在正常范围": "meter.jpg",
	// 「消火栓及灭火器检查」模板项（租户 A 消防月检）
	"灭火器在位且在有效期内": "fireext.jpg",
	"消火栓箱完好无损坏":  "fireext.jpg",
	"手动报警按钮外观完好":  "corridor.jpg",
}

// DemoPassword 全部演示账号的统一密码（满足 8–32 位含字母数字策略）。
const DemoPassword = "Demo@12345"

// demoTenantACode 演示租户 A 的公司代码（幂等标记：存在即跳过）。
const demoTenantACode = "huaan"

type demoSeeder struct {
	db         *gorm.DB
	store      *storage.Storage
	photoCache map[string]string
	roleIDs    map[string]string
	pwdHash    string
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
	d := &demoSeeder{store: store, photoCache: map[string]string{}, roleIDs: map[string]string{}}
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

// photo 生成/复用演示打卡照片（真实图片内置于 demoassets，仅本地存储驱动落盘；返回 file_key，失败返回空串）。
func (d *demoSeeder) photo(tenantID, ownerID, label string) string {
	if d.store == nil || !d.store.IsLocal() {
		return ""
	}
	if key, ok := d.photoCache[label]; ok {
		return key
	}
	data, err := demoAssetByLabel_(label)
	if err != nil {
		return ""
	}
	key, url, data, md5, err := d.store.Save("checkin", ownerID, "jpg", bytes.NewReader(data))
	if err != nil {
		return ""
	}
	rec := sysmodel.UploadFile{
		TenantID: &tenantID, FileKey: key, Scene: "checkin", UserID: ownerID,
		MimeType: "image/jpeg", URL: url, WatermarkedURL: url,
		Name: label + ".jpg", MD5: md5, Storage: d.store.DriverName(),
	}
	if err := d.db.Create(&rec).Error; err != nil {
		return ""
	}
	d.photoCache[label] = key
	return key
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
	name, requirement, photoReq, aiHint string
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
		Sort:       sort, Status: sysmodel.StatusEnabled, Remark: "演示点位（seed-demo 生成）",
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

// createPoints 批量创建点位（一次取整段二维码序列号 + CreateInBatches；几千点位逐行插太慢）。
// 点位的 QRCodeNo 由本函数分配，调用方不传。
func (d *demoSeeder) createPoints(points []insmodel.InspectionPoint) error {
	if len(points) == 0 {
		return nil
	}
	var seqs []int64
	if err := d.db.Raw("SELECT nextval('qrcode_no_seq') FROM generate_series(1, ?)", len(points)).Scan(&seqs).Error; err != nil {
		return err
	}
	if len(seqs) != len(points) {
		return fmt.Errorf("二维码序列号数量不符: %d/%d", len(seqs), len(points))
	}
	for i := range points {
		points[i].QRCodeNo = fmt.Sprintf("P%06d", seqs[i])
	}
	return d.db.CreateInBatches(&points, 500).Error
}

// ---------- 租户 A：华安物业（完整演示：A/B 区消防设施月检） ----------

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
	// 打卡审批流程租户级覆盖：主管审核 → 项目经理复核（演示两级审批流，扩展方案 §3）
	flow := sysmodel.ApprovalFlow{
		TenantID: &tid, FlowCode: sysmodel.FlowCheckinReview,
		Steps: types.FlowStepArray{
			{Slot: sysmodel.SlotPatrolReportLine, Name: "主管审核"},
			{Slot: sysmodel.SlotProjectReview, Name: "项目经理复核"},
		},
	}
	if err := d.db.Create(&flow).Error; err != nil {
		return err
	}

	// 账号（统一密码 Demo@12345；除租户管理员外不显式挂角色，演示岗位绑定角色的实时并集）
	// 巡检员按甲方真实月度计划表：黄辉负责 B 区，杨诗负责 A 区+B 区 16/17 栋
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
	huangID, err := d.createUser(tid, "xj_huang", "黄辉", "13800001004")
	if err != nil {
		return err
	}
	yangID, err := d.createUser(tid, "xj_yang", "杨诗", "13800001005")
	if err != nil {
		return err
	}

	// 小区 + 楼栋/区域（A 区 1~12 栋、B 区 1~17 栋挂楼栋行；非楼栋点位挂 A 区/B 区区域行）
	community := sysmodel.Community{
		TenantID: tid, Name: "锦绣华庭", Address: "武汉市洪山区光谷大道 88 号",
		ManagerID: &managerID,
		Status: sysmodel.StatusEnabled, Remark: "演示小区（seed-demo 生成）",
	}
	if err := d.db.Create(&community).Error; err != nil {
		return err
	}
	cid := community.ID
	bldIDs := map[string]string{}  // 楼栋名 → ID（如 "B区1栋"）
	areaIDs := map[string]string{} // 区域名 → ID（如 "B区"）
	sort := 1
	addBuilding := func(name, typ string) (string, error) {
		b := insmodel.Building{
			TenantID: &tid, CommunityID: cid, Name: name, Type: typ,
			Sort: sort, Status: sysmodel.StatusEnabled,
		}
		sort++
		if err := d.db.Create(&b).Error; err != nil {
			return "", err
		}
		return b.ID, nil
	}
	for _, area := range []string{"A区", "B区"} {
		id, err := addBuilding(area, "area")
		if err != nil {
			return err
		}
		areaIDs[area] = id
	}
	for i := 1; i <= 12; i++ {
		name := fmt.Sprintf("A区%d栋", i)
		id, err := addBuilding(name, "building")
		if err != nil {
			return err
		}
		bldIDs[name] = id
	}
	for i := 1; i <= 17; i++ {
		name := fmt.Sprintf("B区%d栋", i)
		id, err := addBuilding(name, "building")
		if err != nil {
			return err
		}
		bldIDs[name] = id
	}

	// 编制（巡检体系相关岗位：项目经理 / 安全主管 / 巡检员）
	staff := []struct {
		uid       string
		posts     []string
		buildings []string
	}{
		{managerID, []string{sysmodel.PostProjectManager}, nil},
		{safetyID, []string{"safety_supervisor"}, nil},
		{huangID, []string{sysmodel.PostInspector}, nil},
		{yangID, []string{sysmodel.PostInspector}, nil},
	}
	for _, s := range staff {
		if err := d.addStaff(tid, cid, s.uid, s.posts, s.buildings...); err != nil {
			return err
		}
	}

	// 检查项模板：全部点位统一绑「消火栓及灭火器检查」（必拍项带 AI 识别要点）
	tplFireID, _, err := d.createTemplate(tid, "消火栓及灭火器检查", fireCheckItems)
	if err != nil {
		return err
	}

	// 点位 + 月度计划 + 本月历史任务/打卡（黄辉 B 区、杨诗 A 区+B区16/17栋）
	if err := d.seedFireMonthly(tid, cid, tplFireID, bldIDs, areaIDs, huangID, yangID); err != nil {
		return err
	}

	// 公告
	notice := sysmodel.SysNotice{
		TenantID: &tid, Title: "关于开展月度消防设施检查的通知",
		Content: "各岗位请注意：本月消防设施（消火栓及灭火器）月检已按 A/B 区下达计划，黄辉负责 B 区、杨诗负责 A 区及 B 区 16/17 栋，请按每日任务逐项落实，发现异常立即拍照上报，月底前完成全部点位巡查。",
		Status:  1, PublishAt: at(daysAgo(0)), CreatedBy: &adminID, CreatedByName: "王建国",
	}
	return d.db.Create(&notice).Error
}

// fireCheckItems 「消火栓及灭火器检查」模板项（参考真实消防检查；必拍项带 ai_hint，judge_type 走默认 general）。
var fireCheckItems = []demoTemplateItem{
	{"灭火器在位且在有效期内", "灭火器在位、压力表指针在绿区、铅封完好、在有效期内", types.PhotoReqRequired, "灭火器在位、压力表指针在绿区、铅封完好、在有效期内"},
	{"消火栓箱完好无损坏", "箱门完好、水带水枪齐全、无锈蚀破损", types.PhotoReqRequired, "箱门完好、水带水枪齐全、无锈蚀破损"},
	{"消防通道畅通无阻", "通道无杂物堆放、安全出口标识清晰", types.PhotoReqRequired, "通道无杂物堆放、安全出口标识清晰"},
	{"手动报警按钮外观完好", "按钮外观完好、标识清晰", types.PhotoReqOptional, "按钮外观完好、标识清晰"},
}

// fireUnit 楼栋单元定位（区域 + 楼栋号 + 单元号）。
type fireUnit struct {
	area     string
	bld, unit int
}

// firePlanChunk 月度计划分块：一块对应一个 monthly 计划（days 仅 1 天）。
// start/end 为点位切片下标区间 [start, end)。
type firePlanChunk struct {
	name       string
	start, end int
}

func (c firePlanChunk) size() int { return c.end - c.start }

// seedFireMonthly 租户 A 巡检数据：A/B 区消防点位（约 3500 个，全部绑「消火栓及灭火器检查」模板）、
// 按类别分块的月度计划（LPT 分摊到 1~28 日）、本月已过期日的已完成任务与全量打卡。
func (d *demoSeeder) seedFireMonthly(tid, cid, tplID string, bldIDs, areaIDs map[string]string, huangID, yangID string) error {
	var points []insmodel.InspectionPoint
	var chunksHuang, chunksYang []firePlanChunk

	newPoint := func(name, typ, bldID string, unitNo, floor *int, lng, lat float64, fence int) {
		pt := insmodel.InspectionPoint{
			TenantID: &tid, CommunityID: cid, Name: name, Type: typ,
			Longitude: lng, Latitude: lat, FenceRadius: fence,
			Credential: insmodel.CredentialQRCode, RequireFence: true,
			TemplateID: &tplID, Sort: len(points) + 1,
			Status: sysmodel.StatusEnabled, Remark: "演示点位（seed-demo 生成）",
		}
		if bldID != "" {
			pt.BuildingID = &bldID
		}
		pt.UnitNo = unitNo
		pt.Floor = floor
		points = append(points, pt)
	}
	intp := func(v int) *int { return &v }

	// 楼栋坐标：以武汉光谷（114.3985,30.5052）为中心，A/B 区成片偏移；
	// 同栋同单元的垂直点位共坐标。
	bldCoord := func(area string, bld, unit int) (float64, float64) {
		if area == "A区" {
			return 114.3968 + float64((bld-1)%4)*0.0009 + float64(unit-1)*0.00012,
				30.5038 + float64((bld-1)/4)*0.0007
		}
		return 114.3988 + float64((bld-1)%5)*0.0009 + float64(unit-1)*0.00012,
			30.5056 + float64((bld-1)/5)*0.0007
	}

	// 楼栋点位：每单元 33 层各 1 个消防点（连续追加，类别范围由调用方记录）
	buildUnits := func(units []fireUnit) {
		for i := 0; i < len(units); i++ {
			end := min(i+1, len(units))
			for _, u := range units[i:end] {
				lng, lat := bldCoord(u.area, u.bld, u.unit)
				bldID := bldIDs[fmt.Sprintf("%s%d栋", u.area, u.bld)]
				for f := 1; f <= 33; f++ {
					newPoint(fmt.Sprintf("%s%d栋%d单元%d层消防点", u.area, u.bld, u.unit, f),
						"fire_cabinet", bldID, intp(u.unit), intp(f), lng, lat, 80)
				}
			}
		}
	}

	// 商铺点位（沿街网格坐标）
	buildShops := func(area string, count int, lng0, lat0 float64) {
		for i := 0; i < count; i += 50 {
			end := min(i+50, count)
			for j := i; j < end; j++ {
				newPoint(fmt.Sprintf("%s商铺S%03d", area, j+101), "fire_extinguisher", areaIDs[area], nil, nil,
					lng0+float64(j%30)*0.00006, lat0+float64(j/30)*0.00008, 80)
			}
		}
	}

	// 地下车库点位（大片网格坐标，围栏放宽到 150）
	buildGarage := func(floorLabel, series string, floor, count int, lng0, lat0 float64, area string) {
		for i := 0; i < count; i += 50 {
			end := min(i+50, count)
			for j := i; j < end; j++ {
				newPoint(fmt.Sprintf("地下车库%s%s%03d消防点", floorLabel, series, j+1), "garage", areaIDs[area],
					nil, intp(floor), lng0+float64(j%40)*0.00007, lat0+float64(j/40)*0.00005, 150)
			}
		}
	}

	// ---- 黄辉（B 区）：楼栋 22 单元×33 层 = 726，商铺 300，车库负一层 800，门岗 6 + 车棚 12 + 办公 8 ----
	// 类别级分块：每类一个 split 计划，点位在切片内连续（切块时地理聚集）
	mkChunk := func(name string, start int, chunks *[]firePlanChunk) {
		*chunks = append(*chunks, firePlanChunk{name: name, start: start, end: len(points)})
	}
	catStart := len(points)
	buildUnits([]fireUnit{
		{"B区", 1, 1}, {"B区", 1, 2}, {"B区", 2, 1}, {"B区", 2, 2}, {"B区", 3, 1},
		{"B区", 4, 1}, {"B区", 5, 1}, {"B区", 5, 2}, {"B区", 6, 1}, {"B区", 6, 2},
		{"B区", 7, 1}, {"B区", 8, 1}, {"B区", 9, 1}, {"B区", 9, 2}, {"B区", 10, 1},
		{"B区", 10, 2}, {"B区", 11, 1}, {"B区", 12, 1}, {"B区", 13, 1}, {"B区", 14, 1},
		{"B区", 14, 2}, {"B区", 15, 1},
	})
	mkChunk("B区楼栋消防月检", catStart, &chunksHuang)
	catStart = len(points)
	buildShops("B区", 300, 114.3985, 30.5049)
	mkChunk("B区商铺消防月检", catStart, &chunksHuang)
	catStart = len(points)
	buildGarage("负一层", "A", -1, 800, 114.3986, 30.5042, "B区")
	mkChunk("B区地下车库消防月检（负一层）", catStart, &chunksHuang)
	{
		start := len(points)
		for i := 0; i < 6; i++ {
			newPoint(fmt.Sprintf("B区%d号门岗", i+1), "common", areaIDs["B区"], nil, nil,
				114.3982+float64(i%3)*0.0003, 30.5054+float64(i/3)*0.0002, 100)
		}
		for i := 0; i < 12; i++ {
			newPoint(fmt.Sprintf("B区电动车棚%d", i+1), "common", areaIDs["B区"], nil, nil,
				114.3993+float64(i%4)*0.00015, 30.5048+float64(i/4)*0.00012, 100)
		}
		for i, name := range []string{"物业办公室", "多功能厅", "会议室", "客服中心", "档案室", "员工休息室", "监控室", "物资仓库"} {
			newPoint(name+"消防点", "common", areaIDs["B区"], nil, nil,
				114.3986+float64(i%4)*0.0001, 30.5051+float64(i/4)*0.0001, 80)
		}
		mkChunk("B区门岗车棚办公消防月检", start, &chunksHuang)
	}

	// ---- 杨诗（A 区 + B 区 16/17 栋）：楼栋 19 单元×33 层 = 627，商铺 280，车库负二层 720，门岗 5 + 车棚 10 ----
	catStart = len(points)
	buildUnits([]fireUnit{
		{"A区", 1, 1}, {"A区", 2, 1}, {"A区", 2, 2}, {"A区", 3, 1}, {"A区", 3, 2},
		{"A区", 4, 1}, {"A区", 5, 1}, {"A区", 6, 1}, {"A区", 7, 1}, {"A区", 7, 2},
		{"A区", 8, 1}, {"A区", 9, 1}, {"A区", 10, 1}, {"A区", 11, 1}, {"A区", 12, 1},
	})
	buildUnits([]fireUnit{
		{"B区", 16, 1}, {"B区", 16, 2}, {"B区", 17, 1}, {"B区", 17, 2},
	})
	mkChunk("A区楼栋消防月检（含B区16/17栋）", catStart, &chunksYang)
	catStart = len(points)
	buildShops("A区", 280, 114.3970, 30.5045)
	mkChunk("A区商铺消防月检", catStart, &chunksYang)
	catStart = len(points)
	buildGarage("负二层", "B", -2, 720, 114.3969, 30.5033, "A区")
	mkChunk("A区地下车库消防月检（负二层）", catStart, &chunksYang)
	{
		start := len(points)
		for i := 0; i < 5; i++ {
			newPoint(fmt.Sprintf("A区%d号门岗", i+1), "common", areaIDs["A区"], nil, nil,
				114.3967+float64(i%3)*0.0003, 30.5046+float64(i/3)*0.0002, 100)
		}
		for i := 0; i < 10; i++ {
			newPoint(fmt.Sprintf("A区电动车棚%d", i+1), "common", areaIDs["A区"], nil, nil,
				114.3978+float64(i%4)*0.00015, 30.5041+float64(i/4)*0.00012, 100)
		}
		mkChunk("A区门岗车棚消防月检", start, &chunksYang)
	}

	if err := d.createPoints(points); err != nil {
		return err
	}

	// 月度计划：每巡检员按类别 1 个计划（共 8 个），assign_mode=split 全月 1~28 日执行，
	// 点位由系统在任务生成时按「执行日 × 巡检员」连续切块（路线优化排序后地理聚集，每人每日跑一片区域）
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	allDays := make([]int, 28)
	for i := range allDays {
		allDays[i] = i + 1
	}
	ptByID := make(map[string]*insmodel.InspectionPoint, len(points))
	for i := range points {
		ptByID[points[i].ID] = &points[i]
	}
	type planCtx struct {
		plan        *insmodel.InspectionPlan
		inspectorID string
		ordered     types.IDArray // 路线优化后的顺序（切块基准，与 GenerateForDate 同口径）
	}
	var ctxs []planCtx
	for _, spec := range []struct {
		chunks      []firePlanChunk
		inspectorID string
	}{{chunksHuang, huangID}, {chunksYang, yangID}} {
		for _, c := range spec.chunks {
			ids := make(types.IDArray, 0, c.size())
			for _, pt := range points[c.start:c.end] {
				ids = append(ids, pt.ID)
			}
			plan := &insmodel.InspectionPlan{
				TenantID: &tid, CommunityID: cid, Name: c.name, PatrolType: "fire",
				PointIDs: ids, CycleType: "monthly", CycleConfig: types.JSONMap{"days": allDays},
				AssignMode:   insmodel.AssignSplit,
				InspectorIDs: types.IDArray{spec.inspectorID},
				StartDate:    monthStart, TimeWindow: "08:00-18:00",
				Status: sysmodel.StatusEnabled, Remark: "演示计划（seed-demo 生成）",
			}
			if err := d.db.Create(plan).Error; err != nil {
				return err
			}
			ctxs = append(ctxs, planCtx{plan: plan, inspectorID: spec.inspectorID, ordered: inssvc.OrderPointsByRoute(d.db, ids)})
		}
	}

	// 本月历史：1 日至昨天每天每计划 1 条已完成任务（点位按 split 口径取当天切块）+ 全量打卡；
	// 今天及未来由调度器/手动生成
	yesterday := daysAgo(1)
	if yesterday.Month() != now.Month() {
		return nil // 每月 1 日播种：本月无历史日
	}
	ymax := yesterday.Day()

	var tasks []insmodel.InspectionTask
	var taskSlices []types.IDArray
	for _, pc := range ctxs {
		for day := 1; day <= ymax; day++ {
			date := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, time.Local)
			slice := insmodel.SplitPointsForDate(pc.plan, pc.ordered, date, pc.inspectorID)
			if len(slice) == 0 {
				continue
			}
			n := len(slice)
			// 打卡 08:30 起均匀递增，17:30 前巡完
			first := time.Date(now.Year(), now.Month(), day, 8, 30, 0, 0, time.Local)
			last := first.Add(time.Duration(32400/n*(n-1)) * time.Second)
			tasks = append(tasks, insmodel.InspectionTask{
				TenantID: &tid, PlanID: pc.plan.ID, CommunityID: cid, InspectorID: pc.inspectorID,
				PatrolType: "fire", TaskDate: date, Status: insmodel.TaskDone,
				PointIDs: slice, TotalPoints: n, DonePoints: n,
				StartedAt: at(first.Add(-2 * time.Minute)), FinishedAt: at(last.Add(3 * time.Minute)),
			})
			taskSlices = append(taskSlices, slice)
		}
	}
	if len(tasks) == 0 {
		return nil
	}
	if err := d.db.CreateInBatches(&tasks, 500).Error; err != nil {
		return err
	}

	// 打卡记录（约 2% 异常；created_at 与 checkin_time 同月，满足按月分区）
	type abSpec struct{ remark, item, note string }
	abSpecs := []abSpec{
		{"灭火器压力不足，已登记更换", "灭火器在位且在有效期内", "压力表指针在红区"},
		{"消火栓箱门损坏，已拍照报修", "消火栓箱完好无损坏", "箱门变形无法正常关闭"},
		{"消防通道堆放杂物，已现场清理并告知责任人", "消防通道畅通无阻", "通道有杂物堆放"},
		{"灭火器已过有效期，已登记更换", "灭火器在位且在有效期内", "铭牌显示已过有效期"},
	}
	var recs []insmodel.CheckinRecord
	cnt, abCnt := 0, 0
	for ti := range tasks {
		slice := taskSlices[ti]
		n := tasks[ti].TotalPoints
		first := tasks[ti].StartedAt.Add(2 * time.Minute) // 回到 08:30
		stepSec := 32400 / n
		for i := 0; i < n; i++ {
			pt := ptByID[slice[i]]
			ct := first.Add(time.Duration(stepSec*i) * time.Second)
			rec := insmodel.CheckinRecord{
				TenantID: &tid, TaskID: tasks[ti].ID, PointID: pt.ID, InspectorID: tasks[ti].InspectorID,
				CommunityID: cid, CheckinTime: ct, ClientTime: &ct,
				Longitude: floatptr(pt.Longitude + 0.00003), Latitude: floatptr(pt.Latitude + 0.00002),
				DistanceToPoint: floatptr(2.0 + float64(cnt%28)*0.1), CheckinType: insmodel.CredentialQRCode,
				Result: insmodel.ResultNormal, AuditStatus: insmodel.AuditAutoPass, CreatedAt: ct,
			}
			if cnt%50 == 9 {
				ab := abSpecs[abCnt%len(abSpecs)]
				rec.Result = insmodel.ResultAbnormal
				rec.Remark = ab.remark
				abCnt++
			}
			recs = append(recs, rec)
			cnt++
		}
	}
	if err := d.db.CreateInBatches(&recs, 500).Error; err != nil {
		return err
	}

	// 逐项结果快照（4 项；必拍项带共享照片 key；异常记录的对应必拍项 pass=false + note）
	var items []insmodel.CheckinRecordItem
	abCnt = 0
	for i := range recs {
		rec := &recs[i]
		var ab *abSpec
		if rec.Result == insmodel.ResultAbnormal {
			ab = &abSpecs[abCnt%len(abSpecs)]
			abCnt++
		}
		for j, it := range fireCheckItems {
			row := insmodel.CheckinRecordItem{
				RecordID: rec.ID, Name: it.name, Requirement: strptr(it.requirement),
				JudgeType: "general", PhotoRequired: it.photoReq,
				Pass: true, Sort: j + 1, CreatedAt: rec.CheckinTime,
			}
			if it.aiHint != "" {
				row.AIHint = strptr(it.aiHint)
			}
			if it.photoReq == types.PhotoReqRequired {
				if key := d.photo(tid, rec.InspectorID, it.name); key != "" {
					row.Photos = types.StringArray{key}
				}
			}
			if ab != nil && it.name == ab.item {
				row.Pass = false
				row.Note = ab.note
			}
			items = append(items, row)
		}
	}
	return d.db.CreateInBatches(&items, 500).Error
}

// ---------- 租户 B：金源物业（验证多租户隔离） ----------

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

	community := sysmodel.Community{
		TenantID: tid, Name: "金源世纪城", Address: "武汉市江夏区藏龙大道 12 号",
		ManagerID: &managerID,
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
	} {
		if err := d.addStaff(tid, cid, s.uid, s.posts); err != nil {
			return err
		}
	}

	tplID, required, err := d.createTemplate(tid, "安全巡查通用模板", []demoTemplateItem{
		{"消防通道畅通无阻", "通道无杂物堆放，安全出口标识清晰", types.PhotoReqRequired, ""},
		{"灭火器压力正常", "指针在绿色区域，在有效期内", types.PhotoReqRequired, ""},
		{"无违规充电", "无电动车违规入户/飞线充电", types.PhotoReqOptional, ""},
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
		if err := d.db.Create(&rec).Error; err != nil {
			return err
		}
	}

	// 上月（报告期）真实工作量 + 月度报告（待巡检员确认）
	if err := d.seedReportsB(tid, cid, plan.ID, pointIDs, pointMeta, tplID, managerID, xjID); err != nil {
		return err
	}

	notice := sysmodel.SysNotice{
		TenantID: &tid, Title: "中秋节期间值班安排通知",
		Content: "中秋节期间项目服务中心实行值班制：前台每日 8:00–18:00 有人值守，工程与秩序条线按排班表执行巡查，遇紧急情况请第一时间上报项目经理。",
		Status:  1, PublishAt: at(daysAgo(1)), CreatedBy: &adminID, CreatedByName: "林秀英",
	}
	return d.db.Create(&notice).Error
}

// demoTemplateItemsOf 演示点位对应的模板项快照源（租户 B：安全类点位 → 安全模板）。
func demoTemplateItemsOf(pointType, tplSafetyID, tplEquipID string) (struct {
	items          []demoTemplateItem
	requiredPhotos []string
}, bool,
) {
	safety := []demoTemplateItem{
		{"消防通道畅通无阻", "通道无杂物堆放，安全出口标识清晰、应急照明正常", types.PhotoReqRequired, ""},
		{"灭火器压力正常", "指针在绿色区域，铅封完好，在有效期内", types.PhotoReqRequired, ""},
		{"无违规用电", "无私拉电线、无大功率违规电器", types.PhotoReqOptional, ""},
		{"门禁与监控运行正常", "门禁刷卡正常，监控画面清晰无遮挡", types.PhotoReqNone, ""},
	}
	equip := []demoTemplateItem{
		{"设备运行无异响", "运行声音平稳，无异常振动", types.PhotoReqRequired, ""},
		{"仪表读数在正常范围", "压力表/温度表/电压表读数在额定区间内", types.PhotoReqRequired, ""},
		{"无跑冒滴漏", "管路、阀门、泵体无渗漏", types.PhotoReqOptional, ""},
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

// ---------- 时间/编号小工具 ----------

func daysAgo(n int) time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()-n, 0, 0, 0, 0, time.Local)
}

func yesterdayAt(hour, min int) time.Time {
	d := daysAgo(1)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, time.Local)
}

func at(t time.Time) *time.Time { return &t }

func floatptr(f float64) *float64 { return &f }

// ---------- 租户 B 月度报告演示（报告期真实工作量 → 报告，口径一致） ----------
//
// 背景：月报 PDF（pdfData）不按 stats 快照渲染，而是按报告期实时查询任务/打卡，
// 所以演示数据必须在报告期（上月）落真实的任务与打卡，月报才有明细与照片。

// monthWorkload 上月每日巡查工作量参数。
type monthWorkload struct {
	planID      string
	pointIDs    []string
	pointMeta   map[string]demoPoint
	tplSafetyID string
	tplEquipID  string
	inspectors  []string
	overdueDays map[int]bool // 逾期未巡的日期（仅作用于末位巡检员）
	abnormalDay map[int]int  // 日期 → pointIDs 下标（首位巡检员该点位打卡异常）
}

// monthStat 上月工作量统计（与落库数据一致，直接写报告 stats 快照）。
type monthStat struct {
	taskTotal, taskDone, taskOverdue int64
	shouldPts, donePts               int64
	abnormal                         int64
	daily                            []any
}

// seedMonthWorkload 落上月每日任务与打卡（完成/逾期/异常），返回与实际数据一致的统计。
func (d *demoSeeder) seedMonthWorkload(tid, cid string, o monthWorkload) (monthStat, error) {
	var st monthStat
	lm, _ := time.ParseInLocation("2006-01", lastMonthPeriod(), time.Local)
	days := time.Date(lm.Year(), lm.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	var recs []insmodel.CheckinRecord
	for day := 1; day <= days; day++ {
		date := time.Date(lm.Year(), lm.Month(), day, 0, 0, 0, 0, time.Local)
		dayTotal, dayDone, dayAb := int64(0), int64(0), int64(0)
		for ii, inspectorID := range o.inspectors {
			dayTotal++
			st.taskTotal++
			st.shouldPts += int64(len(o.pointIDs))
			task := insmodel.InspectionTask{
				TenantID: &tid, PlanID: o.planID, CommunityID: cid, InspectorID: inspectorID,
				PatrolType: insmodel.PatrolSafety, TaskDate: date,
				TotalPoints: len(o.pointIDs),
			}
			if o.overdueDays[day] && ii == len(o.inspectors)-1 {
				task.Status = insmodel.TaskOverdue
				st.taskOverdue++
				if err := d.db.Create(&task).Error; err != nil {
					return st, err
				}
				continue
			}
			startT := time.Date(lm.Year(), lm.Month(), day, 9, ii*30, 0, 0, time.Local)
			endT := startT.Add(90 * time.Minute)
			task.Status = insmodel.TaskDone
			task.DonePoints = len(o.pointIDs)
			task.StartedAt = &startT
			task.FinishedAt = &endT
			if err := d.db.Create(&task).Error; err != nil {
				return st, err
			}
			dayDone++
			st.taskDone++
			st.donePts += int64(len(o.pointIDs))
			abIdx, hasAb := o.abnormalDay[day]
			for pi, pid := range o.pointIDs {
				p := o.pointMeta[pid]
				ct := startT.Add(time.Duration(5+pi*12) * time.Minute)
				abnormal := hasAb && pi == abIdx && ii == 0 // 异常仅记首位巡检员，一天一条
				rec := insmodel.CheckinRecord{
					TenantID: &tid, TaskID: task.ID, PointID: pid, InspectorID: inspectorID, CommunityID: cid,
					CheckinTime: ct, ClientTime: &ct,
					Longitude: floatptr(p.lng + 0.00003), Latitude: floatptr(p.lat + 0.00002),
					DistanceToPoint: floatptr(3.4), CheckinType: insmodel.CredentialQRCode,
					Result: insmodel.ResultNormal, AuditStatus: insmodel.AuditAutoPass, CreatedAt: ct,
				}
				if p.credential == insmodel.CredentialNone {
					rec.CheckinType = "fence"
				}
				if abnormal {
					rec.Result = insmodel.ResultAbnormal
					rec.Remark = "现场发现异常，已拍照上报"
					st.abnormal++
					dayAb = 1
				}
				if _, ok := demoTemplateItemsOf(p.typ, o.tplSafetyID, o.tplEquipID); !ok {
					return st, fmt.Errorf("演示点位模板缺失: %s", p.name)
				}
				recs = append(recs, rec)
			}
		}
		st.daily = append(st.daily, map[string]any{
			"date": date.Format("2006-01-02"), "task_total": dayTotal, "task_done": dayDone, "abnormal": dayAb,
		})
	}
	if err := d.db.CreateInBatches(&recs, 200).Error; err != nil {
		return st, err
	}
	// 逐项结果快照（依赖批量插入后回填的 record ID）
	var items []insmodel.CheckinRecordItem
	for _, rec := range recs {
		p := o.pointMeta[rec.PointID]
		tpl, _ := demoTemplateItemsOf(p.typ, o.tplSafetyID, o.tplEquipID)
		for j, it := range tpl.items {
			row := insmodel.CheckinRecordItem{
				RecordID: rec.ID, Name: it.name, Requirement: strptr(it.requirement),
				PhotoRequired: it.photoReq, Pass: true, Sort: j + 1, CreatedAt: rec.CheckinTime,
			}
			if it.photoReq == types.PhotoReqRequired {
				key := d.photo(tid, rec.InspectorID, it.name)
				if key != "" {
					row.Photos = types.StringArray{key}
				}
			}
			if rec.Result == insmodel.ResultAbnormal && j == 0 {
				row.Pass = false
				row.Note = "现场发现异常"
			}
			items = append(items, row)
		}
	}
	if err := d.db.CreateInBatches(&items, 200).Error; err != nil {
		return st, err
	}
	return st, nil
}

// seedReportsB 租户 B 上月报告：报告期真实工作量；报告待巡检员确认。
func (d *demoSeeder) seedReportsB(tid, cid, planID string, pointIDs []string, pointMeta map[string]demoPoint,
	tplID, managerID, xjID string) error {
	period := lastMonthPeriod()

	// 1) 上月每日巡查：单巡检员，2 天逾期，1 条异常打卡
	st, err := d.seedMonthWorkload(tid, cid, monthWorkload{
		planID: planID, pointIDs: pointIDs, pointMeta: pointMeta,
		tplSafetyID: tplID, tplEquipID: tplID,
		inspectors:  []string{xjID},
		overdueDays: map[int]bool{12: true, 24: true},
		abnormalDay: map[int]int{15: 1}, // 配电房
	})
	if err != nil {
		return err
	}

	// 2) 报告：待巡检员确认（签字流程第一级；项目无安全主管，主管级自动跳过）
	report := rptmodel.InspectionReport{
		TenantID: &tid, CommunityID: cid, Period: period,
		Title:           demoReportTitle("金源世纪城", period),
		Status:          rptmodel.StatusPendingInspector,
		Stats:           demoReportStats(st, 0, []any{}),
		InspectorIDs:    types.IDArray{xjID},
		InspectorSigned: types.SignArray{},
		SupervisorIDs:   types.IDArray{},
		ManagerIDs:      types.IDArray{managerID},
	}
	return d.db.Create(&report).Error
}

// lastMonthPeriod 上个月期间（YYYY-MM）。
func lastMonthPeriod() string {
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	return first.AddDate(0, -1, 0).Format("2006-01")
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

// demoReportStats 与 ReportService.buildStats 同结构的统计快照（数字来自实际落库的上月工作量）。
// 演示看板指标占位。
func demoReportStats(st monthStat, suspect int64, records []any) types.JSONMap {
	return types.JSONMap{
		"task_total": st.taskTotal, "task_done": st.taskDone, "task_overdue": st.taskOverdue,
		"should_points": st.shouldPts, "done_points": st.donePts,
		"coverage_rate":  pct1(st.donePts, st.shouldPts),
		"abnormal_count": st.abnormal, "suspect_count": suspect,
		"wo_created": int64(0), "wo_closed": int64(0), "wo_unclosed": int64(0),
		"wo_close_rate": int64(0),
		"daily":         st.daily, "records": records,
	}
}
