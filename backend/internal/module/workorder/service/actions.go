package service

import (
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
	"anxuncloud/internal/pkg/types"
)

type AssignReq struct {
	AssigneeID  string `json:"assignee_id" binding:"required"`
	Reason      string `json:"reason"`
	OperationID string `json:"operation_id"`
}

type FinishReq struct {
	IssueID     string   `json:"issue_id" binding:"required"`
	Note        string   `json:"note" binding:"required"`
	Photos      []string `json:"photos" binding:"required,min=1"`
	OperationID string   `json:"operation_id"`
}

type ReviewReq struct {
	IssueID     string `json:"issue_id" binding:"required"`
	Action      string `json:"action" binding:"required,oneof=approve reject"`
	Reason      string `json:"reason"`
	OperationID string `json:"operation_id"`
}

type CloseReq struct {
	Reason      string `json:"reason" binding:"required"`
	OperationID string `json:"operation_id"`
}

func (s *WorkOrderService) Claim(c *gin.Context, id string) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	return s.transaction(func(tx *gorm.DB) error {
		var order model.WorkOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND status = ?", id, model.StatusPending).Error; err != nil {
			return errs.ErrConflict.WithMsg("工单已被接单或不存在")
		}
		if be := s.assertOrderScope(tx, c, &order); be != nil {
			return be
		}
		if !canClaim(identity, order.WorkType) {
			return errs.ErrNoPerm.WithMsg("当前角色不能抢该类型工单")
		}
		order.Status = model.StatusInProgress
		order.AssigneeID = &identity.UserID
		order.Revision++
		if err := tx.Model(&order).Updates(map[string]any{"status": order.Status, "assignee_id": order.AssigneeID, "revision": order.Revision}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ? AND status = ?", id, model.StatusPending).Update("status", model.StatusInProgress).Error; err != nil {
			return err
		}
		var staffCount int64
		if err := tx.Model(&sysmodel.ProjectStaff{}).Where("project_id = ? AND user_id = ? AND status = ?", order.CommunityID, identity.UserID, sysmodel.StatusEnabled).Count(&staffCount).Error; err != nil {
			return err
		}
		if staffCount == 0 {
			expires := time.Now().Add(48 * time.Hour)
			if order.Deadline != nil {
				expires = *order.Deadline
			}
			if err := tx.Create(&sysmodel.ProjectStaff{TenantID: &order.TenantID, ProjectID: order.CommunityID,
				UserID: identity.UserID, BuildingIDs: types.IDArray{}, Status: sysmodel.StatusEnabled,
				EffectiveTo: &expires, TemporaryOrderID: &order.ID}).Error; err != nil {
				return err
			}
		}
		return s.addEvent(tx, &order, nil, "claimed", identity.UserID, model.StatusPending, model.StatusInProgress, "claim:"+id)
	})
}

func (s *WorkOrderService) Assign(c *gin.Context, id string, req AssignReq) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	return s.transaction(func(tx *gorm.DB) error {
		var order model.WorkOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND status IN ?", id, []string{model.StatusPending, model.StatusInProgress}).Error; err != nil {
			return errs.ErrConflict.WithMsg("当前状态不能分派")
		}
		if be := s.assertOrderScope(tx, c, &order); be != nil {
			return be
		}
		var user sysmodel.SysUser
		if err := tx.First(&user, "id = ? AND tenant_id = ? AND status = ?", req.AssigneeID, order.TenantID, sysmodel.StatusEnabled).Error; err != nil || user.RoleID == nil {
			return errs.ErrParam.WithMsg("执行人须为本租户启用账号")
		}
		var role sysmodel.SysRole
		if err := tx.First(&role, "id = ?", *user.RoleID).Error; err != nil {
			return errs.ErrInternal
		}
		assigneeIdentity := &middleware.Identity{UserID: user.ID, ClaimWorkTypes: role.ClaimWorkTypes}
		if !canClaim(assigneeIdentity, order.WorkType) {
			return errs.ErrParam.WithMsg("执行人角色不符合工单类型")
		}
		projectIDs, err := middleware.StaffProjectIDs(tx, user.ID)
		if err != nil {
			return err
		}
		if !types.IDArray(projectIDs).Contains(order.CommunityID) {
			return errs.ErrParam.WithMsg("执行人没有该小区的有效任职")
		}
		from := order.Status
		order.Status = model.StatusInProgress
		order.AssigneeID = &req.AssigneeID
		order.Revision++
		if err := tx.Model(&order).Updates(map[string]any{"status": order.Status, "assignee_id": order.AssigneeID, "revision": order.Revision}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ? AND status = ?", id, model.StatusPending).Update("status", model.StatusInProgress).Error; err != nil {
			return err
		}
		return s.addEvent(tx, &order, nil, "assigned", identity.UserID, from, model.StatusInProgress, req.OperationID)
	})
}

func (s *WorkOrderService) Finish(c *gin.Context, id string, req FinishReq) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil || strings.TrimSpace(req.Note) == "" {
		return errs.ErrParam.WithMsg("整改说明和整改照片不能为空")
	}
	return s.transaction(func(tx *gorm.DB) error {
		var order model.WorkOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND status = ?", id, model.StatusInProgress).Error; err != nil {
			return errs.ErrConflict.WithMsg("工单当前不能提交整改")
		}
		if order.AssigneeID == nil || *order.AssigneeID != identity.UserID {
			return errs.ErrNoPerm.WithMsg("只能提交本人负责工单的整改")
		}
		var issue model.WorkOrderIssue
		if err := tx.First(&issue, "id = ? AND order_id = ? AND status = ?", req.IssueID, id, model.StatusInProgress).Error; err != nil {
			return errs.ErrNotFound
		}
		var round int
		if err := tx.Model(&model.WorkOrderFixAttempt{}).Select("COALESCE(MAX(round), 0)").Where("issue_id = ?", issue.ID).Scan(&round).Error; err != nil {
			return err
		}
		attempt := model.WorkOrderFixAttempt{OrderID: id, IssueID: issue.ID, Round: round + 1,
			OperatorID: identity.UserID, Note: strings.TrimSpace(req.Note), Photos: req.Photos, SubmittedAt: time.Now()}
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		if err := filesvc.LinkObject(tx, order.TenantID, order.CommunityID, "work_order_fix_attempt", attempt.ID, req.Photos); err != nil {
			return err
		}
		issue.Status = model.StatusPendingReview
		if err := tx.Model(&issue).Update("status", issue.Status).Error; err != nil {
			return err
		}
		from := order.Status
		order.Status = model.StatusPendingReview
		var inProgress int64
		if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ? AND status IN ?", id, []string{model.StatusPending, model.StatusInProgress}).Count(&inProgress).Error; err != nil {
			return err
		}
		if inProgress > 0 {
			order.Status = model.StatusInProgress
		}
		order.Revision++
		if err := tx.Model(&order).Updates(map[string]any{"status": order.Status, "revision": order.Revision}).Error; err != nil {
			return err
		}
		return s.addEvent(tx, &order, &issue.ID, "fix_submitted", identity.UserID, from, order.Status, req.OperationID)
	})
}

func (s *WorkOrderService) Review(c *gin.Context, id string, req ReviewReq) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	if req.Action == "reject" && strings.TrimSpace(req.Reason) == "" {
		return errs.ErrParam.WithMsg("驳回整改必须填写原因")
	}
	return s.transaction(func(tx *gorm.DB) error {
		var order model.WorkOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND status = ?", id, model.StatusPendingReview).Error; err != nil {
			return errs.ErrConflict.WithMsg("工单当前不在待审核状态")
		}
		if be := s.assertOrderScope(tx, c, &order); be != nil {
			return be
		}
		if order.ReviewStep >= len(order.ReviewFlow) {
			return errs.ErrConflict.WithMsg("工单审核链未配置，已挂起")
		}
		step := order.ReviewFlow[order.ReviewStep]
		if step.Kind == sysmodel.FlowStepKindAI || !communitysvc.RoleCandidateAuthorized(tx, []string{order.CommunityID}, step.RoleCode, identity) {
			return errs.ErrNotInSlot.WithMsg("当前用户不是工单审核节点候选人")
		}
		if order.AssigneeID != nil && communitysvc.SameNaturalPerson(tx, identity.UserID, *order.AssigneeID) {
			return errs.ErrNoPerm.WithMsg("不能审核本人提交的整改")
		}
		var issue model.WorkOrderIssue
		if err := tx.First(&issue, "id = ? AND order_id = ? AND status = ?", req.IssueID, id, model.StatusPendingReview).Error; err != nil {
			return errs.ErrNotFound
		}
		from := order.Status
		if req.Action == "reject" {
			issue.Status = model.StatusInProgress
			order.Status = model.StatusInProgress
		} else {
			issue.Status = model.StatusDone
			if err := s.completeMaintenanceIssue(tx, &order, &issue, identity.UserID); err != nil {
				return err
			}
			if order.ReviewStep+1 < len(order.ReviewFlow) {
				order.ReviewStep++
				order.Status = model.StatusPendingReview
			} else {
				order.Status = model.StatusDone
				var open int64
				if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ? AND id <> ? AND status <> ?", id, issue.ID, model.StatusDone).Count(&open).Error; err != nil {
					return err
				}
				if open > 0 {
					var executing int64
					if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ? AND status IN ?", id, []string{model.StatusPending, model.StatusInProgress}).Count(&executing).Error; err != nil {
						return err
					}
					order.Status = model.StatusPendingReview
					if executing > 0 {
						order.Status = model.StatusInProgress
					}
				} else {
					now := time.Now()
					order.CompletedAt = &now
					if err := tx.Where("temporary_order_id = ?", id).Delete(&sysmodel.ProjectStaff{}).Error; err != nil {
						return err
					}
				}
			}
		}
		if err := tx.Model(&issue).Update("status", issue.Status).Error; err != nil {
			return err
		}
		order.Revision++
		if err := tx.Model(&order).Updates(map[string]any{"status": order.Status, "review_step": order.ReviewStep,
			"completed_at": order.CompletedAt, "revision": order.Revision}).Error; err != nil {
			return err
		}
		return s.addEvent(tx, &order, &issue.ID, "review_"+req.Action, identity.UserID, from, order.Status, req.OperationID)
	})
}

func (s *WorkOrderService) Close(c *gin.Context, id string, req CloseReq) *errs.Error {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		return errs.ErrUnauthorized
	}
	return s.transaction(func(tx *gorm.DB) error {
		var order model.WorkOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND status NOT IN ?", id, []string{model.StatusDone, model.StatusClosed}).Error; err != nil {
			return errs.ErrConflict.WithMsg("已结束工单不能关闭")
		}
		if be := s.assertOrderScope(tx, c, &order); be != nil {
			return be
		}
		from := order.Status
		now := time.Now()
		order.Status = model.StatusClosed
		order.CloseReason = strings.TrimSpace(req.Reason)
		order.ClosedAt = &now
		order.Revision++
		if err := tx.Model(&order).Updates(map[string]any{"status": order.Status, "close_reason": order.CloseReason,
			"closed_at": now, "revision": order.Revision}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ?", id).Update("status", model.StatusClosed).Error; err != nil {
			return err
		}
		if err := tx.Where("temporary_order_id = ?", id).Delete(&sysmodel.ProjectStaff{}).Error; err != nil {
			return err
		}
		return s.addEvent(tx, &order, nil, "closed", identity.UserID, from, model.StatusClosed, req.OperationID)
	})
}

func (s *WorkOrderService) assertOrderScope(tx *gorm.DB, c *gin.Context, order *model.WorkOrder) *errs.Error {
	if be := middleware.CheckTenantRow(c, tx, &order.TenantID); be != nil {
		return be
	}
	identity := middleware.CurrentIdentity(c)
	if identity.DataScopeAll {
		return nil
	}
	if identity.ScopeSelf && (order.AssigneeID == nil || *order.AssigneeID != identity.UserID) {
		if order.Status != model.StatusPending || !canClaim(identity, order.WorkType) {
			return errs.ErrDataScope
		}
		return nil
	}
	if !types.IDArray(identity.ProjectIDs).Contains(order.CommunityID) {
		return errs.ErrDataScope
	}
	return nil
}

func (s *WorkOrderService) transaction(fn func(*gorm.DB) error) *errs.Error {
	err := s.db.Transaction(fn)
	if err == nil {
		return nil
	}
	if be, ok := err.(*errs.Error); ok {
		return be
	}
	return errs.ErrInternal
}

func (s *WorkOrderService) AssignCandidates(c *gin.Context, communityID, workType string) ([]gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	if workType != model.WorkTypeInspectionFix && workType != model.WorkTypeMaintenance {
		return nil, errs.ErrParam.WithMsg("工单类型无效")
	}
	var roles []struct {
		ID       string
		Name     string
		RoleName string
	}
	now := time.Now()
	if err := s.db.Model(&sysmodel.ProjectStaff{}).
		Select("sys_user.id", "sys_user.name", "sys_role.name AS role_name").
		Joins("JOIN sys_user ON sys_user.id = project_staff.user_id AND sys_user.deleted_at IS NULL AND sys_user.status = ?", sysmodel.StatusEnabled).
		Joins("JOIN sys_role ON sys_role.id = sys_user.role_id AND sys_role.deleted_at IS NULL AND sys_role.status = ?", sysmodel.StatusEnabled).
		Joins("JOIN community ON community.id = project_staff.project_id AND community.deleted_at IS NULL").
		Where("project_staff.project_id = ? AND project_staff.status = ?", communityID, sysmodel.StatusEnabled).
		Where("(project_staff.effective_from IS NULL OR project_staff.effective_from <= ?)", now).
		Where("(project_staff.effective_to IS NULL OR project_staff.effective_to > ?)", now).
		Where("sys_role.claim_work_types @> ?::jsonb", `["`+workType+`"]`).
		Order("sys_user.name").Scan(&roles).Error; err != nil {
		return nil, errs.ErrInternal
	}
	items := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		items = append(items, gin.H{"id": role.ID, "name": role.Name, "role_name": role.RoleName})
	}
	return items, nil
}
