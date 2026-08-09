// Package service 巡检模块业务逻辑：点位/计划/任务/打卡记录。
package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"time"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/dto"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
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
	if q.CheckinMode != "" {
		db = db.Where("checkin_mode = ?", q.CheckinMode)
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
		"fence_radius": p.FenceRadius, "checkin_mode": p.CheckinMode,
		"required_photo_items": p.RequiredPhotoItems,
		"sort": p.Sort, "status": sysmodel.StatusInt(p.Status), "created_at": timefmt.T(p.CreatedAt),
	}
}

// Create 新增点位；qrcode_no 按 P-+6位序列 自动生成。
func (s *PointService) Create(c *gin.Context, req *dto.PointSaveReq) (string, string, *errs.Error) {
	if be := middleware.CheckCommunity(c, req.CommunityID); be != nil {
		return "", "", be
	}
	if be := s.validate(req); be != nil {
		return "", "", be
	}
	p := model.InspectionPoint{
		CommunityID:        req.CommunityID,
		BuildingID:         req.BuildingID,
		Name:               req.Name,
		Type:               req.Type,
		TemplateID:         templatePtr(req.TemplateID),
		NfcID:              req.NfcID,
		Longitude:          req.Longitude,
		Latitude:           req.Latitude,
		FenceRadius:        s.fenceRadius(req.FenceRadius),
		CheckinMode:        modeOrDefault(req.CheckinMode),
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
	var seq int64
	if err := s.db.Raw("SELECT nextval('qrcode_no_seq')").Scan(&seq).Error; err != nil {
		return "", "", errs.ErrInternal
	}
	p.QRCodeNo = fmt.Sprintf("P-%06d", seq)
	if err := s.db.Create(&p).Error; err != nil {
		return "", "", errs.ErrInternal
	}
	return p.ID, p.QRCodeNo, nil
}

// Detail 点位详情（含引用中的启用计划）。
func (s *PointService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var p model.InspectionPoint
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, p.CommunityID); be != nil {
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
	if be := middleware.CheckCommunity(c, p.CommunityID); be != nil {
		return be
	}
	if be := s.validate(req); be != nil {
		return be
	}
	updates := map[string]any{
		"community_id": req.CommunityID, "building_id": req.BuildingID, "name": req.Name,
		"type": req.Type, "template_id": templatePtr(req.TemplateID), "nfc_id": req.NfcID,
		"longitude": req.Longitude, "latitude": req.Latitude,
		"fence_radius": s.fenceRadius(req.FenceRadius), "checkin_mode": modeOrDefault(req.CheckinMode),
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
	if be := middleware.CheckCommunity(c, p.CommunityID); be != nil {
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
	if be := middleware.CheckCommunity(c, communityID); be != nil {
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
			"fence_radius": p.FenceRadius, "checkin_mode": p.CheckinMode,
			"status": sysmodel.StatusInt(p.Status),
		})
	}
	return list, nil
}

// QRCodeBatch 批量生成二维码并打包 zip 下载（码内容 inspection://checkin?no=<qrcode_no>）。
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
		if be := middleware.CheckCommunity(c, p.CommunityID); be != nil {
			return nil, be
		}
	}
	withTitle := true
	if req.WithTitle != nil {
		withTitle = *req.WithTitle
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range points {
		content := fmt.Sprintf("inspection://checkin?no=%s", p.QRCodeNo)
		img, err := qrcode.New(content, qrcode.Medium)
		if err != nil {
			return nil, errs.ErrInternal
		}
		pngBytes, err := img.PNG(400)
		if err != nil {
			return nil, errs.ErrInternal
		}
		if withTitle {
			pngBytes = appendTitle(pngBytes, p.Name, p.QRCodeNo)
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
	_, url, err := s.store.SaveGenerated("qrcode", fileName, buf.Bytes())
	if err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{
		"file_url":  url,
		"file_name": fileName,
		"expire_at": time.Now().Add(24 * time.Hour).Format(timefmt.Layout),
	}, nil
}

// appendTitle 在二维码图下方拼接点位名称与编号（白底）。
func appendTitle(pngBytes []byte, name, no string) []byte {
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return pngBytes
	}
	h := 80
	dst := image.NewRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()+h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, src.Bounds(), src, image.Point{}, draw.Src)
	watermark.TextRGBA(dst, 12, src.Bounds().Dy()+32, name)
	watermark.TextRGBA(dst, 12, src.Bounds().Dy()+64, no)
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
	switch modeOrDefault(req.CheckinMode) {
	case model.ModeQRCode, model.ModeFence, model.ModeEither, model.ModeBoth, model.ModeNFC:
	default:
		return errs.ErrParam.WithMsg("checkin_mode 取值非法")
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

func modeOrDefault(mode string) string {
	if mode == "" {
		return model.ModeEither
	}
	return mode
}
