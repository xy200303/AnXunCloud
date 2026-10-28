package service

import (
	"github.com/redis/go-redis/v9"
	"github.com/riverqueue/river"
	"gorm.io/gorm"

	inssvc "anxuncloud/internal/module/inspection/service"
	"anxuncloud/internal/pkg/notify"
	"anxuncloud/internal/pkg/storage"
)

type CheckinService = inssvc.CheckinService

func NewCheckinService(db *gorm.DB, rdb *redis.Client, store *storage.Storage, getCfg func(string) (string, bool), notifier *notify.Notifier) *CheckinService {
	return inssvc.NewCheckinService(db, rdb, store, getCfg, notifier)
}

func NewAIItemWorker(s *CheckinService) river.Worker[inssvc.AIItemJobArgs] {
	return inssvc.NewAIItemWorker(s)
}
