// Package service 工单业务逻辑：闭环状态机流转 + 名单制授权（槽位）+ 全程留痕 + 站内消息。
// 状态机见 statemachine.go；业务身份一律走「槽位绑定 → 岗位 → 项目编制名单」解析（设计方案 §5.3），不按角色推导。
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/dto"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/storage"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/types"
)

// OrderService 工单服务。
type OrderService struct {
	db    *gorm.DB
	rdb   *redis.Client
	store *storage.Storage
}

func NewOrderService(db *gorm.DB, rdb *redis.Client, store *storage.Storage) *OrderService {
	return &OrderService{db: db, rdb: rdb, store: store}
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

// notifySlot 按槽位名单发定向通知（名单为空则该环节无待办提醒，与签字名单解析口径一致）。
func (s *OrderService) notifySlot(projectID, slot, title, content string, bizID *string) {
	for _, uid := range communitysvc.SlotUserIDs(s.db, projectID, slot) {
		s.Notify(uid, "workorder", title, content, bizID)
	}
}

// inSlot 名单制授权判定（名单即授权 + 超管/租户管理员默认放行，统一走 communitysvc.SlotAuthorized）。
func (s *OrderService) inSlot(projectID, slot string, id *middleware.Identity) bool {
	return communitysvc.SlotAuthorized(s.db, projectID, slot, id)
}

// DispatchCandidates 派单候选人：本项目「工单接单」槽位名单成员（三级回落解析，含姓名/电话）。
// 候选人解析口径与派单校验完全一致，前端不再自行按岗位/角色过滤。
func (s *OrderService) DispatchCandidates(communityID string) ([]gin.H, *errs.Error) {
	ids := communitysvc.SlotUserIDs(s.db, communityID, sysmodel.SlotOrderAccept)
	out := make([]gin.H, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	idList := make([]string, len(ids)) // IDArray 直接进 Where 会按 JSON 序列化，须先转 []string
	copy(idList, ids)
	var users []sysmodel.SysUser
	if err := s.db.Select("id", "name", "phone").Where("id IN ?", idList).Find(&users).Error; err != nil {
		return nil, errs.ErrInternal
	}
	byID := make(map[string]sysmodel.SysUser, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	for _, id := range ids { // 保持槽位名单顺序（编制创建时间正序）
		if u, ok := byID[id]; ok {
			out = append(out, gin.H{"user_id": u.ID, "user_name": u.Name, "phone": u.Phone})
		}
	}
	return out, nil
}

// communityFlags 读项目级工单开关（triage 受理 / grab 抢单）；项目不存在时按默认（开受理、关抢单）。
func (s *OrderService) communityFlags(communityID string) (triage, grab bool) {
	var comm sysmodel.Community
	if err := s.db.Select("wo_triage_enabled", "wo_grab_enabled").First(&comm, "id = ?", communityID).Error; err != nil {
		return true, false
	}
	return comm.WoTriageEnabled, comm.WoGrabEnabled
}

// validPriority 优先级取值校验（空串返回默认 normal）。
func validPriority(p string) (string, *errs.Error) {
	if p == "" {
		return "normal", nil
	}
	switch p {
	case "low", "normal", "high", "urgent":
		return p, nil
	}
	return "", errs.ErrParam.WithMsg("priority 取值非法")
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
	counts := gin.H{
		"reported": int64(0), "pending_dispatch": int64(0), "processing": int64(0),
		"pending_confirm": int64(0), "closed": int64(0), "closed_invalid": int64(0),
	}
	for _, r := range cntRows {
		if _, ok := counts[r.Status]; ok {
			counts[r.Status] = r.Cnt
		}
	}

	db := base.Session(&gorm.Session{})
	if q.Status != "" {
		db = db.Where("status IN ?", strings.Split(q.Status, ","))
	}
	if q.Priority != "" {
		db = db.Where("priority = ?", q.Priority)
	}
	if q.Source != "" {
		db = db.Where("source = ?", q.Source)
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
		"source": o.Source, "category": o.Category,
		"priority": o.Priority, "status": o.Status,
		"reporter_id": o.ReporterID, "reporter_name": s.userName(o.ReporterID),
		"assignee_id": o.AssigneeID, "assignee_name": s.userNamePtr(o.AssigneeID),
		// SLA 简化实现：按优先级硬编码期望完成时长，仅展示不推送（见 statemachine.go）
		"sla_deadline": timefmt.TP(SLADeadline(o.Priority, o.CreatedAt)),
		"sla_overdue":  SLAOverdue(o.Status, o.Priority, o.CreatedAt, time.Now()),
		"created_at":   timefmt.T(o.CreatedAt),
	}
}

// Create 管理端建单（前台代录，source=frontdesk）；
// 指定处理人须为 order_accept 槽位成员，视为直接派单（processing，省略受理/派单环节）。
func (s *OrderService) Create(ctx context.Context, c *gin.Context, req *dto.OrderCreateReq) (gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, req.CommunityID); be != nil {
		return nil, be
	}
	priority, be := validPriority(req.Priority)
	if be != nil {
		return nil, be
	}
	if req.AssigneeID != nil {
		if be := s.checkAcceptMember(req.CommunityID, *req.AssigneeID); be != nil {
			return nil, be
		}
	}
	if be := s.checkPointBelongs(req.CommunityID, req.PointID); be != nil {
		return nil, be
	}
	photos, be := s.resolvePhotos(req.Photos)
	if be != nil {
		return nil, be
	}
	items, be := s.resolveCreateItems(req.Items)
	if be != nil {
		return nil, be
	}
	orderNo, err := s.GenOrderNo(ctx)
	if err != nil {
		return nil, errs.ErrInternal
	}
	identity := middleware.CurrentIdentity(c)
	now := time.Now()
	order := model.WorkOrder{
		TenantID: middleware.CommunityTenantID(s.db, req.CommunityID), // 冗余列（=所属小区租户）
		OrderNo: orderNo, CommunityID: req.CommunityID, PointID: req.PointID,
		Title: req.Title, Description: req.Description, Photos: photos, Items: items,
		Source: model.SourceFrontdesk, ReporterID: identity.UserID, AssigneeID: req.AssigneeID,
		Priority: priority,
	}
	triageEnabled, _ := s.communityFlags(req.CommunityID)
	detail := "前台代录建单"
	switch {
	case req.AssigneeID != nil:
		// 直接派单：派单即接单
		order.Status = model.OrderProcessing
		order.DispatcherID = &identity.UserID
		order.DispatchAt, order.AcceptAt = &now, &now
		detail = "前台代录建单并派单给 " + s.userName(*req.AssigneeID)
	case triageEnabled:
		order.Status = model.OrderReported
	default:
		order.Status = model.OrderPendingDispatch
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return tx.Create(&model.WorkOrderLog{OrderID: order.ID, Action: model.ActionCreate, OperatorID: identity.UserID, Detail: detail}).Error
	})
	if err != nil {
		return nil, errs.ErrInternal
	}
	s.notifyAfterCreate(&order)
	return gin.H{"id": order.ID, "order_no": orderNo, "status": order.Status}, nil
}

// notifyAfterCreate 建单后的定向通知：处理中→维修工；待受理→受理名单；待派单→派单名单。
func (s *OrderService) notifyAfterCreate(o *model.WorkOrder) {
	switch o.Status {
	case model.OrderProcessing:
		if o.AssigneeID != nil {
			s.Notify(*o.AssigneeID, "workorder", "新工单指派", fmt.Sprintf("工单 %s「%s」已指派给您，请及时处理", o.OrderNo, o.Title), &o.ID)
		}
	case model.OrderReported:
		s.notifySlot(o.CommunityID, sysmodel.SlotOrderTriage, "新工单待受理", fmt.Sprintf("工单 %s「%s」已上报，请受理", o.OrderNo, o.Title), &o.ID)
	case model.OrderPendingDispatch:
		s.notifySlot(o.CommunityID, sysmodel.SlotOrderDispatch, "新工单待派单", fmt.Sprintf("工单 %s「%s」待派单，请安排维修工", o.OrderNo, o.Title), &o.ID)
	}
}

// Report 移动端主动上报（source=active）；上报人须为该项目在职编制成员。
func (s *OrderService) Report(ctx context.Context, userID string, req *dto.OrderReportReq) (gin.H, *errs.Error) {
	var staffCount int64
	s.db.Model(&sysmodel.ProjectStaff{}).
		Where("project_id = ? AND user_id = ? AND status = ?", req.CommunityID, userID, sysmodel.StatusEnabled).Count(&staffCount)
	if staffCount == 0 {
		return nil, errs.ErrDataScope
	}
	priority, be := validPriority(req.Priority)
	if be != nil {
		return nil, be
	}
	if be := s.checkPointBelongs(req.CommunityID, req.PointID); be != nil {
		return nil, be
	}
	photos, be := s.resolvePhotos(req.Photos)
	if be != nil {
		return nil, be
	}
	orderNo, err := s.GenOrderNo(ctx)
	if err != nil {
		return nil, errs.ErrInternal
	}
	triageEnabled, _ := s.communityFlags(req.CommunityID)
	status := model.OrderReported
	if !triageEnabled {
		status = model.OrderPendingDispatch
	}
	order := model.WorkOrder{
		TenantID: middleware.CommunityTenantID(s.db, req.CommunityID), // 冗余列（=所属小区租户）
		OrderNo: orderNo, CommunityID: req.CommunityID, PointID: req.PointID,
		Title: req.Title, Description: req.Description, Photos: photos,
		Source: model.SourceActive, ReporterID: userID, Priority: priority, Status: status,
		Items: types.OrderItemArray{},
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return tx.Create(&model.WorkOrderLog{OrderID: order.ID, Action: model.ActionCreate, OperatorID: userID, Detail: "移动端主动上报"}).Error
	})
	if err != nil {
		return nil, errs.ErrInternal
	}
	s.notifyAfterCreate(&order)
	return gin.H{"id": order.ID, "order_no": orderNo, "status": status}, nil
}

// Detail 工单详情（含流转时间线）。
func (s *OrderService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := middleware.CheckCommunity(s.db, c, o.CommunityID); be != nil {
		return nil, be
	}
	return s.detailOf(&o), nil
}

// detailOf 组装详情（管理后台与移动端共用）。
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
		"items": s.itemViews(o.Items),
		"source": o.Source, "category": o.Category,
		"reporter_id": o.ReporterID, "reporter_name": s.userName(o.ReporterID),
		"assignee_id": o.AssigneeID, "assignee_name": s.userNamePtr(o.AssigneeID),
		"dispatcher_id": o.DispatcherID, "dispatcher_name": s.userNamePtr(o.DispatcherID),
		"priority": o.Priority, "status": o.Status,
		"triage_by": o.TriageBy, "triage_by_name": s.userNamePtr(o.TriageBy),
		"triage_at": timefmt.TP(o.TriageAt), "triage_note": nullableStr(o.TriageNote),
		"dispatch_at": timefmt.TP(o.DispatchAt), "accept_at": timefmt.TP(o.AcceptAt),
		"finish_photos": o.FinishPhotos, "finish_note": nullableStr(o.FinishNote), "finish_at": timefmt.TP(o.FinishAt),
		"confirm_by": o.ConfirmBy, "confirm_by_name": s.userNamePtr(o.ConfirmBy),
		"confirm_at": timefmt.TP(o.ConfirmAt), "confirm_note": nullableStr(o.ConfirmNote),
		"reject_reason": nullableStr(o.RejectReason),
		// SLA 简化实现：按优先级硬编码期望完成时长，仅展示不推送（见 statemachine.go）
		"sla_deadline": timefmt.TP(SLADeadline(o.Priority, o.CreatedAt)),
		"sla_overdue":  SLAOverdue(o.Status, o.Priority, o.CreatedAt, time.Now()),
		"created_at": timefmt.T(o.CreatedAt), "logs": logItems,
	}
}

// itemViews 不合格项快照视图：file_key 转可访问 URL（整改前后对比）。
func (s *OrderService) itemViews(items types.OrderItemArray) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"name": it.Name, "remark": it.Remark,
			"before_photo_urls": s.photoURLs(it.BeforePhotos),
			"after_photo_urls":  s.photoURLs(it.AfterPhotos),
		})
	}
	return out
}

// photoURLs file_key 数组转可访问 URL 数组。
func (s *OrderService) photoURLs(keys types.StringArray) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.store.URL(k))
	}
	return out
}

// Update 修改工单（仅待受理/待派单可改）。
func (s *OrderService) Update(c *gin.Context, id string, req *dto.OrderUpdateReq) *errs.Error {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return be
	}
	if o.Status != model.OrderReported && o.Status != model.OrderPendingDispatch {
		return errs.ErrOrderStatusNotAllowed.WithMsg("仅待受理/待派单状态可修改")
	}
	updates := map[string]any{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		priority, be := validPriority(*req.Priority)
		if be != nil {
			return be
		}
		updates["priority"] = priority
	}
	if err := s.db.Model(o).Updates(updates).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Delete 软删除（仅待受理/待派单可删）。
func (s *OrderService) Delete(c *gin.Context, id string) *errs.Error {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return be
	}
	if o.Status != model.OrderReported && o.Status != model.OrderPendingDispatch {
		return errs.ErrOrderStatusNotAllowed.WithMsg("仅待受理/待派单状态可删除")
	}
	if err := s.db.Delete(o).Error; err != nil {
		return errs.ErrInternal
	}
	return nil
}

// Triage 受理（order_triage 槽位成员；reported → pending_dispatch / closed_invalid）。
func (s *OrderService) Triage(c *gin.Context, id string, req *dto.TriageReq) (string, *errs.Error) {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return "", be
	}
	identity := middleware.CurrentIdentity(c)
	if !s.inSlot(o.CommunityID, sysmodel.SlotOrderTriage, identity) {
		return "", errs.ErrOrderNotInSlot.WithMsg("当前用户不在本项目工单受理名单内")
	}
	action := model.ActionTriagePass
	if req.Result == "reject" {
		action = model.ActionTriageReject
		if strings.TrimSpace(req.Note) == "" {
			return "", errs.ErrTriageNoteRequired
		}
	}
	newStatus, ok := CanTransit(action, o.Status)
	if !ok {
		return "", errs.ErrOrderStatusNotAllowed.WithMsg("仅待受理状态可执行受理")
	}
	updates := map[string]any{
		"status": newStatus, "triage_by": identity.UserID, "triage_at": time.Now(), "triage_note": req.Note,
	}
	if req.Result == "reject" {
		updates["reject_reason"] = req.Note
	} else {
		if req.Priority != "" {
			priority, be := validPriority(req.Priority)
			if be != nil {
				return "", be
			}
			updates["priority"] = priority
		}
		if req.Category != "" {
			updates["category"] = req.Category
		}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(updates).Error; err != nil {
			return err
		}
		detail := "受理通过，进入待派单"
		if req.Result == "reject" {
			detail = "受理驳回（作废）：" + req.Note
		} else if req.Note != "" {
			detail += "（" + req.Note + "）"
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: action, OperatorID: identity.UserID, Detail: detail}).Error
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	if req.Result == "reject" {
		s.Notify(o.ReporterID, "workorder", "工单受理驳回", fmt.Sprintf("工单 %s「%s」受理驳回：%s", o.OrderNo, o.Title, req.Note), &o.ID)
	} else {
		s.notifySlot(o.CommunityID, sysmodel.SlotOrderDispatch, "新工单待派单", fmt.Sprintf("工单 %s「%s」待派单，请安排维修工", o.OrderNo, o.Title), &o.ID)
	}
	return newStatus, nil
}

// Dispatch 派单（order_dispatch 槽位成员；pending_dispatch → processing，派单即视为接单）。
func (s *OrderService) Dispatch(c *gin.Context, id string, req *dto.DispatchReq) *errs.Error {
	o, be := s.getWithScope(c, id)
	if be != nil {
		return be
	}
	identity := middleware.CurrentIdentity(c)
	if !s.inSlot(o.CommunityID, sysmodel.SlotOrderDispatch, identity) {
		return errs.ErrOrderNotInSlot.WithMsg("当前用户不在本项目工单派单名单内")
	}
	if _, ok := CanTransit(model.ActionDispatch, o.Status); !ok {
		return errs.ErrOrderStatusNotAllowed.WithMsg("仅待派单状态可派单")
	}
	if be := s.checkAcceptMember(o.CommunityID, req.AssigneeID); be != nil {
		return be
	}
	now := time.Now()
	assigneeName := s.userName(req.AssigneeID)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(map[string]any{
			"assignee_id": req.AssigneeID, "dispatcher_id": identity.UserID,
			"dispatch_at": now, "accept_at": now, "status": model.OrderProcessing,
		}).Error; err != nil {
			return err
		}
		detail := "派单给 " + assigneeName
		if req.Remark != "" {
			detail += "（" + req.Remark + "）"
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: model.ActionDispatch, OperatorID: identity.UserID, Detail: detail}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	s.Notify(req.AssigneeID, "workorder", "新工单指派", fmt.Sprintf("工单 %s「%s」已指派给您，请及时处理", o.OrderNo, o.Title), &o.ID)
	return nil
}

// Grab 抢单（项目开启抢单模式时，order_accept 槽位成员从待派单池抢单；pending_dispatch → processing）。
// 并发安全：状态条件更新（WHERE status=pending_dispatch），影响行数为 0 即被他人抢先。
func (s *OrderService) Grab(id string, identity *middleware.Identity) *errs.Error {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return errs.ErrNotFound
	}
	_, grabEnabled := s.communityFlags(o.CommunityID)
	if !grabEnabled {
		return errs.ErrOrderGrabDisabled
	}
	if !s.inSlot(o.CommunityID, sysmodel.SlotOrderAccept, identity) {
		return errs.ErrOrderNotInSlot.WithMsg("当前用户不在本项目工单接单名单内")
	}
	userID := identity.UserID
	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.WorkOrder{}).Where("id = ? AND status = ?", o.ID, model.OrderPendingDispatch).
			Updates(map[string]any{
				"assignee_id": userID, "dispatcher_id": nil,
				"dispatch_at": now, "accept_at": now, "status": model.OrderProcessing,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.ErrOrderStatusNotAllowed.WithMsg("工单已被抢或不在待派单状态")
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: model.ActionGrab, OperatorID: userID, Detail: "从工单池抢单"}).Error
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return be
		}
		return errs.ErrInternal
	}
	s.Notify(userID, "workorder", "抢单成功", fmt.Sprintf("工单 %s「%s」抢单成功，请及时处理", o.OrderNo, o.Title), &o.ID)
	return nil
}

// Finish 完工提交（processing → pending_confirm，附完工照片；后台代录与移动端共用）。
func (s *OrderService) Finish(operatorID string, o *model.WorkOrder, req *dto.FinishReq) *errs.Error {
	if _, ok := CanTransit(model.ActionFinish, o.Status); !ok {
		return errs.ErrOrderStatusNotAllowed.WithMsg("当前状态不可提交完工")
	}
	photos, be := s.resolvePhotos(req.FixPhotos)
	if be != nil {
		return be
	}
	items, be := s.mergeFinishItems(o, req.AfterPhotos)
	if be != nil {
		return be
	}
	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(map[string]any{
			"status": model.OrderPendingConfirm, "finish_note": req.FixRemark,
			"finish_photos": photos, "items": items, "finish_at": now,
			"reject_reason": "", // 新一轮完工提交后清空上次退回原因（历史见流转日志）
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: model.ActionFinish, OperatorID: operatorID, Detail: "完工提交：" + req.FixRemark}).Error
	})
	if err != nil {
		return errs.ErrInternal
	}
	// 通知验收人：报单人 + 受理名单（验收授权为两者，去重）
	notified := map[string]bool{}
	s.Notify(o.ReporterID, "workorder", "工单待验收", fmt.Sprintf("工单 %s「%s」已完工，请验收", o.OrderNo, o.Title), &o.ID)
	notified[o.ReporterID] = true
	for _, uid := range communitysvc.SlotUserIDs(s.db, o.CommunityID, sysmodel.SlotOrderTriage) {
		if !notified[uid] {
			s.Notify(uid, "workorder", "工单待验收", fmt.Sprintf("工单 %s「%s」已完工，请验收", o.OrderNo, o.Title), &o.ID)
		}
	}
	return nil
}

// Confirm 验收（报单人本人或 order_triage 槽位成员；pending_confirm → closed / 退回 processing）。
func (s *OrderService) Confirm(identity *middleware.Identity, o *model.WorkOrder, result, note string) (string, *errs.Error) {
	operatorID := identity.UserID
	if o.ReporterID != operatorID && !s.inSlot(o.CommunityID, sysmodel.SlotOrderTriage, identity) {
		return "", errs.ErrOrderNotInSlot.WithMsg("仅报单人或工单受理名单成员可验收")
	}
	action := model.ActionConfirmPass
	if result == "reject" {
		action = model.ActionConfirmReject
		if strings.TrimSpace(note) == "" {
			return "", errs.ErrConfirmNoteRequired
		}
	}
	newStatus, ok := CanTransit(action, o.Status)
	if !ok {
		return "", errs.ErrOrderStatusNotAllowed.WithMsg("仅待验收状态可执行验收")
	}
	updates := map[string]any{
		"status": newStatus, "confirm_by": operatorID, "confirm_at": time.Now(), "confirm_note": note,
	}
	if result == "reject" {
		updates["reject_reason"] = note
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(o).Updates(updates).Error; err != nil {
			return err
		}
		detail := "验收通过，工单闭环"
		if result == "reject" {
			detail = "验收不通过，退回返工：" + note
		}
		return tx.Create(&model.WorkOrderLog{OrderID: o.ID, Action: action, OperatorID: operatorID, Detail: detail}).Error
	})
	if err != nil {
		return "", errs.ErrInternal
	}
	if result == "reject" && o.AssigneeID != nil {
		s.Notify(*o.AssigneeID, "workorder", "工单验收退回", fmt.Sprintf("工单 %s 验收不通过：%s，请重新处理", o.OrderNo, note), &o.ID)
	}
	return newStatus, nil
}

// resolveCreateItems 手工建单不合格项入参校验与转换（before_photos file_key 须已上传）。
func (s *OrderService) resolveCreateItems(reqs []dto.OrderItemReq) (types.OrderItemArray, *errs.Error) {
	items := make(types.OrderItemArray, 0, len(reqs))
	for _, it := range reqs {
		if strings.TrimSpace(it.Name) == "" {
			return nil, errs.ErrParam.WithMsg("不合格项名称不能为空")
		}
		for _, key := range it.BeforePhotos {
			if !s.fileExists(key) {
				return nil, errs.ErrPhotoNotUploaded
			}
		}
		items = append(items, types.OrderItem{
			Name: strings.TrimSpace(it.Name), Remark: it.Remark,
			BeforePhotos: types.StringArray(it.BeforePhotos),
		})
	}
	return items, nil
}

// fileExists file_key 是否已上传确认。
func (s *OrderService) fileExists(key string) bool {
	var count int64
	s.db.Model(&sysmodel.UploadFile{}).Where("file_key = ?", key).Count(&count)
	return count > 0
}

// mergeFinishItems 整改回传逐项补图：after_photos（检查项名 → file_key 数组）按 name 合并进
// items 快照；未知项追加为新条目。map 遍历前先排序键，保证日志与存储顺序稳定。
func (s *OrderService) mergeFinishItems(o *model.WorkOrder, afterPhotos map[string][]string) (types.OrderItemArray, *errs.Error) {
	items := o.Items
	if items == nil {
		items = types.OrderItemArray{}
	}
	names := make([]string, 0, len(afterPhotos))
	for name := range afterPhotos {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		rawKeys := afterPhotos[name]
		if name == "" || len(rawKeys) == 0 {
			return nil, errs.ErrParam.WithMsg("整改后照片须按检查项名提供")
		}
		keys := make(types.StringArray, 0, len(rawKeys))
		for _, k := range rawKeys {
			if !s.fileExists(k) {
				return nil, errs.ErrPhotoNotUploaded
			}
			keys = append(keys, k)
		}
		merged := false
		for i := range items {
			if items[i].Name == name {
				items[i].AfterPhotos = append(items[i].AfterPhotos, keys...)
				merged = true
				break
			}
		}
		if !merged {
			items = append(items, types.OrderItem{Name: name, AfterPhotos: keys})
		}
	}
	return items, nil
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

// ========== 移动端（app/mp 共用） ==========

// GetForMP 移动端取工单（上报人/处理人/受理/派单名单成员可见；待派单且项目开启抢单时接单名单成员可见）。
func (s *OrderService) GetForMP(id string, identity *middleware.Identity) (gin.H, *errs.Error) {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if be := s.checkMPVisible(&o, identity); be != nil {
		return nil, be
	}
	return s.detailOf(&o), nil
}

// checkMPVisible 移动端详情可见性判定。
func (s *OrderService) checkMPVisible(o *model.WorkOrder, identity *middleware.Identity) *errs.Error {
	userID := identity.UserID
	if o.ReporterID == userID || (o.AssigneeID != nil && *o.AssigneeID == userID) {
		return nil
	}
	if s.inSlot(o.CommunityID, sysmodel.SlotOrderTriage, identity) ||
		s.inSlot(o.CommunityID, sysmodel.SlotOrderDispatch, identity) {
		return nil
	}
	if o.Status == model.OrderPendingDispatch {
		if _, grab := s.communityFlags(o.CommunityID); grab && s.inSlot(o.CommunityID, sysmodel.SlotOrderAccept, identity) {
			return nil
		}
	}
	return errs.ErrNoPerm
}

// FinishForMP 移动端完工提交（仅当前维修工本人）。
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

// ConfirmForMP 移动端验收（我上报的工单；报单人本人或受理名单成员，service 内判定）。
func (s *OrderService) ConfirmForMP(id string, identity *middleware.Identity, req *dto.ConfirmReq) (string, *errs.Error) {
	var o model.WorkOrder
	if err := s.db.First(&o, "id = ?", id).Error; err != nil {
		return "", errs.ErrNotFound
	}
	return s.Confirm(identity, &o, req.Result, req.ConfirmNote)
}

// PoolOrders 可抢工单池：用户在职编制中「开启抢单且本人在 order_accept 名单」项目的待派单工单。
func (s *OrderService) PoolOrders(userID string, q *dto.OrderListQuery) (*response.Page, *errs.Error) {
	communityIDs := s.grabbableCommunityIDs(userID)
	if len(communityIDs) == 0 {
		return &response.Page{List: []gin.H{}, Total: 0, Page: q.Page, PageSize: q.PageSize}, nil
	}
	db := s.db.Model(&model.WorkOrder{}).
		Where("status = ? AND community_id IN ?", model.OrderPendingDispatch, communityIDs)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var rows []model.WorkOrder
	offset, limit := q.Normalize()
	if err := db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, errs.ErrInternal
	}
	list := make([]gin.H, 0, len(rows))
	for i := range rows {
		list = append(list, s.toItem(&rows[i]))
	}
	return &response.Page{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// PoolCount 可抢池数量（与 PoolOrders 同口径，供移动端角标）。
func (s *OrderService) PoolCount(userID string) int64 {
	communityIDs := s.grabbableCommunityIDs(userID)
	if len(communityIDs) == 0 {
		return 0
	}
	var cnt int64
	s.db.Model(&model.WorkOrder{}).
		Where("status = ? AND community_id IN ?", model.OrderPendingDispatch, communityIDs).Count(&cnt)
	return cnt
}

// grabbableCommunityIDs 用户可抢单的项目集合：在职编制项目 ∩ 项目开启抢单 ∩ 本人在 order_accept 名单。
func (s *OrderService) grabbableCommunityIDs(userID string) []string {
	projectIDs, err := middleware.StaffProjectIDs(s.db, userID)
	if err != nil || len(projectIDs) == 0 {
		return nil
	}
	var comms []sysmodel.Community
	s.db.Select("id").Where("id IN ? AND wo_grab_enabled", projectIDs).Find(&comms)
	out := make([]string, 0, len(comms))
	for _, cm := range comms {
		// 抢单池是"我的工作台"过滤，按本人名单成员身份判定（管理员绕过不适用于此）
		for _, uid := range communitysvc.SlotUserIDs(s.db, cm.ID, sysmodel.SlotOrderAccept) {
			if uid == userID {
				out = append(out, cm.ID)
				break
			}
		}
	}
	return out
}

// CreateFromCheckin 异常打卡自动生成工单（在打卡事务内调用）；items 为不合格项快照（before_photos=打卡时该项照片）。
// 巡检异常转单视同已受理（source=inspection，直接进待派单）。
// 同事务内定向通知：该巡查业务线的汇报线成员（异常归口对应线主管，patrolType 路由）+ 工单派单槽位成员（下一步处理人）。
func CreateFromCheckin(tx *gorm.DB, orderNo string, checkinID, communityID string, pointID *string, title, description string, photos types.PhotoArray, items types.OrderItemArray, reporterID, patrolType string) (*model.WorkOrder, error) {
	order := model.WorkOrder{
		TenantID: middleware.CommunityTenantID(tx, communityID), // 冗余列（=所属小区租户）
		OrderNo: orderNo, CheckinID: &checkinID, CommunityID: communityID, PointID: pointID,
		Title: title, Description: description, Photos: photos, Items: items,
		Source: model.SourceInspection, ReporterID: reporterID, Priority: "normal", Status: model.OrderPendingDispatch,
	}
	if err := tx.Create(&order).Error; err != nil {
		return nil, err
	}
	log := model.WorkOrderLog{OrderID: order.ID, Action: model.ActionCreate, OperatorID: reporterID, Detail: "巡检异常上报，自动生成工单（视同已受理）"}
	if err := tx.Create(&log).Error; err != nil {
		return nil, err
	}
	reporterName := ""
	var u sysmodel.SysUser
	if tx.Select("name").First(&u, "id = ?", reporterID).Error == nil {
		reporterName = u.Name
	}
	notify := func(uid, msgTitle, content string) { // 通知写入失败不阻断打卡事务
		_ = tx.Create(&sysmodel.SysMessage{UserID: uid, Type: "workorder", Title: msgTitle, Content: content, BizID: &order.ID}).Error
	}
	reportLineSlot := communitysvc.ResolveReportLineSlot(tx, communityID, patrolType)
	for _, uid := range communitysvc.SlotUserIDs(tx, communityID, reportLineSlot) {
		notify(uid, "异常打卡待处理", fmt.Sprintf("巡检员%s打卡异常：%s，已自动转工单 %s，请跟进", reporterName, title, orderNo))
	}
	for _, uid := range communitysvc.SlotUserIDs(tx, communityID, sysmodel.SlotOrderDispatch) {
		notify(uid, "新工单待派单", fmt.Sprintf("工单 %s「%s」待派单，请安排维修工", orderNo, title))
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
	if be := middleware.CheckCommunity(s.db, c, o.CommunityID); be != nil {
		return nil, be
	}
	return &o, nil
}

// checkAcceptMember 被指派人须为本项目 order_accept 槽位名单成员（校验的是"被指派人的任职资格"，不享受操作者绕过）。
func (s *OrderService) checkAcceptMember(communityID, userID string) *errs.Error {
	for _, uid := range communitysvc.SlotUserIDs(s.db, communityID, sysmodel.SlotOrderAccept) {
		if uid == userID {
			return nil
		}
	}
	return errs.ErrAssigneeInvalid.WithMsg("被指派人须为本项目工单接单岗位成员")
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

// checkPointBelongs 校验关联点位属于该小区且启用（nil/空 放行）。
func (s *OrderService) checkPointBelongs(communityID string, pointID *string) *errs.Error {
	if pointID == nil || *pointID == "" {
		return nil
	}
	var count int64
	s.db.Table("inspection_point").
		Where("id = ? AND community_id = ? AND status = ?", *pointID, communityID, sysmodel.StatusEnabled).
		Count(&count)
	if count == 0 {
		return errs.ErrParam.WithMsg("关联点位须属于该项目且为启用状态")
	}
	return nil
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
