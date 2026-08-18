// Package service 巡检模块业务逻辑：点位/计划/任务/打卡记录。
package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	systemsvc "anxuncloud/internal/module/system/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/excel"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/watermark"
)

// PointService 点位服务。
type PointService struct {
	db     *gorm.DB
	store  *storage.Storage
	getCfg func(key string) (string, bool)
}

func NewPointService(db *gorm.DB, store *storage.Storage, getCfg func(string) (string, bool)) *PointService {
	return &PointService{db: db, store: store, getCfg: getCfg}
}

// List 点位分页列表。
func (s *PointService) List(c *gin.Context, q *dto.PointListQuery) (*response.Page, *errs.Error) {
	db := s.db.Model(&model.InspectionPoint{})
	if q.CommunityID != "" {
		db = db.Where("community_id = ?", q.CommunityID)
	}
	if q.BuildingID != "" {
		db = db.Where("building_id = ?", q.BuildingID)
	}
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Type != "" {
		db = db.Where("type = ?", q.Type)
	}
	if q.Credential != "" {
		db = db.Where("credential = ?", q.Credential)
	}
	if status, ok, _ := bind.StatusFilter(q.Status); ok {
		db = db.Where("status = ?", status)
	}
	db = middleware.ApplyCommunityFilter(db, c, "community_id")
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.InspectionPoint
	offset, limit := q.Normalize()
	if err := db.Order("sort ASC, id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, s.toItem(&rows[i]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *PointService) toItem(p *model.InspectionPoint) gin.H {
	commName, buildingName, typeLabel := "", "", ""
	var comm sysmodel.Community
	if s.db.Select("name").First(&comm, "id = ?", p.CommunityID).Error == nil {
		commName = comm.Name
	}
	if p.BuildingID != nil {
		var b model.Building
		if s.db.Select("name").First(&b, "id = ?", *p.BuildingID).Error == nil {
			buildingName = b.Name
		}
	}
	var dd sysmodel.SysDictData
	if s.db.Select("label").Where("type_code = 'point_type' AND value = ?", p.Type).First(&dd).Error == nil {
		typeLabel = dd.Label
	}
	templateName := ""
	if p.TemplateID != nil {
		var t model.CheckTemplate
		if s.db.Select("name").First(&t, "id = ?", *p.TemplateID).Error == nil {
			templateName = t.Name
		}
	}
	return gin.H{
		"id": p.ID, "community_id": p.CommunityID, "community_name": commName,
		"building_id": p.BuildingID, "building_name": buildingName,
		"name": p.Name, "type": p.Type, "type_label": typeLabel,
		"qrcode_no": p.QRCodeNo, "nfc_id": p.NfcID,
		"template_id": p.TemplateID, "template_name": templateName,
		"longitude": p.Longitude, "latitude": p.Latitude,
		"fence_radius": p.FenceRadius, "credential": p.Credential, "require_fence": p.RequireFence,
		"required_photo_items": p.RequiredPhotoItems,
		"sort": p.Sort, "status": sysmodel.StatusInt(p.Status), "created_at": timefmt.T(p.CreatedAt),
	}
}

// Create 新增点位；qrcode_no 按 P-+6位序列 自动生成。
func (s *PointService) Create(c *gin.Context, req *dto.PointSaveReq) (string, string, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return "", "", be
	}
	if be := s.validate(req); be != nil {
		return "", "", be
	}
	p := model.InspectionPoint{
		TenantID:           middleware.CommunityTenantID(s.db, req.CommunityID), // 冗余列（=所属小区租户）
		CommunityID:        req.CommunityID,
		BuildingID:         req.BuildingID,
		Name:               req.Name,
		Type:               req.Type,
		TemplateID:         templatePtr(req.TemplateID),
		NfcID:              normalizeNfcID(req.NfcID),
		Longitude:          req.Longitude,
		Latitude:           req.Latitude,
		FenceRadius:        s.fenceRadius(req.FenceRadius),
		Credential:         credentialOrDefault(req.Credential),
		RequireFence:       req.RequireFence,
		RequiredPhotoItems: req.RequiredPhotoItems,
		Sort:               req.Sort,
		Status:             sysmodel.StatusEnabled,
		Remark:             req.Remark,
	}
	if req.Status != nil {
		p.Status = sysmodel.StatusStr(*req.Status)
	}
	if p.RequiredPhotoItems == nil {
		p.RequiredPhotoItems = types.StringArray{}
	}
	// 二维码编号：P-+6 位序列（序号源为 PG 序列 qrcode_no_seq，业务编号与 UUID 主键解耦）
	no, be := s.nextQRCodeNo()
	if be != nil {
		return "", "", be
	}
	p.QRCodeNo = no
	if err := s.db.Create(&p).Error; err != nil {
		return "", "", errs.ErrInternal
	}
	return p.ID, p.QRCodeNo, nil
}

// normalizeNfcID NFC 卡号统一大写去空白（插件读出为大写十六进制，手工录入可能小写，入库统一规格）。
func normalizeNfcID(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

// nextQRCodeNo 取下一个二维码编号（P+6 位序列，如 P000018；无分隔符，肉眼报号与扫码解析都最简单）。
func (s *PointService) nextQRCodeNo() (string, *errs.Error) {
	var seq int64
	if err := s.db.Raw("SELECT nextval('qrcode_no_seq')").Scan(&seq).Error; err != nil {
		return "", errs.ErrInternal
	}
	return fmt.Sprintf("P%06d", seq), nil
}

// Detail 点位详情（含引用中的启用计划）。
func (s *PointService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var p model.InspectionPoint
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return nil, be
	}
	item := s.toItem(&p)
	item["updated_at"] = timefmt.T(p.UpdatedAt)
	var plans []model.InspectionPlan
	s.db.Select("id", "name").Where("point_ids @> ?::jsonb AND status = ?", fmt.Sprintf(`["%s"]`, id), sysmodel.StatusEnabled).Find(&plans)
	refs := make([]gin.H, 0, len(plans))
	for _, pl := range plans {
		refs = append(refs, gin.H{"id": pl.ID, "name": pl.Name})
	}
	item["referenced_plans"] = refs
	return item, nil
}

// Update 修改点位（qrcode_no 不可改）。
func (s *PointService) Update(c *gin.Context, id string, req *dto.PointSaveReq) *errs.Error {
	var p model.InspectionPoint
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	if be := s.validate(req); be != nil {
		return be
	}
	updates := map[string]any{
		"community_id": req.CommunityID, "building_id": req.BuildingID, "name": req.Name,
		"type": req.Type, "template_id": templatePtr(req.TemplateID), "nfc_id": normalizeNfcID(req.NfcID),
		"longitude": req.Longitude, "latitude": req.Latitude,
		"fence_radius": s.fenceRadius(req.FenceRadius), "credential": credentialOrDefault(req.Credential), "require_fence": req.RequireFence,
		"required_photo_items": types.StringArray(req.RequiredPhotoItems),
		"sort": req.Sort, "remark": req.Remark,
	}
	if req.Status != nil {
		updates["status"] = sysmodel.StatusStr(*req.Status)
	}
	if err := s.db.Model(&p).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 软删除；被启用计划引用时拒绝（43002）。
func (s *PointService) Delete(c *gin.Context, id string) *errs.Error {
	var p model.InspectionPoint
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
		return be
	}
	var count int64
	s.db.Model(&model.InspectionPlan{}).
		Where("point_ids @> ?::jsonb AND status = ?", fmt.Sprintf(`["%s"]`, id), sysmodel.StatusEnabled).Count(&count)
	if count > 0 {
		return errs.ErrPointReferenced
	}
	if err := s.db.Delete(&p).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// MapPoints 小区全部点位坐标（不分页，地图撒点用）。
func (s *PointService) MapPoints(c *gin.Context, communityID string) ([]gin.H, *errs.Error) {
	if communityID == "" {
		return nil, errs.ErrParam.WithMsg("community_id 为必填项")
	}
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	var rows []model.InspectionPoint
	if err := s.db.Where("community_id = ?", communityID).Order("sort ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for _, p := range rows {
		buildingName := ""
		if p.BuildingID != nil {
			var b model.Building
			if s.db.Select("name").First(&b, "id = ?", *p.BuildingID).Error == nil {
				buildingName = b.Name
			}
		}
		list = append(list, gin.H{
			"id": p.ID, "name": p.Name, "building_name": buildingName,
			"longitude": p.Longitude, "latitude": p.Latitude,
			"fence_radius": p.FenceRadius, "credential": p.Credential, "require_fence": p.RequireFence,
			"status": sysmodel.StatusInt(p.Status),
		})
	}
	return list, nil
}

// QRCodeBatch 批量生成二维码并打包 zip 下载。
// 码内容为短链接 {APP_BASE_URL}/p/{code}：外来人员用微信/相机扫码直接打开点位信息公开页；
// 巡检端扫码后由客户端提取编号（兼容旧版纯编号二维码，见 app/utils/scan.ts extractPointCode）。
func (s *PointService) QRCodeBatch(c *gin.Context, req *dto.QRCodeBatchReq) (gin.H, *errs.Error) {
	if len(req.PointIDs) > 200 {
		return nil, errs.ErrParam.WithMsg("单次最多生成 200 个点位二维码")
	}
	var points []model.InspectionPoint
	if err := s.db.Where("id IN ?", req.PointIDs).Find(&points).Error; err != nil {
		return nil, errs.ErrInternal
	}
	if len(points) == 0 {
		return nil, errs.ErrNotFound
	}
	for _, p := range points {
		if be := middleware.CheckCommunity(s.db, c, p.CommunityID); be != nil {
			return nil, be
		}
	}
	withTitle := true
	if req.WithTitle != nil {
		withTitle = *req.WithTitle
	}
	// 预载小区/楼栋名称（标题行「小区·楼栋」用）
	commNames := map[string]string{}
	{
		var comms []sysmodel.Community
		s.db.Select("id", "name").Find(&comms)
		for _, cm := range comms {
			commNames[cm.ID] = cm.Name
		}
	}
	buildingNames := map[string]string{}
	{
		var bs []model.Building
		s.db.Select("id", "name").Find(&bs)
		for _, b := range bs {
			buildingNames[b.ID] = b.Name
		}
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	origin := strings.TrimRight(s.store.BaseURL(), "/")
	for _, p := range points {
		content := origin + "/p/" + p.QRCodeNo
		img, err := qrcode.New(content, qrcode.Medium)
		if err != nil {
			return nil, errs.ErrInternal
		}
		pngBytes, err := img.PNG(600)
		if err != nil {
			return nil, errs.ErrInternal
		}
		if withTitle {
			location := commNames[p.CommunityID]
			if p.BuildingID != nil {
				location += "·" + buildingNames[*p.BuildingID]
			}
			pngBytes = appendTitle(pngBytes, p.Name, location, p.QRCodeNo)
		}
		w, err := zw.Create(fmt.Sprintf("%s_%s.png", p.QRCodeNo, p.Name))
		if err != nil {
			return nil, errs.ErrInternal
		}
		if _, err := w.Write(pngBytes); err != nil {
			return nil, errs.ErrInternal
		}
	}
	if err := zw.Close(); err != nil {
		return nil, errs.ErrInternal
	}
	commName := "多小区"
	if len(points) > 0 {
		var comm sysmodel.Community
		if s.db.Select("name").First(&comm, "id = ?", points[0].CommunityID).Error == nil {
			commName = comm.Name
		}
	}
	fileName := fmt.Sprintf("点位二维码_%s_%s.zip", commName, time.Now().Format("20060102"))
	zipData := buf.Bytes()
	key, url, err := s.store.SaveGenerated("qrcode", fileName, zipData)
	if err != nil {
		return nil, errs.ErrInternal
	}
	systemsvc.RegisterGeneratedFile(s.db, s.store, fileName, "application/zip", storage.MD5Hex(zipData), key, url, int64(len(zipData)))
	return gin.H{
		"file_url":  url,
		"file_name": fileName,
		"expire_at": time.Now().Add(24 * time.Hour).Format(timefmt.Layout),
	}, nil
}

// appendTitle 在二维码图下方拼接标牌区（LOGO + 三行居中文字：点位名称 / 小区·楼栋 / 编号），方便打印张贴时对号。
// 字号随 600px 图宽设计：名称 42、其余 34，全量 hinting 保证打印清晰。
func appendTitle(pngBytes []byte, name, location, no string) []byte {
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return pngBytes
	}
	h := 276 // 上边距 14 + LOGO(88) + 三行文字 + 下边距
	dst := image.NewRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()+h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, src.Bounds(), src, image.Point{}, draw.Src)
	y := src.Bounds().Dy() + 14
	y += watermark.DrawLogoCentered(dst, y, 260, 88) // 无 LOGO 时返回 0，文字自然上移
	watermark.TextCenterRGBA(dst, y+52, name, 42)
	watermark.TextCenterRGBA(dst, y+106, location, 34)
	watermark.TextCenterRGBA(dst, y+154, no, 34)
	var out bytes.Buffer
	if png.Encode(&out, dst) != nil {
		return pngBytes
	}
	return out.Bytes()
}

// validate 点位参数校验。
func (s *PointService) validate(req *dto.PointSaveReq) *errs.Error {
	var count int64
	s.db.Model(&sysmodel.Community{}).Where("id = ?", req.CommunityID).Count(&count)
	if count == 0 {
		return errs.ErrCommunityNotExist
	}
	if req.BuildingID != nil {
		s.db.Model(&model.Building{}).Where("id = ? AND community_id = ?", *req.BuildingID, req.CommunityID).Count(&count)
		if count == 0 {
			return errs.ErrParam.WithMsg("building_id 不存在或不属于该小区")
		}
	}
	if req.FenceRadius != 0 && (req.FenceRadius < 10 || req.FenceRadius > 2000) {
		return errs.ErrParam.WithMsg("fence_radius 须在 10–2000 米之间")
	}
	if req.Longitude < -180 || req.Longitude > 180 || req.Latitude < -90 || req.Latitude > 90 {
		return errs.ErrParam.WithMsg("经纬度取值非法")
	}
	if tid := templatePtr(req.TemplateID); tid != nil {
		s.db.Model(&model.CheckTemplate{}).Where("id = ?", *tid).Count(&count)
		if count == 0 {
			return errs.ErrParam.WithMsg("template_id 对应的检查项模板不存在")
		}
	}
	switch credentialOrDefault(req.Credential) {
	case model.CredentialQRCode, model.CredentialNFC, model.CredentialNone, model.CredentialAny:
	default:
		return errs.ErrParam.WithMsg("credential 取值非法（qrcode/nfc/none/any）")
	}
	// 凭证与围栏至少启用一项
	if credentialOrDefault(req.Credential) == model.CredentialNone && !req.RequireFence {
		return errs.ErrParam.WithMsg("点位凭证与围栏校验至少启用一项")
	}
	if credentialOrDefault(req.Credential) == model.CredentialNFC && strings.TrimSpace(req.NfcID) == "" {
		return errs.ErrParam.WithMsg("凭证方式为 NFC 时须填写 NFC 卡号")
	}
	return nil
}

// templatePtr 空字符串模板 ID 转 NULL。
func templatePtr(id *string) *string {
	if id == nil || *id == "" {
		return nil
	}
	return id
}

// fenceRadius 围栏半径：缺省取系统参数 inspection.fence_default_radius。
func (s *PointService) fenceRadius(v int) int {
	if v > 0 {
		return v
	}
	if s.getCfg != nil {
		if val, ok := s.getCfg("inspection.fence_default_radius"); ok {
			var n int
			if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return 100
}

// credentialOrDefault 凭证方式缺省为扫码。
func credentialOrDefault(c string) string {
	if c == "" {
		return model.CredentialQRCode
	}
	return c
}

// pointImportMaxRows 点位单次导入数据行上限。
const pointImportMaxRows = 500

// importModeMap 导入模板「打卡方式」中文 → (凭证, 是否围栏校验)。
var importModeMap = map[string][2]any{
	"扫码":    {model.CredentialQRCode, false},
	"NFC":   {model.CredentialNFC, false},
	"任一":    {model.CredentialAny, false},
	"围栏":    {model.CredentialNone, true},
	"扫码+围栏": {model.CredentialQRCode, true},
	"NFC+围栏": {model.CredentialNFC, true},
	"任一+围栏": {model.CredentialAny, true},
}

// Import 逐行校验导入点位（跳过失败行，成功行落库；二维码编号照常自动发号）。
func (s *PointService) Import(c *gin.Context, r io.Reader) (*dto.PointImportResult, string, *errs.Error) {
	rows, err := excel.ParsePointImport(r)
	if err != nil {
		return nil, "", errs.ErrImportFileType
	}
	dataRows := make([][]string, 0, len(rows))
	rowNums := make([]int, 0, len(rows))
	for i, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		dataRows = append(dataRows, row)
		rowNums = append(rowNums, i+3) // Excel 实际行号（1 表头 + 1 示例）
	}
	if len(dataRows) == 0 {
		return nil, "", errs.ErrImportEmpty
	}
	if len(dataRows) > pointImportMaxRows {
		return nil, "", errs.ErrImportTooMany
	}

	// 预载名称映射：小区、楼栋、点位类型字典、启用模板、已有点位（防重复）
	// 租户隔离（P3）：非超管仅解析本租户小区，避免按名称跨租户写入；
	// 超管同样限定「租户上下文」（EffectiveTenantID，缺省=默认租户），失败直接报错返回
	commByName := map[string]string{}
	{
		q := s.db.Select("id", "name")
		if identity := middleware.CurrentIdentity(c); identity != nil {
			if identity.SuperAdmin {
				tid, be := middleware.EffectiveTenantID(c, s.db)
				if be != nil {
					return nil, "", be
				}
				q = q.Where("tenant_id = ?", tid)
			} else {
				q = q.Where("tenant_id = ?", identity.TenantID)
			}
		}
		var comms []sysmodel.Community
		q.Find(&comms)
		for _, cm := range comms {
			commByName[cm.Name] = cm.ID
		}
	}
	buildingByKey := map[string]string{} // commID|楼栋名 → id
	{
		var bs []model.Building
		s.db.Select("id", "community_id", "name").Find(&bs)
		for _, b := range bs {
			buildingByKey[b.CommunityID+"|"+b.Name] = b.ID
		}
	}
	typeByText := map[string]string{} // 字典 label 与 value 均可匹配
	{
		var dds []sysmodel.SysDictData
		s.db.Select("label", "value").Where("type_code = 'point_type'").Find(&dds)
		for _, dd := range dds {
			typeByText[dd.Label] = dd.Value
			typeByText[dd.Value] = dd.Value
		}
	}
	templateByName := map[string]string{}
	{
		var ts []model.CheckTemplate
		s.db.Select("id", "name").Where("status = ?", sysmodel.StatusEnabled).Find(&ts)
		for _, t := range ts {
			templateByName[t.Name] = t.ID
		}
	}
	existingNames := map[string]bool{} // commID|buildingID|名称（楼栋空归为 ""）
	{
		var ps []model.InspectionPoint
		s.db.Select("community_id", "building_id", "name").Find(&ps)
		for _, p := range ps {
			bid := ""
			if p.BuildingID != nil {
				bid = *p.BuildingID
			}
			existingNames[p.CommunityID+"|"+bid+"|"+p.Name] = true
		}
	}

	result := &dto.PointImportResult{Total: len(dataRows), FailDetails: []dto.PointImportFail{}}
	seen := map[string]bool{}
	for i, row := range dataRows {
		cell := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		commText, buildingText, name := cell(0), cell(1), cell(2)
		typeText, tplText, nfcID := cell(3), cell(4), cell(5)
		lonText, latText, radiusText, modeText := cell(6), cell(7), cell(8), cell(9)
		photoText, statusText, remark := cell(10), cell(11), cell(12)

		fail := func(reason string) {
			result.FailDetails = append(result.FailDetails, dto.PointImportFail{Row: rowNums[i], Name: name, Reason: reason})
		}
		// 1. 必填：小区/点位名称/点位类型/经纬度
		if commText == "" || name == "" || typeText == "" || lonText == "" || latText == "" {
			fail("小区/点位名称/点位类型/经度/纬度均为必填")
			continue
		}
		// 2. 小区存在性与数据权限
		commID, ok := commByName[commText]
		if !ok {
			fail("小区「" + commText + "」不存在")
			continue
		}
		if be := middleware.CheckCommunity(s.db, c, commID); be != nil {
			fail("无小区「" + commText + "」的数据权限")
			continue
		}
		// 3. 楼栋（可空；填写则须属于该小区）
		var buildingID *string
		if buildingText != "" {
			bid, ok := buildingByKey[commID+"|"+buildingText]
			if !ok {
				fail("楼栋「" + buildingText + "」不存在或不属于该小区")
				continue
			}
			buildingID = &bid
		}
		// 4. 点位类型（字典 label/value 均可）
		pointType, ok := typeByText[typeText]
		if !ok {
			fail("点位类型「" + typeText + "」不在字典 point_type 中")
			continue
		}
		// 5. 检查项模板（可空，须为启用模板）
		var templateID *string
		if tplText != "" {
			tid, ok := templateByName[tplText]
			if !ok {
				fail("检查项模板「" + tplText + "」不存在或已停用")
				continue
			}
			templateID = &tid
		}
		// 6. 经纬度与围栏半径
		lon, errLon := strconv.ParseFloat(lonText, 64)
		lat, errLat := strconv.ParseFloat(latText, 64)
		if errLon != nil || errLat != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
			fail("经纬度格式或取值非法")
			continue
		}
		radius := 0
		if radiusText != "" {
			n, err := strconv.Atoi(radiusText)
			if err != nil || n < 10 || n > 2000 {
				fail("围栏半径须为 10–2000 的整数")
				continue
			}
			radius = n
		}
		// 7. 打卡方式（默认扫码+围栏）；NFC 凭证必须填卡号
		credential, requireFence := model.CredentialQRCode, true
		if modeText != "" {
			m, ok := importModeMap[modeText]
			if !ok {
				fail("打卡方式「" + modeText + "」非法（扫码/NFC/任一/围栏/扫码+围栏/NFC+围栏/任一+围栏）")
				continue
			}
			credential, _ = m[0].(string)
			requireFence, _ = m[1].(bool)
		}
		if credential == model.CredentialNFC && nfcID == "" {
			fail("打卡方式含 NFC 时 NFC卡号 必填")
			continue
		}
		// 8. 必拍项（英文逗号分隔，可空）
		photoItems := types.StringArray{}
		if photoText != "" {
			for _, item := range strings.Split(photoText, ",") {
				if item = strings.TrimSpace(item); item != "" {
					photoItems = append(photoItems, item)
				}
			}
		}
		// 9. 同楼栋下名称防重（库中或本批次）
		bid := ""
		if buildingID != nil {
			bid = *buildingID
		}
		dupKey := commID + "|" + bid + "|" + name
		if existingNames[dupKey] || seen[dupKey] {
			fail("同楼栋下已存在同名点位")
			continue
		}
		status := sysmodel.StatusEnabled
		if statusText == "停用" {
			status = sysmodel.StatusDisabled
		}
		no, be := s.nextQRCodeNo()
		if be != nil {
			return nil, "", be
		}
		p := model.InspectionPoint{
			TenantID: middleware.CommunityTenantID(s.db, commID), // 冗余列（=所属小区租户）
			CommunityID: commID, BuildingID: buildingID, Name: name, Type: pointType,
			TemplateID: templateID, NfcID: normalizeNfcID(nfcID), QRCodeNo: no,
			Longitude: lon, Latitude: lat, FenceRadius: s.fenceRadius(radius),
			Credential: credential, RequireFence: requireFence, RequiredPhotoItems: photoItems,
			Status: status, Remark: remark,
		}
		if err := s.db.Create(&p).Error; err != nil {
			fail("写入失败：" + err.Error())
			continue
		}
		seen[dupKey] = true
		result.SuccessCount++
	}
	result.FailCount = len(result.FailDetails)
	msg := fmt.Sprintf("导入完成：成功 %d 条，失败 %d 条", result.SuccessCount, result.FailCount)
	return result, msg, nil
}

// isEmptyRow 判断整行为空。
func isEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
