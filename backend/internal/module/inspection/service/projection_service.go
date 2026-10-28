package service

import (
	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
)

var taskProjectionSink contract.ProjectionSink

func BindProjectionSink(sink contract.ProjectionSink) { taskProjectionSink = sink }

func SyncTaskProjection(tx *gorm.DB, taskID string) error {
	if taskProjectionSink == nil || taskID == "" {
		return nil
	}
	return taskProjectionSink.SyncTask(tx, taskID)
}

func DeleteTaskProjection(tx *gorm.DB, taskID string) error {
	if taskProjectionSink == nil || taskID == "" {
		return nil
	}
	return taskProjectionSink.DeleteTask(tx, taskID)
}

func syncConfigAlertProjection(tx *gorm.DB, pointID string) error {
	if taskProjectionSink == nil || pointID == "" {
		return nil
	}
	return taskProjectionSink.SyncConfigAlert(tx, pointID)
}

func LoadTaskProjection(tx *gorm.DB, taskID string) (contract.TaskProjection, []contract.CheckinProjection, error) {
	var task model.InspectionTask
	if err := tx.First(&task, "id = ?", taskID).Error; err != nil {
		return contract.TaskProjection{}, nil, err
	}
	var plan model.InspectionPlan
	_ = tx.Unscoped().Select("name", "cycle_config").First(&plan, "id = ?", task.PlanID).Error
	var community sysmodel.Community
	_ = tx.Select("name").First(&community, "id = ?", task.CommunityID).Error
	var inspector sysmodel.SysUser
	_ = tx.Select("name").First(&inspector, "id = ?", task.InspectorID).Error
	blocked, err := BlockedUnfinishedPointIDs(tx, task.ID, []string(model.TaskPointIDs(&task)))
	if err != nil {
		return contract.TaskProjection{}, nil, err
	}
	tenantID := ""
	if task.TenantID != nil {
		tenantID = *task.TenantID
	}
	projection := contract.TaskProjection{
		ID: task.ID, TenantID: tenantID, PlanID: task.PlanID, PlanName: plan.Name,
		CommunityID: task.CommunityID, CommunityName: community.Name,
		InspectorID: task.InspectorID, InspectorName: inspector.Name,
		PatrolType: task.PatrolType, TaskDate: task.TaskDate, RoundName: task.RoundName,
		TimeWindow: task.TimeWindow, DueDate: task.DueDate, Status: task.Status,
		PointIDs: model.TaskPointIDs(&task), TotalPoints: task.TotalPoints,
		DonePoints: task.DonePoints, BlockedPoints: len(blocked),
		StartedAt: task.StartedAt, FinishedAt: task.FinishedAt,
		DailyMinRounds: model.PlanDailyMinRounds(plan.CycleConfig), SourceUpdatedAt: task.UpdatedAt,
	}
	var records []model.CheckinRecord
	if err := tx.Where("task_id = ?", task.ID).Find(&records).Error; err != nil {
		return contract.TaskProjection{}, nil, err
	}
	pointNames := map[string]string{}
	for _, record := range records {
		if _, ok := pointNames[record.PointID]; !ok {
			var point model.InspectionPoint
			_ = tx.Select("name").First(&point, "id = ?", record.PointID).Error
			pointNames[record.PointID] = point.Name
		}
	}
	out := make([]contract.CheckinProjection, 0, len(records))
	for _, record := range records {
		out = append(out, contract.CheckinProjection{
			ID: record.ID, TaskID: record.TaskID, TenantID: tenantID,
			CommunityID: task.CommunityID, CommunityName: community.Name,
			PointID: record.PointID, PointName: pointNames[record.PointID],
			InspectorID: record.InspectorID, InspectorName: inspector.Name,
			CheckinTime: record.CheckinTime, Result: record.Result,
			IsSuspect: record.IsSuspect, Remark: record.Remark,
			SupersededBy: record.SupersededBy, SourceUpdatedAt: record.CreatedAt,
		})
	}
	return projection, out, nil
}

func LoadConfigAlertProjection(tx *gorm.DB, pointID string) (contract.ConfigAlertProjection, error) {
	var block model.PointConfigBlock
	if err := tx.Where("point_id = ?", pointID).Order("started_at DESC").First(&block).Error; err != nil {
		return contract.ConfigAlertProjection{}, err
	}
	var point model.InspectionPoint
	if err := tx.Select("name").First(&point, "id = ?", pointID).Error; err != nil {
		return contract.ConfigAlertProjection{}, err
	}
	var community sysmodel.Community
	_ = tx.Select("name").First(&community, "id = ?", block.CommunityID).Error
	updatedAt := block.StartedAt
	if block.EndedAt != nil {
		updatedAt = *block.EndedAt
	}
	return contract.ConfigAlertProjection{
		ID: block.ID, TenantID: block.TenantID, CommunityID: block.CommunityID,
		CommunityName: community.Name, PointID: pointID, PointName: point.Name,
		Reason: block.Reason, StartedAt: block.StartedAt, EndedAt: block.EndedAt,
		SourceUpdatedAt: updatedAt,
	}, nil
}

func ProjectionTaskIDs(tx *gorm.DB) ([]string, error) {
	ids := []string{}
	err := tx.Model(&model.InspectionTask{}).Order("id").Pluck("id", &ids).Error
	return ids, err
}

func ProjectionAlertIDs(tx *gorm.DB) ([]string, error) {
	ids := []string{}
	err := tx.Model(&model.PointConfigBlock{}).Distinct().Order("point_id").Pluck("point_id", &ids).Error
	return ids, err
}
