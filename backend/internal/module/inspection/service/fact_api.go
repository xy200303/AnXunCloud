package service

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/inspection/model"
)

func ReadApprovedFacts(db *gorm.DB, query contract.ApprovedFactQuery, yield func(contract.ApprovedFactPage) error) error {
	if query.TenantID == "" || query.CommunityID == "" || query.Start.IsZero() || !query.End.After(query.Start) || yield == nil {
		return fmt.Errorf("已审核事实查询缺少有效范围或消费者")
	}
	page := contract.ApprovedFactPage{}
	var points []model.InspectionPoint
	if err := db.Select("id", "name", "type", "qrcode_no", "nfc_id", "building_id", "unit_no", "floor").
		Where("tenant_id = ? AND community_id = ?", query.TenantID, query.CommunityID).Order("sort, id").Find(&points).Error; err != nil {
		return err
	}
	for _, point := range points {
		page.Points = append(page.Points, contract.PointFact{ID: point.ID, Name: point.Name, Type: point.Type, QRCodeNo: point.QRCodeNo, NfcID: point.NfcID, BuildingID: point.BuildingID, UnitNo: point.UnitNo, Floor: point.Floor})
	}
	if query.PatrolType != "" {
		var tasks []model.InspectionTask
		if err := db.Select("id", "plan_id", "point_ids").Where("tenant_id = ? AND community_id = ? AND task_date >= ? AND task_date < ? AND patrol_type = ?", query.TenantID, query.CommunityID, query.Start.Format("2006-01-02"), query.End.Format("2006-01-02"), query.PatrolType).Find(&tasks).Error; err != nil {
			return err
		}
		for _, task := range tasks {
			page.Tasks = append(page.Tasks, contract.TaskFact{ID: task.ID, PlanID: task.PlanID, PointIDs: append([]string(nil), task.PointIDs...)})
		}
	}
	first := true
	var cursor time.Time
	cursorID := ""
	for {
		recordsQuery := db.Model(&model.CheckinRecord{}).
			Where("tenant_id = ? AND community_id = ? AND checkin_time >= ? AND checkin_time < ? AND superseded_by IS NULL AND audit_status IN ?", query.TenantID, query.CommunityID, query.Start, query.End, []string{model.AuditPass, model.AuditAutoPass})
		if query.PatrolType != "" {
			recordsQuery = recordsQuery.Where("task_id IN (SELECT id FROM inspection_task WHERE tenant_id = ? AND community_id = ? AND patrol_type = ? AND deleted_at IS NULL)", query.TenantID, query.CommunityID, query.PatrolType)
		}
		if !cursor.IsZero() {
			recordsQuery = recordsQuery.Where("(checkin_time > ? OR (checkin_time = ? AND id > ?))", cursor, cursor, cursorID)
		}
		var records []model.CheckinRecord
		if err := recordsQuery.Select("id", "point_id", "inspector_id", "checkin_time", "result", "remark", "audit_status").
			Order("checkin_time, id").Limit(500).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		batch := contract.ApprovedFactPage{}
		if first {
			batch.Points, batch.Tasks = page.Points, page.Tasks
			first = false
		}
		recordIDs := make([]string, 0, len(records))
		for _, record := range records {
			batch.Records = append(batch.Records, contract.RecordFact{ID: record.ID, PointID: record.PointID, InspectorID: record.InspectorID, CheckinTime: record.CheckinTime, Result: record.Result, Remark: record.Remark, AuditStatus: record.AuditStatus})
			recordIDs = append(recordIDs, record.ID)
		}
		var items []model.CheckinRecordItem
		if err := db.Where("tenant_id = ? AND record_id IN ?", query.TenantID, recordIDs).Order("record_id, sort, id").Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			batch.Items = append(batch.Items, contract.ItemFact{RecordID: item.RecordID, TemplateID: item.TemplateID, ItemCode: item.ItemCode, EquipmentID: item.EquipmentID, Name: item.Name, Result: item.Result, Photos: append([]string(nil), item.Photos...), AbnormalTags: append([]string(nil), item.AbnormalTags...), ValueNum: item.ValueNum, ValueDate: item.ValueDate, ValueText: item.ValueText, Note: item.Note, ExceptionType: item.ExceptionType})
		}
		if err := yield(batch); err != nil {
			return err
		}
		last := records[len(records)-1]
		cursor, cursorID = last.CheckinTime, last.ID
		if len(records) < 500 {
			return nil
		}
	}
}
