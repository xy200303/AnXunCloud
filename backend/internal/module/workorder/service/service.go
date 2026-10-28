package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/middleware"
	communitysvc "anxuncloud/internal/module/community/service"
	filesvc "anxuncloud/internal/module/file/service"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
	"anxuncloud/internal/pkg/types"
	"anxuncloud/internal/pkg/uploadfile"
)

type WorkOrderService struct {
	db *gorm.DB
}

func NewWorkOrderService(db *gorm.DB) *WorkOrderService { return &WorkOrderService{db: db} }

func (s *WorkOrderService) CreateFromInspection(tx *gorm.DB, req CreateFromInspectionReq) error {
	if req.TenantID == "" || req.CommunityID == "" || req.RecordID == "" || req.OperatorID == "" {
		return fmt.Errorf("巡检异常入池缺少租户、对象或审核人")
	}
	flow := communitysvc.ResolveFlow(tx, req.CommunityID, sysmodel.FlowWorkorderReview)
	if len(flow) == 0 {
		return fmt.Errorf("工单审核链未配置")
	}
	now := time.Now()
	for _, item := range req.Items {
		var existing model.WorkOrderIssue
		if err := tx.Where("tenant_id = ? AND source_type = ? AND source_item_id = ?", req.TenantID, "inspection.checkin", item.ItemID).First(&existing).Error; err == nil {
			continue
		}
		deadline := now.Add(48 * time.Hour)
		order := model.WorkOrder{
			TenantID: req.TenantID, CommunityID: req.CommunityID,
			OrderNo:  fmt.Sprintf("WO%s%s", now.Format("20060102150405"), strings.ToUpper(strings.ReplaceAll(types.NewID(), "-", "")[16:])),
			WorkType: model.WorkTypeInspectionFix, Title: "巡检异常整改 · " + item.Title,
			Status: model.StatusPending, Priority: "high", Deadline: &deadline,
			ReviewFlow: flow, Revision: 1,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		issue := model.WorkOrderIssue{
			TenantID: req.TenantID, CommunityID: req.CommunityID, OrderID: order.ID,
			SourceType: "inspection.checkin", SourceRecordID: req.RecordID, SourceItemID: item.ItemID,
			SourceRevision: req.SourceRevision, Title: item.Title, Problem: item.Problem, Location: req.Location,
			EquipmentID: item.EquipmentID, Photos: item.Photos, Status: model.StatusPending,
		}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "source_type"}, {Name: "source_item_id"}},
			DoNothing: true,
		}).Create(&issue)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.Where("order_id = ?", order.ID).Delete(&model.WorkOrderEvent{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&order).Error; err != nil {
				return err
			}
			continue
		}
		if err := s.addEvent(tx, &order, nil, "created", req.OperatorID, "", model.StatusPending, req.RecordID); err != nil {
			return err
		}
		if err := filesvc.LinkObject(tx, issue.TenantID, issue.CommunityID, "work_order_issue", issue.ID, issue.Photos); err != nil {
			return err
		}
	}
	return nil
}

func (s *WorkOrderService) scoped(c *gin.Context) (*gorm.DB, *middleware.Identity, *errs.Error) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return nil, nil, errs.ErrUnauthorized
	}
	tenantID := identity.TenantID
	if identity.SuperAdmin {
		var be *errs.Error
		tenantID, be = middleware.ExplicitTenantID(c, s.db)
		if be != nil {
			return nil, nil, be
		}
		if tenantID == "" {
			tenantID = identity.TenantID
		}
	}
	if tenantID == "" {
		return nil, nil, errs.ErrDataScope
	}
	query := s.db.Model(&model.WorkOrder{}).Where("work_order.tenant_id = ?", tenantID)
	if identity.DataScopeAll {
		return query, identity, nil
	}
	if identity.ScopeSelf {
		return query.Where("work_order.assignee_id = ? OR (work_order.status = ? AND work_order.work_type IN ?)", identity.UserID, model.StatusPending, claimTypes(identity)), identity, nil
	}
	if len(identity.ProjectIDs) == 0 {
		return query.Where("1 = 0"), identity, nil
	}
	return query.Where("work_order.community_id IN ?", identity.ProjectIDs), identity, nil
}

func claimTypes(identity *middleware.Identity) []string {
	if len(identity.ClaimWorkTypes) == 0 {
		return []string{"__none__"}
	}
	return identity.ClaimWorkTypes
}

func canClaim(identity *middleware.Identity, workType string) bool {
	for _, allowed := range identity.ClaimWorkTypes {
		if allowed == workType {
			return true
		}
	}
	return false
}

type ListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Status      string `form:"status"`
	WorkType    string `form:"work_type"`
	Mine        string `form:"mine"`
}

func (s *WorkOrderService) List(c *gin.Context, q *ListQuery) (*response.Page, *errs.Error) {
	query, identity, be := s.scoped(c)
	if be != nil {
		return nil, be
	}
	if q.CommunityID != "" {
		query = query.Where("work_order.community_id = ?", q.CommunityID)
	}
	if q.Status != "" {
		query = query.Where("work_order.status = ?", q.Status)
	}
	if q.WorkType != "" {
		query = query.Where("work_order.work_type = ?", q.WorkType)
	}
	if q.Mine == "1" || q.Mine == "true" {
		query = query.Where("work_order.assignee_id = ?", identity.UserID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var orders []model.WorkOrder
	offset, limit := q.Normalize()
	if err := query.Order("work_order.created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		var issueCount int64
		s.db.Model(&model.WorkOrderIssue{}).Where("order_id = ?", order.ID).Count(&issueCount)
		items = append(items, gin.H{
			"id": order.ID, "order_no": order.OrderNo, "title": order.Title,
			"work_type": order.WorkType, "status": order.Status, "priority": order.Priority,
			"community_id": order.CommunityID, "community_name": communityName(s.db, order.CommunityID),
			"assignee_id": order.AssigneeID, "assignee_name": userName(s.db, order.AssigneeID),
			"deadline": order.Deadline, "revision": order.Revision, "issue_count": issueCount,
			"can_claim":  order.Status == model.StatusPending && canClaim(identity, order.WorkType),
			"can_finish": order.Status == model.StatusInProgress && order.AssigneeID != nil && *order.AssigneeID == identity.UserID,
			"created_at": order.CreatedAt,
		})
	}
	return &response.Page{List: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *WorkOrderService) Detail(c *gin.Context, id string) (gin.H, *errs.Error) {
	query, identity, be := s.scoped(c)
	if be != nil {
		return nil, be
	}
	var order model.WorkOrder
	if err := query.First(&order, "work_order.id = ?", id).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var issues []model.WorkOrderIssue
	if err := s.db.Where("order_id = ?", id).Order("created_at, id").Find(&issues).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var attempts []model.WorkOrderFixAttempt
	if err := s.db.Where("order_id = ?", id).Order("submitted_at, id").Find(&attempts).Error; err != nil {
		return nil, errs.ErrInternal
	}
	var events []model.WorkOrderEvent
	if err := s.db.Where("order_id = ?", id).Order("created_at, id").Find(&events).Error; err != nil {
		return nil, errs.ErrInternal
	}
	issueItems := make([]gin.H, 0, len(issues))
	for _, issue := range issues {
		issueItems = append(issueItems, gin.H{
			"id": issue.ID, "title": issue.Title, "problem": issue.Problem,
			"location": issue.Location, "equipment_id": issue.EquipmentID,
			"photos": photoViews(s.db, issue.Photos), "status": issue.Status,
			"source_type": issue.SourceType, "source_record_id": issue.SourceRecordID, "source_item_id": issue.SourceItemID,
		})
	}
	attemptItems := make([]gin.H, 0, len(attempts))
	for _, attempt := range attempts {
		attemptItems = append(attemptItems, gin.H{"id": attempt.ID, "issue_id": attempt.IssueID,
			"round": attempt.Round, "note": attempt.Note, "photos": photoViews(s.db, attempt.Photos), "submitted_at": attempt.SubmittedAt})
	}
	eventItems := make([]gin.H, 0, len(events))
	for _, event := range events {
		eventItems = append(eventItems, gin.H{"event_type": event.EventType, "issue_id": event.IssueID,
			"from_status": event.FromStatus, "to_status": event.ToStatus, "reason": event.Reason,
			"actor_id": event.ActorID, "actor_name": userName(s.db, &event.ActorID), "revision": event.Revision, "created_at": event.CreatedAt})
	}
	return gin.H{
		"id": order.ID, "order_no": order.OrderNo, "title": order.Title, "work_type": order.WorkType,
		"status": order.Status, "priority": order.Priority, "community_id": order.CommunityID,
		"community_name": communityName(s.db, order.CommunityID), "assignee_id": order.AssigneeID,
		"assignee_name": userName(s.db, order.AssigneeID), "deadline": order.Deadline,
		"review_flow": order.ReviewFlow, "review_step": order.ReviewStep, "revision": order.Revision,
		"close_reason": order.CloseReason, "completed_at": order.CompletedAt,
		"can_finish": order.Status == model.StatusInProgress && order.AssigneeID != nil && *order.AssigneeID == identity.UserID,
		"can_review": order.Status == model.StatusPendingReview && order.ReviewStep < len(order.ReviewFlow) &&
			communitysvc.RoleCandidateAuthorized(s.db, []string{order.CommunityID}, order.ReviewFlow[order.ReviewStep].RoleCode, identity) &&
			(order.AssigneeID == nil || !communitysvc.SameNaturalPerson(s.db, identity.UserID, *order.AssigneeID)),
		"issues": issueItems, "attempts": attemptItems, "events": eventItems,
	}, nil
}

func (s *WorkOrderService) addEvent(tx *gorm.DB, order *model.WorkOrder, issueID *string, eventType, actorID, fromStatus, toStatus, operationID string) error {
	if operationID == "" {
		operationID = types.NewID()
	}
	return tx.Create(&model.WorkOrderEvent{
		TenantID: order.TenantID, OrderID: order.ID, IssueID: issueID,
		EventType: eventType, OperationID: operationID, ActorID: actorID,
		FromStatus: fromStatus, ToStatus: toStatus, Revision: order.Revision,
	}).Error
}

func photoViews(db *gorm.DB, refs []string) []gin.H {
	files := uploadfile.ByIDs(db, refs)
	views := make([]gin.H, 0, len(refs))
	for _, ref := range refs {
		if file, ok := files[ref]; ok {
			views = append(views, gin.H{"file_id": ref, "url": file.URL, "watermarked_url": file.WatermarkedURL})
		}
	}
	return views
}

func communityName(db *gorm.DB, id string) string {
	var name string
	db.Model(&sysmodel.Community{}).Select("name").Where("id = ?", id).Limit(1).Pluck("name", &name)
	return name
}

func userName(db *gorm.DB, id *string) string {
	if id == nil || *id == "" {
		return ""
	}
	var name string
	db.Model(&sysmodel.SysUser{}).Select("name").Where("id = ?", *id).Limit(1).Pluck("name", &name)
	return name
}
