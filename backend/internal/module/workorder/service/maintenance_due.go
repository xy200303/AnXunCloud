package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	communitysvc "anxuncloud/internal/module/community/service"
	eqmodel "anxuncloud/internal/module/equipment/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/module/workorder/model"
	"anxuncloud/internal/pkg/types"
)

func (s *WorkOrderService) CreateDueMaintenance(now time.Time) (int, error) {
	var due []eqmodel.Equipment
	if err := s.db.Where("next_due_date IS NOT NULL AND next_due_date <= ? AND status = ? AND deleted_at IS NULL", now.AddDate(0, 0, 7), eqmodel.StatusInService).Order("tenant_id, community_id, type, next_due_date, id").Find(&due).Error; err != nil {
		return 0, err
	}
	type group struct {
		tenantID, communityID, equipmentType, action, duePeriod string
		equipment                                               []eqmodel.Equipment
	}
	groups := map[string]*group{}
	for _, equipment := range due {
		if equipment.TenantID == nil || equipment.NextDueDate == nil {
			continue
		}
		action := "maintain"
		if equipment.Type == "extinguisher" {
			action = "refill"
		}
		period := equipment.NextDueDate.Format("2006-01")
		key := fmt.Sprintf("%s/%s/%s/%s/%s", *equipment.TenantID, equipment.CommunityID, equipment.Type, action, period)
		if groups[key] == nil {
			groups[key] = &group{tenantID: *equipment.TenantID, communityID: equipment.CommunityID, equipmentType: equipment.Type, action: action, duePeriod: period}
		}
		groups[key].equipment = append(groups[key].equipment, equipment)
	}
	created := 0
	for _, group := range groups {
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			flow := communitysvc.ResolveFlow(tx, group.communityID, sysmodel.FlowMaintReview)
			if len(flow) == 0 {
				return fmt.Errorf("维保审批链未配置")
			}
			var order model.WorkOrder
			err := tx.Where("tenant_id = ? AND community_id = ? AND work_type = ? AND title = ? AND status IN ?", group.tenantID, group.communityID, model.WorkTypeMaintenance,
				"周期维保 · "+group.equipmentType+" · "+group.action+" · "+group.duePeriod, []string{model.StatusPending, model.StatusInProgress, model.StatusPendingReview}).First(&order).Error
			if err != nil {
				order = model.WorkOrder{TenantID: group.tenantID, CommunityID: group.communityID,
					OrderNo:  fmt.Sprintf("WO%s%s", now.Format("20060102150405"), strings.ToUpper(strings.ReplaceAll(types.NewID(), "-", "")[16:])),
					WorkType: model.WorkTypeMaintenance, Title: "周期维保 · " + group.equipmentType + " · " + group.action + " · " + group.duePeriod,
					Status: model.StatusPending, Priority: "normal", ReviewFlow: flow, Revision: 1}
				if err := tx.Create(&order).Error; err != nil {
					return err
				}
			}
			for _, equipment := range group.equipment {
				sourceItemID := equipment.ID + ":" + group.duePeriod
				var count int64
				if err := tx.Model(&model.WorkOrderIssue{}).Where("tenant_id = ? AND source_type = ? AND source_item_id = ?", group.tenantID, "equipment.maintenance.due", sourceItemID).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					continue
				}
				issue := model.WorkOrderIssue{TenantID: group.tenantID, CommunityID: group.communityID, OrderID: order.ID,
					SourceType: "equipment.maintenance.due", SourceRecordID: equipment.ID, SourceItemID: sourceItemID,
					SourceRevision: 1, Title: "周期维保 · " + equipment.Name, Problem: "设备已到期，需要执行" + group.action,
					EquipmentID: &equipment.ID, Status: model.StatusPending}
				if err := tx.Create(&issue).Error; err != nil {
					return err
				}
				item := model.WorkOrderMaintenanceItem{TenantID: group.tenantID, OrderID: order.ID, IssueID: issue.ID,
					EquipmentID: equipment.ID, MaintenanceAction: group.action, DuePeriod: group.duePeriod}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
				created++
			}
			return nil
		}); err != nil {
			return created, err
		}
	}
	return created, nil
}
