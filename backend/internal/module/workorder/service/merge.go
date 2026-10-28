package service

import (
	"sort"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/errs"
)

type MergeReq struct {
	OrderIDs    []string `json:"order_ids" binding:"required,min=2"`
	OperationID string   `json:"operation_id" binding:"required"`
}

func (s *WorkOrderService) Merge(c *gin.Context, req MergeReq) (gin.H, *errs.Error) {
	identity := identityFrom(c)
	if identity == nil {
		return nil, errs.ErrUnauthorized
	}
	ids := append([]string(nil), req.OrderIDs...)
	sort.Strings(ids)
	var kept model.WorkOrder
	merged := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		orders := make([]model.WorkOrder, 0, len(ids))
		for _, id := range ids {
			var order model.WorkOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ? AND status = ?", id, model.StatusPending).Error; err != nil {
				return errs.ErrConflict.WithMsg("只能合并未接单工单")
			}
			if be := s.assertOrderScope(tx, c, &order); be != nil {
				return be
			}
			orders = append(orders, order)
		}
		kept = orders[0]
		for _, order := range orders[1:] {
			if order.TenantID != kept.TenantID || order.CommunityID != kept.CommunityID || order.WorkType != kept.WorkType {
				return errs.ErrParam.WithMsg("只能合并同小区、同专业的工单")
			}
			if err := tx.Model(&model.WorkOrderIssue{}).Where("order_id = ?", order.ID).Update("order_id", kept.ID).Error; err != nil {
				return err
			}
			from := order.Status
			order.Status = model.StatusClosed
			order.CloseReason = "合并至 " + kept.OrderNo
			order.Revision++
			if err := tx.Model(&order).Updates(map[string]any{"status": order.Status, "close_reason": order.CloseReason, "revision": order.Revision, "closed_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
				return err
			}
			if err := s.addEvent(tx, &order, nil, "merged", identity.UserID, from, model.StatusClosed, req.OperationID); err != nil {
				return err
			}
			merged++
		}
		kept.Revision++
		return tx.Model(&kept).Update("revision", kept.Revision).Error
	})
	if err != nil {
		if be, ok := err.(*errs.Error); ok {
			return nil, be
		}
		return nil, errs.ErrInternal
	}
	return gin.H{"id": kept.ID, "merged": merged, "status": kept.Status}, nil
}

func identityFrom(c *gin.Context) *middleware.Identity { return middleware.CurrentIdentity(c) }
