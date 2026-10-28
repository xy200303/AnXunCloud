package service

import (
	"time"

	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/inspection/model"
)

func (s *TaskService) SummaryForAI(c *gin.Context, start, end time.Time) (gin.H, error) {
	var tasks struct {
		Total   int64
		Done    int64
		Overdue int64
	}
	if err := middleware.ApplyCommunityFilter(s.db.Model(&model.InspectionTask{}), c, "inspection_task.community_id").
		Where("task_date >= ? AND task_date < ?", start.Format("2006-01-02"), end.Format("2006-01-02")).
		Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE status = 'done') AS done, COUNT(*) FILTER (WHERE status = 'overdue') AS overdue").Scan(&tasks).Error; err != nil {
		return nil, err
	}
	var abnormal int64
	if err := middleware.ApplyCommunityFilter(s.db.Model(&model.CheckinRecord{}), c, "checkin_record.community_id").
		Where("checkin_time >= ? AND checkin_time < ? AND result = ? AND superseded_by IS NULL", start, end, model.ResultAbnormal).Count(&abnormal).Error; err != nil {
		return nil, err
	}
	return gin.H{"task_total": tasks.Total, "task_done": tasks.Done, "task_overdue": tasks.Overdue, "abnormal_count": abnormal, "start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02")}, nil
}
