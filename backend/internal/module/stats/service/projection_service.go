package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"anxuncloud/internal/module/inspection/contract"
	"anxuncloud/internal/module/stats/model"
)

type ProjectionService struct {
	db        *gorm.DB
	loadTask  contract.ProjectionLoader
	loadAlert contract.ConfigAlertLoader
	taskIDs   func(*gorm.DB) ([]string, error)
	alertIDs  func(*gorm.DB) ([]string, error)
}

func NewProjectionService(db *gorm.DB, loadTask contract.ProjectionLoader, loadAlert contract.ConfigAlertLoader, taskIDs, alertIDs func(*gorm.DB) ([]string, error)) *ProjectionService {
	return &ProjectionService{db: db, loadTask: loadTask, loadAlert: loadAlert, taskIDs: taskIDs, alertIDs: alertIDs}
}

func (s *ProjectionService) SyncTask(tx *gorm.DB, taskID string) error {
	if s.loadTask == nil || taskID == "" {
		return fmt.Errorf("任务投影加载器未配置")
	}
	task, records, err := s.loadTask(tx, taskID)
	if err != nil {
		return err
	}
	now := time.Now()
	row := model.Task{
		ID: task.ID, TenantID: task.TenantID, PlanID: task.PlanID, PlanName: task.PlanName,
		CommunityID: task.CommunityID, CommunityName: task.CommunityName,
		InspectorID: task.InspectorID, InspectorName: task.InspectorName,
		PatrolType: task.PatrolType, TaskDate: task.TaskDate, RoundName: task.RoundName,
		TimeWindow: task.TimeWindow, DueDate: task.DueDate, Status: task.Status,
		PointIDs: task.PointIDs, TotalPoints: task.TotalPoints, DonePoints: task.DonePoints,
		BlockedPoints: task.BlockedPoints, StartedAt: task.StartedAt, FinishedAt: task.FinishedAt,
		DailyMinRounds: task.DailyMinRounds, SourceUpdatedAt: task.SourceUpdatedAt, ProjectedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, UpdateAll: true}).Create(&row).Error; err != nil {
		return err
	}
	for _, record := range records {
		item := model.Checkin{
			ID: record.ID, TaskID: record.TaskID, TenantID: record.TenantID,
			CommunityID: record.CommunityID, CommunityName: record.CommunityName,
			PointID: record.PointID, PointName: record.PointName,
			InspectorID: record.InspectorID, InspectorName: record.InspectorName,
			CheckinTime: record.CheckinTime, Result: record.Result, IsSuspect: record.IsSuspect,
			Remark: record.Remark, SupersededBy: record.SupersededBy,
			SourceUpdatedAt: record.SourceUpdatedAt, ProjectedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, UpdateAll: true}).Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *ProjectionService) DeleteTask(tx *gorm.DB, taskID string) error {
	if err := tx.Delete(&model.Checkin{}, "task_id = ?", taskID).Error; err != nil {
		return err
	}
	return tx.Delete(&model.Task{}, "id = ?", taskID).Error
}

func (s *ProjectionService) SyncConfigAlert(tx *gorm.DB, pointID string) error {
	if s.loadAlert == nil || pointID == "" {
		return fmt.Errorf("配置告警投影加载器未配置")
	}
	alert, err := s.loadAlert(tx, pointID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	row := model.ConfigAlert{
		ID: alert.ID, TenantID: alert.TenantID, CommunityID: alert.CommunityID,
		CommunityName: alert.CommunityName, PointID: alert.PointID, PointName: alert.PointName,
		Reason: alert.Reason, StartedAt: alert.StartedAt, EndedAt: alert.EndedAt,
		SourceUpdatedAt: alert.SourceUpdatedAt, ProjectedAt: time.Now(),
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, UpdateAll: true}).Create(&row).Error
}

func (s *ProjectionService) Rebuild() error {
	if s.taskIDs == nil || s.alertIDs == nil {
		return fmt.Errorf("全量投影数据源未配置")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		taskIDs, err := s.taskIDs(tx)
		if err != nil {
			return err
		}
		alertIDs, err := s.alertIDs(tx)
		if err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM stats_checkin").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM stats_task").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM stats_config_alert").Error; err != nil {
			return err
		}
		for _, id := range taskIDs {
			if err := s.SyncTask(tx, id); err != nil {
				return err
			}
		}
		for _, id := range alertIDs {
			if err := s.SyncConfigAlert(tx, id); err != nil {
				return err
			}
		}
		now := time.Now()
		meta := model.ProjectionMeta{Source: "inspection", LastRebuildAt: now, SourceUpdatedAt: now}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source"}}, UpdateAll: true}).Create(&meta).Error
	})
}

func (s *ProjectionService) Status() (model.ProjectionMeta, error) {
	var meta model.ProjectionMeta
	err := s.db.First(&meta, "source = ?", "inspection").Error
	return meta, err
}
