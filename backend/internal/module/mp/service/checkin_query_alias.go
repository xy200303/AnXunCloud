package service

import (
	"github.com/gin-gonic/gin"

	inssvc "anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/errs"
)

func (s *MPService) CheckinItems(inspectorID, checkinID string) ([]gin.H, *errs.Error) {
	return inssvc.NewAppCheckinQueryService(s.db).CheckinItems(inspectorID, checkinID)
}

func (s *MPService) CheckinBrief(inspectorID, checkinID string) (gin.H, *errs.Error) {
	return inssvc.NewAppCheckinQueryService(s.db).CheckinBrief(inspectorID, checkinID)
}
