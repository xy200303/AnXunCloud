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
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/xuri/excelize/v2"
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
		"unit_no": p.UnitNo, "floor": p.Floor,
		"name": p.Name, "type": p.Type, "type_label": typeLabel,
		"qrcode_no": p.QRCodeNo, "nfc_id": p.NfcID,
		"template_id": p.TemplateID, "template_name": templateName,
		"longitude": p.Longitude, "latitude": p.Latitude,
		"fence_radius": p.FenceRadius, "credential": p.Credential, "require_fence": p.RequireFence,
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
		TenantID:     middleware.CommunityTenantID(s.db, req.CommunityID), // 冗余列（=所属小区租户）
		CommunityID:  req.CommunityID,
		BuildingID:   req.BuildingID,
		Name:         req.Name,
		Type:         req.Type,
		TemplateID:   templatePtr(req.TemplateID),
		NfcID:        normalizeNfcID(req.NfcID),
		Longitude:    req.Longitude,
		Latitude:     req.Latitude,
		FenceRadius:  s.fenceRadius(req.FenceRadius),
		Credential:   credentialOrDefault(req.Credential),
		RequireFence: req.RequireFence,
		Sort:         req.Sort,
		Status:       sysmodel.StatusEnabled,
		Remark:       req.Remark,
	}
	// 结构化位置：仅挂楼栋时有意义；非楼栋点位强制清空（车库/公园等区域无单元楼层）
	if req.BuildingID != nil {
		p.UnitNo = req.UnitNo
		p.Floor = req.Floor
	}
	if req.Status != nil {
		p.Status = sysmodel.StatusStr(*req.Status)
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
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return be
	}
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return errs.ErrCommunityNotExist
	}
	if be := s.validate(req); be != nil {
		return be
	}
	updates := map[string]any{
		"tenant_id": tenantID, "community_id": req.CommunityID, "building_id": req.BuildingID, "name": req.Name,
		"type": req.Type, "template_id": templatePtr(req.TemplateID), "nfc_id": normalizeNfcID(req.NfcID),
		"longitude": req.Longitude, "latitude": req.Latitude,
		"fence_radius": s.fenceRadius(req.FenceRadius), "credential": credentialOrDefault(req.Credential), "require_fence": req.RequireFence,
		"sort": req.Sort, "remark": req.Remark,
	}
	// 结构化位置：挂楼栋时写入，非楼栋点位强制清空
	if req.BuildingID != nil {
		updates["unit_no"] = req.UnitNo
		updates["floor"] = req.Floor
	} else {
		updates["unit_no"] = nil
		updates["floor"] = nil
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
	buildingNames := pointBuildingNames(s.db, rows)
	list := make([]gin.H, 0, len(rows))
	for _, p := range rows {
		buildingName := ""
		if p.BuildingID != nil {
			buildingName = buildingNames[*p.BuildingID]
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
// 码内容为短链接 {APP_BASE_URL}/p/{code}：外来人员用微信/相机扫码直接打开点位信息公开页。
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
	systemsvc.RegisterGeneratedFile(s.db, s.store, fileName, "application/zip", storage.MD5Hex(zipData), key, url)
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
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return errs.ErrCommunityNotExist
	}
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
	} else if req.UnitNo != nil || req.Floor != nil {
		// 单元/楼层是楼栋下的结构化位置，非楼栋点位（车库/公园/区域）不允许填
		return errs.ErrParam.WithMsg("单元/楼层仅在点位挂靠楼栋时可填写")
	}
	if req.UnitNo != nil && *req.UnitNo <= 0 {
		return errs.ErrParam.WithMsg("单元号须为正整数")
	}
	if req.Floor != nil && (*req.Floor < -50 || *req.Floor > 200 || *req.Floor == 0) {
		return errs.ErrParam.WithMsg("楼层取值须为 -50~200 且不为 0（地下层用负数，-1 即 B1）")
	}
	if req.FenceRadius != 0 && (req.FenceRadius < 10 || req.FenceRadius > 2000) {
		return errs.ErrParam.WithMsg("fence_radius 须在 10–2000 米之间")
	}
	if req.Longitude < -180 || req.Longitude > 180 || req.Latitude < -90 || req.Latitude > 90 {
		return errs.ErrParam.WithMsg("经纬度取值非法")
	}
	// 点位强制绑定检查项模板（v21 起）：必拍项/逐项判定均由模板驱动，无模板点位不可打卡
	tid := templatePtr(req.TemplateID)
	if tid == nil {
		return errs.ErrParam.WithMsg("点位必须绑定检查项模板")
	}
	s.db.Model(&model.CheckTemplate{}).Where("id = ? AND (tenant_id IS NULL OR tenant_id = ?)", *tid, *tenantID).Count(&count)
	if count == 0 {
		return errs.ErrParam.WithMsg("template_id 对应的检查项模板不存在")
	}
	switch credentialOrDefault(req.Credential) {
	case model.CredentialQRCode, model.CredentialNFC, model.CredentialNone, model.CredentialAny:
	default:
		return errs.ErrParam.WithMsg("credential 取值非法（qrcode/nfc/none/any）")
	}
	// 免核验点位（credential=none 且无围栏）合法：开放式巡更场景（如演示/低管控点位）
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

// validPointType 点位类型字典驱动校验：仅字典 point_type 的启用项合法（计划圈选/批量建点共用）。
func validPointType(db *gorm.DB, t string) bool {
	var status string
	if err := db.Model(&sysmodel.SysDictData{}).
		Where("type_code = ? AND value = ?", "point_type", t).
		Limit(1).Pluck("status", &status).Error; err != nil {
		return false
	}
	return status == sysmodel.StatusEnabled
}

// pointBatchMaxCount 批量建点单次上限（与导入上限同口径）。
const pointBatchMaxCount = 500

// previewPointsLimit 圈选命中预览返回的示例点位条数上限（命中总数走 COUNT 全量统计）。
const previewPointsLimit = 50

// pointBuildingNames 批量解析点位列表的楼栋名（id→name），消除逐点查询的 N+1。
func pointBuildingNames(db *gorm.DB, pts []model.InspectionPoint) map[string]string {
	idSet := map[string]struct{}{}
	for i := range pts {
		if pts[i].BuildingID != nil && *pts[i].BuildingID != "" {
			idSet[*pts[i].BuildingID] = struct{}{}
		}
	}
	names := map[string]string{}
	if len(idSet) == 0 {
		return names
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var buildings []model.Building
	db.Select("id", "name").Where("id IN ?", ids).Find(&buildings)
	for _, b := range buildings {
		names[b.ID] = b.Name
	}
	return names
}

// BatchCreate 批量建点：按 楼栋×楼层×每层数量 生成点位（《专项巡检与专项检查报告设计方案》§3.3）。
// 名称占位符 {building}/{floor}/{seq}（负楼层渲染为 B1/B2…）；同楼栋下同名跳过（幂等重入）；
// 围栏半径取系统参数缺省值，经纬度随入参（小区无坐标字段，缺省 0——扫码凭证不依赖围栏）。
func (s *PointService) BatchCreate(c *gin.Context, req *dto.PointBatchReq) (*dto.PointBatchResult, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return nil, be
	}
	var count int64
	s.db.Model(&sysmodel.Community{}).Where("id = ?", req.CommunityID).Count(&count)
	if count == 0 {
		return nil, errs.ErrCommunityNotExist
	}
	tenantID := middleware.CommunityTenantID(s.db, req.CommunityID)
	if tenantID == nil {
		return nil, errs.ErrCommunityNotExist
	}
	if !validPointType(s.db, req.Type) {
		return nil, errs.ErrParam.WithMsg("type 须为字典 point_type 的启用项")
	}
	switch credentialOrDefault(req.Credential) {
	case model.CredentialQRCode, model.CredentialNFC, model.CredentialNone, model.CredentialAny:
	default:
		return nil, errs.ErrParam.WithMsg("credential 取值非法（qrcode/nfc/none/any）")
	}
	// 点位强制绑定检查项模板（v21 起）
	tid := templatePtr(req.TemplateID)
	if tid == nil {
		return nil, errs.ErrParam.WithMsg("点位必须绑定检查项模板")
	}
	s.db.Model(&model.CheckTemplate{}).Where("id = ? AND (tenant_id IS NULL OR tenant_id = ?)", *tid, *tenantID).Count(&count)
	if count == 0 {
		return nil, errs.ErrParam.WithMsg("template_id 对应的检查项模板不存在")
	}
	if req.FloorTo < req.FloorFrom {
		return nil, errs.ErrParam.WithMsg("floor_to 不能小于 floor_from")
	}
	// 单元维度：缺省 1（单单元）；不配楼栋时忽略（非楼栋点位无单元/楼层结构）
	unitFrom, unitTo := req.UnitFrom, req.UnitTo
	if unitFrom <= 0 {
		unitFrom = 1
	}
	if unitTo < unitFrom {
		unitTo = unitFrom
	}
	if len(req.BuildingIDs) == 0 {
		unitFrom, unitTo = 1, 1
	}
	perFloor := req.PerFloor
	if perFloor <= 0 {
		perFloor = 1
	}
	if req.Longitude < -180 || req.Longitude > 180 || req.Latitude < -90 || req.Latitude > 90 {
		return nil, errs.ErrParam.WithMsg("经纬度取值非法")
	}
	// 楼栋清单：building_ids 空=整个小区下（不挂楼栋，{building} 渲染为空）
	type buildingRef struct {
		id   *string
		name string
	}
	buildings := []buildingRef{{}}
	if len(req.BuildingIDs) > 0 {
		buildings = buildings[:0]
		for _, bid := range uniqueIDs(req.BuildingIDs) {
			var b model.Building
			if err := s.db.Where("id = ? AND community_id = ?", bid, req.CommunityID).First(&b).Error; err != nil {
				return nil, errs.ErrParam.WithMsg("building_ids 中存在无效或不属于该小区的楼栋")
			}
			buildings = append(buildings, buildingRef{id: &b.ID, name: b.Name})
		}
	}
	if total := len(buildings) * (unitTo - unitFrom + 1) * (req.FloorTo - req.FloorFrom + 1) * perFloor; total > pointBatchMaxCount {
		return nil, errs.ErrParam.WithMsg(fmt.Sprintf("单次最多批量生成 %d 个点位（当前展开 %d 个）", pointBatchMaxCount, total))
	}
	result := &dto.PointBatchResult{Skipped: []dto.PointBatchSkip{}}
	for _, b := range buildings {
		for unit := unitFrom; unit <= unitTo; unit++ {
			for floor := req.FloorFrom; floor <= req.FloorTo; floor++ {
				for seq := 1; seq <= perFloor; seq++ {
					name := renderBatchPointName(req.NamePattern, b.name, unit, floor, seq)
					if n := utf8.RuneCountInString(name); n == 0 || n > 128 {
						result.Skipped = append(result.Skipped, dto.PointBatchSkip{Building: b.name, Name: name, Reason: "渲染后名称为空或超过 128 字"})
						continue
					}
					// 同楼栋下同名已存在则跳过（幂等重入）
					dup := s.db.Model(&model.InspectionPoint{}).Where("community_id = ? AND name = ?", req.CommunityID, name)
					if b.id == nil {
						dup = dup.Where("building_id IS NULL")
					} else {
						dup = dup.Where("building_id = ?", *b.id)
					}
					if dup.Count(&count); count > 0 {
						result.Skipped = append(result.Skipped, dto.PointBatchSkip{Building: b.name, Name: name, Reason: "同楼栋下已存在同名点位"})
						continue
					}
					no, be := s.nextQRCodeNo()
					if be != nil {
						return nil, be
					}
					p := model.InspectionPoint{
						TenantID: tenantID, CommunityID: req.CommunityID, BuildingID: b.id,
						Name: name, Type: req.Type, QRCodeNo: no,
						TemplateID: templatePtr(req.TemplateID),
						Longitude:  req.Longitude, Latitude: req.Latitude,
						FenceRadius: s.fenceRadius(0),
						Credential:  credentialOrDefault(req.Credential),
						Status:      sysmodel.StatusEnabled,
					}
					if b.id != nil {
						u, f := unit, floor
						p.UnitNo = &u
						p.Floor = &f
					}
					if err := s.db.Create(&p).Error; err != nil {
						result.Skipped = append(result.Skipped, dto.PointBatchSkip{Building: b.name, Name: name, Reason: "写入失败"})
						continue
					}
					result.Created++
				}
			}
		}
	}
	return result, nil
}

// renderBatchPointName 批量建点名称渲染：{building} 楼栋名、{floor} 楼层、{seq} 每层序号。
func renderBatchPointName(pattern, building string, unit, floor, seq int) string {
	name := strings.ReplaceAll(pattern, "{building}", building)
	name = strings.ReplaceAll(name, "{unit}", strconv.Itoa(unit))
	name = strings.ReplaceAll(name, "{floor}", floorLabel(floor))
	name = strings.ReplaceAll(name, "{seq}", strconv.Itoa(seq))
	return name
}

// floorLabel 楼层显示名：地下层按惯例渲染 B1/B2…，地上层原样数字。
func floorLabel(floor int) string {
	if floor < 0 {
		return "B" + strconv.Itoa(-floor)
	}
	return strconv.Itoa(floor)
}

// pointImportMaxRows 点位单次导入数据行上限。
const pointImportMaxRows = 500

// importModeMap 导入模板「打卡方式」中文 → (凭证, 是否围栏校验)。
var importModeMap = map[string][2]any{
	"扫码":     {model.CredentialQRCode, false},
	"NFC":    {model.CredentialNFC, false},
	"任一":     {model.CredentialAny, false},
	"围栏":     {model.CredentialNone, true},
	"扫码+围栏":  {model.CredentialQRCode, true},
	"NFC+围栏": {model.CredentialNFC, true},
	"任一+围栏":  {model.CredentialAny, true},
}

// ImportTemplate 生成点位导入模板（选项类字段带下拉验证，数据源按调用方数据范围生成：
// 非超管仅本租户小区，超管显式租户收窄否则全部——与 Import 的名称解析口径一致）。
func (s *PointService) ImportTemplate(c *gin.Context) (*excelize.File, *errs.Error) {
	commNames := []string{}
	{
		q := s.db.Model(&sysmodel.Community{})
		if identity := middleware.CurrentIdentity(c); identity != nil {
			if identity.SuperAdmin {
				tid, be := middleware.ExplicitTenantID(c, s.db)
				if be != nil {
					return nil, be
				}
				if tid != "" {
					q = q.Where("tenant_id = ?", tid)
				}
			} else {
				q = q.Where("tenant_id = ?", identity.TenantID)
			}
		}
		if err := q.Order("name ASC").Pluck("name", &commNames).Error; err != nil {
			return nil, errs.ErrInternal
		}
	}
	typeLabels := []string{}
	if err := s.db.Model(&sysmodel.SysDictData{}).
		Where("type_code = 'point_type' AND status = ?", sysmodel.StatusEnabled).
		Order("sort ASC").Pluck("label", &typeLabels).Error; err != nil {
		return nil, errs.ErrInternal
	}
	tplNames := []string{}
	if err := s.db.Model(&model.CheckTemplate{}).
		Where("status = ?", sysmodel.StatusEnabled).
		Order("name ASC").Pluck("name", &tplNames).Error; err != nil {
		return nil, errs.ErrInternal
	}
	f, err := excel.PointImportTemplate(commNames, typeLabels, tplNames)
	if err != nil {
		return nil, errs.ErrInternal
	}
	return f, nil
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
	// 超管未指定租户时允许解析全部租户，显式指定租户时收窄，失败直接报错返回
	commByName := map[string][]string{}
	var importTenantID string // 解析出的租户上下文（模板映射复用同一口径）
	{
		q := s.db.Select("id", "name")
		if identity := middleware.CurrentIdentity(c); identity != nil {
			tid, be := middleware.TenantScopeOrDefault(c, s.db)
			if be != nil {
				return nil, "", be
			}
			importTenantID = tid
			if tid != "" {
				q = q.Where("tenant_id = ?", tid)
			}
		}
		var comms []sysmodel.Community
		q.Find(&comms)
		for _, cm := range comms {
			commByName[cm.Name] = append(commByName[cm.Name], cm.ID)
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
		tq := s.db.Select("id", "name").Where("status = ?", sysmodel.StatusEnabled)
		if importTenantID != "" {
			tq = tq.Where("tenant_id = ?", importTenantID) // 模板与小区同租户口径（v22 起模板按租户隔离）
		}
		tq.Find(&ts)
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
		statusText, remark := cell(11), cell(12) // cell(10) 原为必拍项列（v21 起废除，必拍由模板项推导，忽略该列）

		fail := func(reason string) {
			result.FailDetails = append(result.FailDetails, dto.PointImportFail{Row: rowNums[i], Name: name, Reason: reason})
		}
		// 1. 必填：小区/点位名称/点位类型（经纬度是否必填取决于打卡方式是否含围栏，见第 6 步）
		if commText == "" || name == "" || typeText == "" {
			fail("小区/点位名称/点位类型均为必填")
			continue
		}
		// 2. 小区存在性与数据权限
		commIDs, ok := commByName[commText]
		if !ok {
			fail("小区「" + commText + "」不存在")
			continue
		}
		if len(commIDs) != 1 {
			fail("小区「" + commText + "」在多个租户中重名，请通过 tenant_id 指定租户")
			continue
		}
		commID := commIDs[0]
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
		// 5. 检查项模板（v21 起必填，须为启用模板）
		if tplText == "" {
			fail("检查项模板必填（v21 起点位强制绑定模板）")
			continue
		}
		var templateID *string
		{
			tid, ok := templateByName[tplText]
			if !ok {
				fail("检查项模板「" + tplText + "」不存在或已停用")
				continue
			}
			templateID = &tid
		}
		// 6. 打卡方式（默认扫码+围栏）；NFC 凭证必须填卡号
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
		// 7. 经纬度与围栏半径：含围栏时经纬度必填；无围栏可留空记 0,0（现场用手机「获取当前位置」刷新补齐）
		lon, lat := 0.0, 0.0
		if lonText == "" && latText == "" {
			if requireFence {
				fail("打卡方式含围栏时 经度/纬度 必填")
				continue
			}
		} else {
			if lonText == "" || latText == "" {
				fail("经度/纬度须同时填写或同时留空")
				continue
			}
			var errLon, errLat error
			lon, errLon = strconv.ParseFloat(lonText, 64)
			lat, errLat = strconv.ParseFloat(latText, 64)
			if errLon != nil || errLat != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
				fail("经纬度格式或取值非法")
				continue
			}
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
		// 8. 同楼栋下名称防重（库中或本批次）
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
			TenantID:    middleware.CommunityTenantID(s.db, commID), // 冗余列（=所属小区租户）
			CommunityID: commID, BuildingID: buildingID, Name: name, Type: pointType,
			TemplateID: templateID, NfcID: normalizeNfcID(nfcID), QRCodeNo: no,
			Longitude: lon, Latitude: lat, FenceRadius: s.fenceRadius(radius),
			Credential: credential, RequireFence: requireFence,
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
