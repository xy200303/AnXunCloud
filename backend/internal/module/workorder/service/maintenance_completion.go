package service

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	eqmodel "anxuncloud/internal/module/equipment/model"
	eqsvc "anxuncloud/internal/module/equipment/service"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/errs"
)

func (s *WorkOrderService) completeMaintenanceIssue(tx *gorm.DB, order *model.WorkOrder, issue *model.WorkOrderIssue, reviewerID string) error {
	if order.WorkType != model.WorkTypeMaintenance {
		return nil
	}
	var item model.WorkOrderMaintenanceItem
	if err := tx.First(&item, "issue_id = ? AND tenant_id = ?", issue.ID, order.TenantID).Error; err != nil {
		return errs.ErrConflict.WithMsg("维保工单项缺失")
	}
	if item.MaintenanceID != nil {
		return nil
	}
	var equipment eqmodel.Equipment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&equipment, "id = ? AND tenant_id = ? AND community_id = ? AND deleted_at IS NULL", item.EquipmentID, order.TenantID, order.CommunityID).Error; err != nil {
		return errs.ErrConflict.WithMsg("维保设备已失效")
	}
	if equipment.Status == eqmodel.StatusScrapped {
		return errs.ErrConflict.WithMsg("维保设备已报废")
	}
	rules := eqsvc.NewEquipmentService(tx).TypeRules()
	now := time.Now()
	operatorID := reviewerID
	if order.AssigneeID != nil {
		operatorID = *order.AssigneeID
	}
	record := eqmodel.EquipmentMaintenance{TenantID: &order.TenantID, EquipmentID: equipment.ID, CommunityID: order.CommunityID,
		MaintenanceType: item.MaintenanceAction, MaintenanceDate: now, OperatorName: operatorID,
		Note: "周期维保工单审核通过", ConfirmStatus: eqmodel.ConfirmConfirmed, ConfirmedBy: &reviewerID,
		ConfirmedAt: &now, ConfirmMode: eqmodel.ConfirmModeManual, Source: eqmodel.SourceForm,
		FlowSnapshot: order.ReviewFlow, CreatedBy: operatorID}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	if _, err := eqsvc.ApplyLedgerWriteback(tx, &record, &equipment, rules[equipment.Type]); err != nil {
		return err
	}
	return tx.Model(&item).Update("maintenance_id", record.ID).Error
}
