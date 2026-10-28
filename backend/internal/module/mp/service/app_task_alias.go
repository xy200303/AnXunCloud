package service

import (
	"github.com/gin-gonic/gin"

	inssvc "anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/errs"
)

func (s *MPService) TodayTasks(inspectorID string) (gin.H, *errs.Error) {
	return inssvc.NewAppTaskService(s.db).TodayTasks(inspectorID)
}

func (s *MPService) HistoryTasks(inspectorID, date string) (gin.H, *errs.Error) {
	return inssvc.NewAppTaskService(s.db).HistoryTasks(inspectorID, date)
}

func (s *MPService) TaskDetail(inspectorID, taskID string) (gin.H, *errs.Error) {
	return inssvc.NewAppTaskService(s.db).TaskDetail(inspectorID, taskID)
}

func (s *MPService) PointByCode(inspectorID, code string) (gin.H, *errs.Error) {
	return inssvc.NewAppTaskService(s.db).PointByCode(inspectorID, code)
}

func (s *MPService) PublicPoint(code string) (gin.H, *errs.Error) {
	return inssvc.NewAppTaskService(s.db).PublicPoint(code)
}

func (s *MPService) NearbyPoints(inspectorID string, lng, lat float64) (gin.H, *errs.Error) {
	return inssvc.NewAppTaskService(s.db).NearbyPoints(inspectorID, lng, lat)
}
