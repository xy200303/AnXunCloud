// Package service 小程序端打卡核心逻辑（6 步校验、幂等、疑似作弊判定、异常自动建单）。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/module/mp/dto"
	sysmodel "anxuncloud/internal/module/system/model"
	womodel "anxuncloud/internal/module/workorder/model"
	wosvc "anxuncloud/internal/module/workorder/service"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/geo"
	"anxuncloud/internal/pkg/logger"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/watermark"

	"go.uber.org/zap"
)

// CheckinService 打卡服务。
type CheckinService struct {
	db      *gorm.DB
	rdb     *redis.Client
	store   *storage.Storage
	orders  *wosvc.OrderService
	getCfg  func(key string) (string, bool)
	fontOn  bool
}

func NewCheckinService(db *gorm.DB, rdb *redis.Client, store *storage.Storage, orders *wosvc.OrderService, getCfg func(string) (string, bool)) *CheckinService {
	return &CheckinService{db: db, rdb: rdb, store: store, orders: orders, getCfg: getCfg}
}

// cfgInt 读取整数型系统参数。
func (s *CheckinService) cfgInt(key string, def int) int {
	if s.getCfg != nil {
		if v, ok := s.getCfg(key); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
				return n
			}
		}
	}
	return def
}

func (s *CheckinService) cfgFloat(key string, def float64) float64 {
	if s.getCfg != nil {
		if v, ok := s.getCfg(key); ok {
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err == nil && f > 0 {
				return f
			}
		}
	}
	return def
}

func (s *CheckinService) cfgBool(key string, def bool) bool {
	if s.getCfg != nil {
		if v, ok := s.getCfg(key); ok {
			return v == "true"
		}
	}
	return def
}

// Submit 在线打卡提交，返回打卡结果视图。
func (s *CheckinService) Submit(c *gin.Context, inspectorID string, req *dto.CheckinReq) (gin.H, *errs.Error) {
	rec, order, be := s.doCheckin(c.Request.Context(), inspectorID, req, false)
	if be != nil {
		return nil, be
	}
	return s.resultView(rec, order), nil
}

// OfflineSync 离线批量补传（逐条处理，单条失败不影响其他；重复提交幂等拦截）。
func (s *CheckinService) OfflineSync(c *gin.Context, inspectorID string, req *dto.OfflineSyncReq) (gin.H, *errs.Error) {
	limit := s.cfgInt("mp.offline_sync_limit", 50)
	if len(req.Items) > limit {
		return nil, errs.ErrParam.WithMsg(fmt.Sprintf("单次补传最多 %d 条", limit))
	}
	success := []gin.H{}
	failed := []gin.H{}
	for i := range req.Items {
		item := &req.Items[i]
		rec, _, be := s.doCheckin(c.Request.Context(), inspectorID, item, true)
		if be != nil {
			failed = append(failed, gin.H{"point_id": item.PointID, "code": be.Code, "message": be.Msg})
			continue
		}
		success = append(success, gin.H{"point_id": item.PointID, "checkin_id": rec.ID, "checkin_time": timefmt.T(rec.CheckinTime)})
	}
	return gin.H{"success": success, "failed": failed}, nil
}

// doCheckin 打卡主流程（offline 标记离线补传）。
func (s *CheckinService) doCheckin(ctx context.Context, inspectorID string, req *dto.CheckinReq, offline bool) (*insmodel.CheckinRecord, gin.H, *errs.Error) {
	// 防并发重复提交（弱网双击/重试）
	lockKey := fmt.Sprintf("lock:checkin:%s:%s", req.TaskID, req.PointID)
	ok, err := s.rdb.SetNX(ctx, lockKey, "1", 5*time.Second).Result()
	if err != nil {
		return nil, nil, errs.ErrInternal
	}
	if !ok {
		return nil, nil, errs.ErrDuplicateCheckin
	}
	rec, order, be := s.doCheckinLocked(ctx, inspectorID, req, offline)
	// 无论成败均放锁：幂等由 DB 保证（重复点位校验 + 唯一索引 + 客户端 UUIDv7 幂等），
	// 锁仅用于收窄并发窗口；成功后必须放锁，否则客户端携带同一 UUID 的幂等重试会被误拦
	s.rdb.Del(ctx, lockKey)
	return rec, order, be
}

// doCheckinLocked 打卡主流程（已持有防重锁）。
func (s *CheckinService) doCheckinLocked(ctx context.Context, inspectorID string, req *dto.CheckinReq, offline bool) (*insmodel.CheckinRecord, gin.H, *errs.Error) {
	// 客户端 UUIDv7 幂等（UUID 主键的核心收益）：离线暂存时客户端生成 ID，补传/重试携带同一 ID，
	// 已存在则直接返回已有记录，不产生重复数据
	if req.ID != "" {
		if be := validateClientID(req.ID); be != nil {
			return nil, nil, be
		}
		var exist insmodel.CheckinRecord
		if err := s.db.Where("id = ?", req.ID).First(&exist).Error; err == nil {
			var order gin.H
			var wo womodel.WorkOrder
			if s.db.Select("id", "order_no").Where("checkin_id = ?", exist.ID).First(&wo).Error == nil {
				order = gin.H{"id": wo.ID, "order_no": wo.OrderNo}
			}
			return &exist, order, nil
		}
	}
	var task insmodel.InspectionTask
	if err := s.db.First(&task, "id = ?", req.TaskID).Error; err != nil || task.InspectorID != inspectorID {
		return nil, nil, errs.ErrTaskNotOwned
	}
	if task.Status == insmodel.TaskDone {
		return nil, nil, errs.ErrDuplicateCheckin.WithMsg("任务已完成")
	}
	// 点位须属于该任务路线
	var plan insmodel.InspectionPlan
	if err := s.db.First(&plan, "id = ?", task.PlanID).Error; err != nil {
		return nil, nil, errs.ErrTaskNotOwned
	}
	if !plan.PointIDs.Contains(req.PointID) {
		return nil, nil, errs.ErrTaskNotOwned.WithMsg("点位不属于该任务")
	}
	// ② 重复打卡校验（43103）
	var dupCount int64
	s.db.Model(&insmodel.CheckinRecord{}).Where("task_id = ? AND point_id = ?", req.TaskID, req.PointID).Count(&dupCount)
	if dupCount > 0 {
		return nil, nil, errs.ErrDuplicateCheckin
	}
	var point insmodel.InspectionPoint
	if err := s.db.First(&point, "id = ?", req.PointID).Error; err != nil {
		return nil, nil, errs.ErrNotFound.WithMsg("点位不存在")
	}
	// 客户端时间解析
	clientTime, err := timefmt.Parse(req.ClientTime)
	if err != nil {
		return nil, nil, errs.ErrParam.WithMsg("client_time 格式应为 YYYY-MM-DD HH:mm:ss")
	}
	// ③ 打卡方式校验（码值 / 围栏距离）
	distance := geo.Haversine(req.Longitude, req.Latitude, point.Longitude, point.Latitude)
	if be := s.checkMode(req, &point, distance); be != nil {
		return nil, nil, be
	}
	// ④ 必拍项完整性与照片上传确认（43104 / 43106）
	photos, be := s.resolvePhotos(req, &point)
	if be != nil {
		return nil, nil, be
	}
	// 异常时描述必填
	if req.Result == insmodel.ResultAbnormal && strings.TrimSpace(req.Remark) == "" {
		return nil, nil, errs.ErrParam.WithMsg("异常打卡必须填写异常描述")
	}
	// ⑤ 疑似作弊判定（距离倍数 / EXIF 偏差 / 客户端时间偏差，不阻断提交）
	isSuspect, suspectReason := s.suspectCheck(&point, distance, clientTime, photos)
	// 落库类型：离线补传统一记 offline
	checkinType := req.CheckinType
	if offline {
		checkinType = "offline"
	}
	now := time.Now()
	rec := insmodel.CheckinRecord{
		TaskID: req.TaskID, PointID: req.PointID, InspectorID: inspectorID,
		CommunityID: task.CommunityID, CheckinTime: now, ClientTime: &clientTime,
		Longitude: &req.Longitude, Latitude: &req.Latitude, DistanceToPoint: &distance,
		CheckinType: checkinType, Photos: photos, Result: req.Result, Remark: req.Remark,
		IsOfflineSync: offline, IsSuspect: isSuspect, SuspectReason: suspectReason,
	}
	if req.ID != "" {
		rec.ID = req.ID // 客户端 UUIDv7（BeforeCreate 不覆盖已有值）
	}
	var order gin.H
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rec).Error; err != nil {
			if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate") {
				return errs.ErrDuplicateCheckin
			}
			return err
		}
		// 任务进度原子推进
		updates := map[string]any{"done_points": gorm.Expr("done_points + 1")}
		if task.StartedAt == nil {
			updates["started_at"] = now
		}
		newDone := task.DonePoints + 1
		if newDone >= task.TotalPoints {
			updates["status"] = insmodel.TaskDone
			updates["finished_at"] = now
		} else if task.Status == insmodel.TaskPending || task.Status == insmodel.TaskOverdue {
			updates["status"] = insmodel.TaskDoing
		}
		if err := tx.Model(&insmodel.InspectionTask{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
			return err
		}
		task.DonePoints = newDone
		// ⑥ 异常自动生成工单
		if req.Result == insmodel.ResultAbnormal {
			orderNo, err := s.orders.GenOrderNo(ctx)
			if err != nil {
				return err
			}
			title := fmt.Sprintf("%s异常：%s", point.Name, truncateStr(req.Remark, 40))
			wo, err := wosvc.CreateFromCheckin(tx, orderNo, rec.ID, task.CommunityID, &point.ID, title, req.Remark, photos, inspectorID)
			if err != nil {
				return err
			}
			order = gin.H{"id": wo.ID, "order_no": wo.OrderNo}
		}
		return nil
	})
	if err != nil {
		if be, ok2 := err.(*errs.Error); ok2 {
			return nil, nil, be
		}
		return nil, nil, errs.ErrInternal
	}
	// dev 模式：打卡成功后异步打水印（点位/时间/坐标/姓名）
	if s.store.IsDev() && s.cfgBool("inspection.watermark_enabled", true) {
		go s.applyWatermarks(&rec, &point, inspectorID)
	}
	// 任务进度缓存失效
	s.rdb.Del(ctx, "cache:task:progress:"+task.ID)
	return &rec, order, nil
}

// checkMode 按点位 checkin_mode 校验码值与围栏距离。
func (s *CheckinService) checkMode(req *dto.CheckinReq, point *insmodel.InspectionPoint, distance float64) *errs.Error {
	needQR, needFence := false, false
	switch point.CheckinMode {
	case insmodel.ModeQRCode:
		needQR = true
	case insmodel.ModeFence:
		needFence = true
	case insmodel.ModeEither:
		if req.CheckinType == "qrcode" {
			needQR = true
		} else {
			needFence = true
		}
	case insmodel.ModeBoth:
		needQR, needFence = true, true
	}
	if needQR && req.QRCodeNo != point.QRCodeNo {
		return errs.ErrQRCodeMismatch
	}
	if needFence && distance > float64(point.FenceRadius) {
		return errs.ErrOutOfFence.WithMsg(fmt.Sprintf("距点位 %dm，超出围栏半径 %dm", int(distance), point.FenceRadius))
	}
	return nil
}

// resolvePhotos 必拍项校验 + 照片上传确认，返回落库照片数组。
func (s *CheckinService) resolvePhotos(req *dto.CheckinReq, point *insmodel.InspectionPoint) (types.PhotoArray, *errs.Error) {
	// 必拍项齐全校验
	if len(point.RequiredPhotoItems) > 0 {
		got := map[string]bool{}
		for _, p := range req.Photos {
			got[p.Item] = true
		}
		var missing []string
		for _, item := range point.RequiredPhotoItems {
			if !got[item] {
				missing = append(missing, item)
			}
		}
		if len(missing) > 0 {
			return nil, errs.ErrPhotoMissing.WithMsg("必拍项照片缺失：" + strings.Join(missing, "、"))
		}
	}
	photos := make(types.PhotoArray, 0, len(req.Photos))
	requiredSet := map[string]bool{}
	for _, item := range point.RequiredPhotoItems {
		requiredSet[item] = true
	}
	for _, ref := range req.Photos {
		var f sysmodel.UploadFile
		if err := s.db.Where("file_key = ?", ref.FileKey).First(&f).Error; err != nil {
			return nil, errs.ErrPhotoNotUploaded
		}
		item := types.PhotoItem{
			Item: ref.Item, URL: f.URL, WatermarkedURL: f.WatermarkedURL,
			Required: requiredSet[ref.Item],
		}
		if f.ExifTime != nil {
			item.ExifTime = timefmt.T(*f.ExifTime)
		}
		photos = append(photos, item)
	}
	return photos, nil
}

// suspectCheck 三类疑似作弊判定（距离超倍数 / EXIF 偏差 / 客户端时间偏差）。
func (s *CheckinService) suspectCheck(point *insmodel.InspectionPoint, distance float64, clientTime time.Time, photos types.PhotoArray) (bool, string) {
	ratio := s.cfgFloat("inspection.suspect_distance_ratio", 1.0)
	if distance > float64(point.FenceRadius)*ratio {
		return true, fmt.Sprintf("距点位 %dm，超阈值 %dm", int(distance), int(float64(point.FenceRadius)*ratio))
	}
	exifLimit := s.cfgInt("inspection.exif_deviation_seconds", 300)
	for _, p := range photos {
		if p.ExifTime == "" {
			continue
		}
		if shot, err := timefmt.Parse(p.ExifTime); err == nil {
			dev := shot.Sub(time.Now()).Seconds()
			if dev < 0 {
				dev = -dev
			}
			if dev > float64(exifLimit) {
				return true, fmt.Sprintf("照片拍摄时间与打卡时间偏差 %ds，超阈值 %ds", int(dev), exifLimit)
			}
		}
	}
	timeLimit := s.cfgInt("inspection.time_deviation_seconds", 300)
	dev := time.Since(clientTime).Seconds()
	if dev < 0 {
		dev = -dev
	}
	if dev > float64(timeLimit) {
		return true, fmt.Sprintf("客户端时间与服务端偏差 %ds，超阈值 %ds", int(dev), timeLimit)
	}
	return false, ""
}

// applyWatermarks dev 模式本地打水印并回写照片 watermarked_url。
func (s *CheckinService) applyWatermarks(rec *insmodel.CheckinRecord, point *insmodel.InspectionPoint, inspectorID string) {
	var inspector sysmodel.SysUser
	if s.db.Select("name").First(&inspector, "id = ?", inspectorID).Error != nil {
		return
	}
	lines := []string{
		"点位：" + point.Name,
		"时间：" + timefmt.T(rec.CheckinTime),
		fmt.Sprintf("坐标：%.6f,%.6f", *rec.Longitude, *rec.Latitude),
		"巡检员：" + inspector.Name,
	}
	changed := false
	for i, p := range rec.Photos {
		// 从 url 反推 file_key（dev 本地路径）
		idx := strings.Index(p.URL, "/uploads/")
		if idx < 0 {
			continue
		}
		key := p.URL[idx+len("/uploads/"):]
		srcPath := s.store.LocalPath(key)
		wmKey := strings.TrimSuffix(key, ".jpg") + "_wm.jpg"
		if key == wmKey {
			wmKey = key + "_wm.jpg"
		}
		if err := watermark.DrawToFile(srcPath, s.store.LocalPath(wmKey), "", lines); err != nil {
			logger.L.Warn("水印生成失败", zap.String("key", key), zap.Error(err))
			continue
		}
		rec.Photos[i].WatermarkedURL = s.store.URL(wmKey)
		changed = true
	}
	if changed {
		s.db.Model(&insmodel.CheckinRecord{}).Where("id = ?", rec.ID).Update("photos", rec.Photos)
	}
}

// resultView 打卡响应视图。
func (s *CheckinService) resultView(rec *insmodel.CheckinRecord, order gin.H) gin.H {
	var task insmodel.InspectionTask
	s.db.Select("total_points", "done_points", "status").First(&task, "id = ?", rec.TaskID)
	progress := 0
	if task.TotalPoints > 0 {
		progress = task.DonePoints * 100 / task.TotalPoints
	}
	out := gin.H{
		"checkin_id": rec.ID, "checkin_time": timefmt.T(rec.CheckinTime),
		"distance_to_point": int(*rec.DistanceToPoint),
		"is_suspect": rec.IsSuspect, "suspect_reason": rec.SuspectReason,
		"work_order": order,
		"task_progress": gin.H{
			"total_points": task.TotalPoints, "done_points": task.DonePoints,
			"progress": progress, "task_status": task.Status,
		},
	}
	return out
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// validateClientID 校验客户端打卡 ID：必须 UUIDv7 且时间戳合理（30 天前 ~ 未来 5 分钟内）。
func validateClientID(id string) *errs.Error {
	u, err := uuid.Parse(id)
	if err != nil || u.Version() != 7 {
		return errs.ErrParam.WithMsg("id 须为 UUIDv7 格式")
	}
	sec, _ := u.Time().UnixTime()
	ts := time.Unix(sec, 0)
	if ts.After(time.Now().Add(5*time.Minute)) || ts.Before(time.Now().AddDate(0, 0, -30)) {
		return errs.ErrParam.WithMsg("id 时间戳超出合理范围")
	}
	return nil
}
