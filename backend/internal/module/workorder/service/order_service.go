// Package service 工单业务逻辑：状态机流转 + 全程留痕 + 站内消息。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/dto"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// OrderService 工单服务。
type OrderService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewOrderService(db *gorm.DB, rdb *redis.Client) *OrderService {
	return &OrderService{db: db, rdb: rdb}
}

// GenOrderNo 生成工单号：WX+yyyyMMdd+3位日内序号（Redis INCR，唯一索引兜底）。
func (s *OrderService) GenOrderNo(ctx context.Context) (string, error) {
	day := time.Now().Format("20060102")
	key := "seq:wo:" + day
	seq, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return "", err
	}
	s.rdb.Expire(ctx, key, 48*time.Hour)
	return fmt.Sprintf("WX%s-%03d", day, seq), nil
}

// Notify 写站内消息（微信订阅消息推送预留：msg.subscribe_enabled 开关后续接微信 SDK）。
func (s *OrderService) Notify(userID string, msgType, title, content string, bizID *string) {
	msg := sysmodel.SysMessage{UserID: userID, Type: msgType, Title: title, Content: content, BizID: bizID}
	s.db.Create(&msg)
}

// List 工单分页列表（含状态角标统计）。
func (s *OrderService) List(c *gin.Context, q *dto.OrderListQuery) (*response.Page, gin.H, *errs.Error) {
	base := s.db.Model(&model.WorkOrder{})
	base = middleware.ApplyCommunityFilter(base, c, "work_order.community_id")
	if q.CommunityID != "" {
		base = base.Where("community_id = ?", q.CommunityID)
	}
	// 状态角标（数据权限口径内全量统计）
	// 用 map[string]int64 累加再落成 gin.H，避免 interface{} 类型断言 panic
	type cntRow struct {
		Status string
		Cnt    int64
	}
	var cntRows []cntRow
	base.Session(&gorm.Session{}).Select("status, COUNT(*) AS cnt").Group("status").Scan(&cntRows)
	acc := map[string]int64{"pending": 0, "processing": 0, "review": 0, "closed": 0}
	for _, r := range cntRows {
		switch r.Status {
		case model.OrderPending:
			acc["pending"] += r.Cnt
		case model.OrderAssigned, model.OrderProcessing:
			acc["processing"] += r.Cnt
		case model.OrderReview:
			acc["review"] += r.Cnt
		case model.OrderClosed, model.OrderRejected:
			acc["closed"] += r.Cnt
		}
	}
	counts := gin.H{"pending": acc["pending"], "processing": acc["processing"], "review": acc["review"], "closed": acc["closed"]}

	db := base.Session(&gorm.Session{})
	if q.Status != "" {
		db = db.Where("status IN ?", strings.Split(q.Status, ","))
	}
	if q.Priority != "" {
		db = db.Where("priority = ?", q.Priority)
	}
	if q.AssigneeID != "" {
		db = db.Where("assignee_id = ?", q.AssigneeID)
	}
	if q.ReporterID != "" {
		db = db.Where("reporter_id = ?", q.ReporterID)
	}
	if q.OrderNo != "" {
		db = db.Where("order_no = ?", q.OrderNo)
	}
	var be *errs.Error
	if db, be = woTimeRange(db, "created_at", q.StartTime, q.EndTime); be != nil {
		return nil, nil, be
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, nil, errs.ErrInternal
	}
	var rows []model.WorkOrder
	offset, limit := q.Normalize()
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, s.toItem(&rows[i]))
	}
	page := &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}
	return page, counts, nil
}

func (s *OrderService) toItem(o *model.WorkOrder) gin.H {
	return gin.H{
		"id": o.ID, "order_no": o.OrderNo, "title": o.Title,
		"community_id": o.CommunityID, "community_name": s.commName(o.CommunityID),
		"point_id": o.PointID, "point_name": s.pointName(o.PointID),
		"priority": o.Priority, "status": o.Status,
		"reporter_id": o.ReporterID, "reporter_name": s.userName(o.ReporterID),
		"assignee_id": o.AssigneeID, "assignee_name": s.userNamePtr(o.AssigneeID),
		"created_at": timefmt.T(o.CreatedAt),
	}
}

// Create 后台手工建单；指定处理人则直接 assigned。
func (s *OrderService) Create(ctx context.Context, c *gin.Context, req *dto.OrderCreateReq) (gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(c, req.CommunityID); be != nil {
		return nil, be
	}
	priority := req.Priority
	if priority == "" {
		priority = "normal"
	}
	switch priority {
	case "low", "normal", "high", "urgent":
	default:
		return nil, errs.ErrParam.WithMsg("priority 取值非法")
	}
	if req.AssigneeID != nil {
		if be := s.checkAssignee(*req.AssigneeID); be != nil {
			return nil, be
		}
	}
	photos, be := s.resolvePhotos(req.Photos)
	if be != nil {
		return nil, be
	}
	orderNo, err := s.GenOrderNo(ctx)
	if err != nil {
		return nil, errs.ErrInternal
	}
	identity := middleware.CurrentIdentity(c)
	status := model.OrderPending
	if req.AssigneeID != nil {
		status = model.OrderAssigned
	}
	order := model.WorkOrder{
		OrderNo: orderNo, CommunityID: req.CommunityID, PointID: req.PointID,
		Title: req.Title, Description: req.Description, Photos: photos,
		ReporterID: identity.UserID, AssigneeID: req.AssigneeID,
		Priority: priority, Status: status,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		detail := "后台手工建单"
		if status == model.OrderAssigned {
			detail = "后台手工建单并派单给 " + s.userName(*req.AssigneeID)
		}
		return tx.Create(&model.WorkOrderLog{OrderID: order.ID, Action: model.ActionCreate, OperatorID: identity.UserID, Detail: detail}).Error
	})
	if err != nil {
		return nil, errs.ErrInternal
	}
	if req.AssigneeID != nil {
		s.Notify(*req.AssigneeID, "workorder", "新工单指派", fmt.Sprintf("工单 %s「%s」已指派给您，请及时处理", orderNo, order.Title), &order.ID)
	}
	return gin.H{"id": order.ID, "order_no": orderNo, "status": status}, nil
}

// Detail 工单详情（含流转时间线）。
func (s *OrderService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, o.CommunityID); be != nil {
		return nil, be
	}
	return s.detailOf(&o), nil
}

// detailOf 组装详情（管理后台与小程序共用）。
func (s *OrderService) detailOf(o *model.WorkOrder) gin.H {
	var logs []model.WorkOrderLog
	s.db.Where("order_id = ?", o.ID).Order("created_at ASC, id ASC").Find(&logs)
	logItems := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		logItems = append(logItems, gin.H{
			"action": l.Action, "operator_name": s.userName(l.OperatorID),
			"detail": l.Detail, "created_at": timefmt.T(l.CreatedAt),
		})
	}
	return gin.H{
		"id": o.ID, "order_no": o.OrderNo, "checkin_id": o.CheckinID,
		"title": o.Title, "community_id": o.CommunityID, "community_name": s.commName(o.CommunityID),
		"point_id": o.PointID, "point_name": s.pointName(o.PointID),
		"description": o.Description, "photos": o.Photos,
		"reporter_id": o.ReporterID, "reporter_name": s.userName(o.ReporterID),
		"assignee_id": o.AssigneeID, "assignee_name": s.userNamePtr(o.AssigneeID),
		"priority": o.Priority, "status": o.Status,
		"fix_photos": o.FixPhotos, "fix_remark": o.FixRemark,
		"finished_at": timefmt.TP(o.FinishedAt),
		"reviewed_by": o.ReviewedBy, "review_remark": nullableStr(o.ReviewRemark),
		"created_at": timefmt.T(o.CreatedAt), "logs": logItems,
	}
}

// Update 修改工单（仅 pending 可改）。
func (s *OrderService) Update(c *gin.Context, id string, req *dto.OrderUpdateReq) *errs.Error {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return be
	}
	if o.Status != model.OrderPending {
		return errs.ErrOrderStatusNotAllowed.WithMsg("仅待派单状态可修改")
	}
	updates := map[string]any{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		switch *req.Priority {
		case "low", "normal", "high", "urgent":
		default:
			return errs.ErrParam.WithMsg("priority 取值非法")
		}
		updates["priority"] = *req.Priority
	}
	if err := s.db.Model(o).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 软删除（仅 pending 可删）。
func (s *OrderService) Delete(c *gin.Context, id string) *errs.Error {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return be
	}
	if o.Status != model.OrderPending {
		return errs.ErrOrderStatusNotAllowed.WithMsg("仅待派单状态可删除")
	}
	if err := s.db.Delete(o).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Assign 派单（pending/rejected → assigned）。
func (s *OrderService) Assign(c *gin.Context, id string, req *dto.AssignReq) *errs.Error {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return be
	}
	if o.Status != model.OrderPending && o.Status != model.OrderRejected {
		return errs.ErrOrderStatusNotAllowed
	}
	if be := s.checkAssignee(req.AssigneeID); be != nil {
		return be
	}
	identity := middleware.CurrentIdentity(c)
	assigneeName := s.userName(req.AssigneeID)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(map[string]any{"assignee_id": req.AssigneeID, "status": model.OrderAssigned}).Error; err != nil {
			return err
		}
		detail := "派单给 " + assigneeName
		if req.Remark != "" {
			detail += "（" + req.Remark + "）"
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: model.ActionAssign, OperatorID: identity.UserID, Detail: detail}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	s.Notify(req.AssigneeID, "workorder", "新工单指派", fmt.Sprintf("工单 %s「%s」已指派给您，请及时处理", o.OrderNo, o.Title), &o.ID)
	return nil
}

// Finish 处理反馈（assigned/processing → review；后台代录与小程序共用）。
func (s *OrderService) Finish(operatorID string, o *model.WorkOrder, req *dto.FinishReq) *errs.Error {
	if o.Status != model.OrderAssigned && o.Status != model.OrderProcessing {
		return errs.ErrOrderStatusNotAllowed.WithMsg("当前状态不可提交处理反馈")
	}
	photos, be := s.resolvePhotos(req.FixPhotos)
	if be != nil {
		return be
	}
	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(map[string]any{
			"status": model.OrderReview, "fix_remark": req.FixRemark,
			"fix_photos": photos, "finished_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: model.ActionFinish, OperatorID: operatorID, Detail: "处理反馈：" + req.FixRemark}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Review 复核：pass → closed；reject → processing（退回返工，驳回原因必填）。
func (s *OrderService) Review(c *gin.Context, id string, req *dto.ReviewReq) (string, *errs.Error) {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return "", be
	}
	if o.Status != model.OrderReview {
		return "", errs.ErrOrderStatusNotAllowed.WithMsg("仅待复核状态可执行复核")
	}
	if req.Result == "reject" && strings.TrimSpace(req.ReviewRemark) == "" {
		return "", errs.ErrReviewRemarkRequired
	}
	identity := middleware.CurrentIdentity(c)
	newStatus := model.OrderClosed
	action := model.ActionReviewPass
	if req.Result == "reject" {
		newStatus = model.OrderProcessing
		action = model.ActionReviewReject
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(map[string]any{
			"status": newStatus, "reviewed_by": identity.UserID, "review_remark": req.ReviewRemark,
		}).Error; err != nil {
			return err
		}
		detail := "复核通过"
		if req.Result == "reject" {
			detail = "复核驳回：" + req.ReviewRemark
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: action, OperatorID: identity.UserID, Detail: detail}).Error
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	if req.Result == "reject" && o.AssigneeID != nil {
		s.Notify(*o.AssigneeID, "workorder", "工单复核驳回", fmt.Sprintf("工单 %s 复核驳回：%s，请重新处理", o.OrderNo, req.ReviewRemark), &o.ID)
	}
	return newStatus, nil
}

// woTimeRange 创建时间范围过滤。
func woTimeRange(db *gorm.DB, column, start, end string) (*gorm.DB, *errs.Error) {
	if start != "" {
		t, err := timefmt.Parse(start)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("start_time 格式应为 YYYY-MM-DD HH:mm:ss")
		}
		db = db.Where(column+" >= ?", t)
	}
	if end != "" {
		t, err := timefmt.Parse(end)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("end_time 格式应为 YYYY-MM-DD HH:mm:ss")
		}
		db = db.Where(column+" <= ?", t)
	}
	return db, nil
}

// GetForMP 小程序端取工单（仅上报人/处理人可见）。
func (s *OrderService) GetForMP(id, userID string) (gin.H, *errs.Error) {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if o.ReporterID != userID && (o.AssigneeID == nil || *o.AssigneeID != userID) {
		return nil, errs.ErrNoPerm
	}
	return s.detailOf(&o), nil
}

// FinishForMP 小程序提交反馈（仅当前指派人）。
func (s *OrderService) FinishForMP(id, userID string, req *dto.FinishReq) *errs.Error {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if o.AssigneeID == nil || *o.AssigneeID != userID {
		return errs.ErrNoPerm
	}
	return s.Finish(userID, &o, req)
}

// Accept 接单（assigned → processing），小程序处理人操作。
func (s *OrderService) Accept(id, userID string) *errs.Error {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	if o.AssigneeID == nil || *o.AssigneeID != userID {
		return errs.ErrNoPerm
	}
	if o.Status != model.OrderAssigned {
		return errs.ErrOrderStatusNotAllowed
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&o).Update("status", model.OrderProcessing).Error; err != nil {
			return err
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: model.ActionAccept, OperatorID: userID, Detail: "接单，开始处理"}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	return nil
}

// CreateFromCheckin 异常打卡自动生成工单（在打卡事务内调用）。
func CreateFromCheckin(tx *gorm.DB, orderNo string, checkinID, communityID string, pointID *string, title, description string, photos types.PhotoArray, reporterID string) (*model.WorkOrder, error) {
	order := model.WorkOrder{
		OrderNo: orderNo, CheckinID: &checkinID, CommunityID: communityID, PointID: pointID,
		Title: title, Description: description, Photos: photos,
		ReporterID: reporterID, Priority: "normal", Status: model.OrderPending,
	}
	if err := tx.Create(&order).Error; err != nil {
		return nil, err
	}
	log := model.WorkOrderLog{OrderID: order.ID, Action: model.ActionCreate, OperatorID: reporterID, Detail: "巡检异常上报，自动生成工单"}
	if err := tx.Create(&log).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// GetChecked 取工单并做数据权限校验（供 controller 使用）。
func (s *OrderService) GetChecked(c *gin.Context, id string) (*model.WorkOrder, *errs.Error) {
	return s.getWithScope(c, id)
}

// getWithScope 取工单并做数据权限校验。
func (s *OrderService) getWithScope(c *gin.Context, id string) (*model.WorkOrder, *errs.Error) {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(c, o.CommunityID); be != nil {
		return nil, be
	}
	return &o, nil
}

// checkAssignee 被指派人须存在且启用。
func (s *OrderService) checkAssignee(id string) *errs.Error {
	var count int64
	s.db.Model(&sysmodel.SysUser{}).Where("id = ? AND status = ?", id, sysmodel.StatusEnabled).Count(&count)
	if count == 0 {
		return errs.ErrAssigneeInvalid
	}
	return nil
}

// resolvePhotos 将 file_key 引用解析为照片数组并校验已上传（43106）。
func (s *OrderService) resolvePhotos(refs []dto.PhotoRef) (types.PhotoArray, *errs.Error) {
	photos := types.PhotoArray{}
	for _, ref := range refs {
		var f sysmodel.UploadFile
		if err := s.db.Where("file_key = ?", ref.FileKey).First(&f).Error; err != nil {
			return nil, errs.ErrPhotoNotUploaded
		}
		photos = append(photos, types.PhotoItem{URL: f.URL, WatermarkedURL: f.WatermarkedURL})
	}
	return photos, nil
}

func (s *OrderService) commName(id string) string {
	var c sysmodel.Community
	if s.db.Select("name").First(&c, "id = ?", id).Error == nil {
		return c.Name
	}
	return ""
}

func (s *OrderService) pointName(id *string) string {
	if id == nil {
		return ""
	}
	var name string
	s.db.Table("inspection_point").Select("name").Where("id = ?", *id).Scan(&name)
	return name
}

func (s *OrderService) userName(id string) string {
	var u sysmodel.SysUser
	if s.db.Select("name").First(&u, "id = ?", id).Error == nil {
		return u.Name
	}
	return ""
}

func (s *OrderService) userNamePtr(id *string) any {
	if id == nil {
		return nil
	}
	return s.userName(*id)
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
