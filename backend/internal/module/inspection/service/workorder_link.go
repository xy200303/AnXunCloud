package service

import (
	"fmt"

	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
	workordersvc "anxuncloud/internal/module/workorder/service"
)

func (s *ReviewService) createApprovedWorkOrders(tx *gorm.DB, record *model.CheckinRecord, operatorID string) error {
	if s.orderSink == nil || record.TenantID == nil {
		return fmt.Errorf("工单服务未配置或记录缺少租户")
	}
	var rows []model.CheckinRecordItem
	if err := tx.Where("record_id = ? AND result = ?", record.ID, model.ItemResultAbnormal).Order("sort, id").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	req := workordersvc.CreateFromInspectionReq{
		TenantID: *record.TenantID, CommunityID: record.CommunityID,
		RecordID: record.ID, SourceRevision: record.Revision, OperatorID: operatorID, Location: pointName(tx, record.PointID),
		Items: make([]workordersvc.CreateFromInspectionItem, 0, len(rows)),
	}
	for _, row := range rows {
		problem := row.Note
		if problem == "" {
			problem = row.Name
		}
		req.Items = append(req.Items, workordersvc.CreateFromInspectionItem{
			ItemID: row.ID, EquipmentID: row.EquipmentID, Title: row.Name, Problem: problem, Photos: row.Photos,
		})
	}
	return s.orderSink.CreateFromInspection(tx, req)
}
