package service

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
)

func SyncPointConfigBlock(db *gorm.DB, point *model.InspectionPoint) error {
	reason := model.PointConfigError(point)
	if reason == "" {
		if err := db.Model(&model.PointConfigBlock{}).Where("point_id = ? AND ended_at IS NULL", point.ID).Update("ended_at", time.Now()).Error; err != nil {
			return err
		}
		return syncConfigAlertProjection(db, point.ID)
	}
	var count int64
	if err := db.Model(&model.PointConfigBlock{}).Where("point_id = ? AND ended_at IS NULL", point.ID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		if err := db.Model(&model.PointConfigBlock{}).Where("point_id = ? AND ended_at IS NULL", point.ID).Update("reason", reason).Error; err != nil {
			return err
		}
		return syncConfigAlertProjection(db, point.ID)
	}
	block := model.PointConfigBlock{TenantID: *point.TenantID, CommunityID: point.CommunityID, PointID: point.ID, Reason: reason, StartedAt: time.Now()}
	if err := db.Create(&block).Error; err != nil {
		return err
	}
	return syncConfigAlertProjection(db, point.ID)
}

func ActivePointConfigBlocks(db *gorm.DB, pointIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(pointIDs) == 0 {
		return out, nil
	}
	var blocks []model.PointConfigBlock
	if err := db.Select("point_id", "reason").Where("point_id IN ? AND ended_at IS NULL", pointIDs).Find(&blocks).Error; err != nil {
		return nil, err
	}
	for _, block := range blocks {
		out[block.PointID] = block.Reason
	}
	return out, nil
}

func BlockedUnfinishedPointIDs(db *gorm.DB, taskID string, pointIDs []string) ([]string, error) {
	ids := []string{}
	if len(pointIDs) == 0 {
		return ids, nil
	}
	completed := db.Model(&model.CheckinRecord{}).Select("1").
		Where("task_id = ? AND point_id = inspection_point_config_block.point_id AND superseded_by IS NULL", taskID)
	err := db.Model(&model.PointConfigBlock{}).Select("point_id").
		Where("point_id IN ? AND ended_at IS NULL", pointIDs).
		Where("NOT EXISTS (?)", completed).Pluck("point_id", &ids).Error
	return ids, err
}

func TaskBlockedPointCounts(db *gorm.DB, taskIDs []string) (map[string]int, error) {
	out := map[string]int{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	type row struct {
		TaskID string
		Count  int
	}
	rows := []row{}
	err := db.Table("inspection_task AS t").Select("t.id AS task_id, COUNT(b.point_id) AS count").
		Joins("JOIN inspection_point_config_block b ON b.ended_at IS NULL AND b.point_id IN (SELECT value::uuid FROM jsonb_array_elements_text(t.point_ids))").
		Where("t.id IN ?", taskIDs).
		Where("NOT EXISTS (SELECT 1 FROM checkin_record r WHERE r.task_id = t.id AND r.point_id = b.point_id AND r.superseded_by IS NULL)").
		Group("t.id").Scan(&rows).Error
	for _, r := range rows {
		out[r.TaskID] = r.Count
	}
	return out, err
}

func RequirePointExecutable(db *gorm.DB, point *model.InspectionPoint) *errs.Error {
	if point == nil || point.TenantID == nil || *point.TenantID == "" {
		return errs.ErrDataScope
	}
	if err := SyncPointConfigBlock(db, point); err != nil {
		return errs.ErrInternal
	}
	if reason := model.PointConfigError(point); reason != "" {
		return errs.ErrBlockedByConfig.WithData(map[string]any{"status": "blocked_by_config", "reason": reason})
	}
	return nil
}
