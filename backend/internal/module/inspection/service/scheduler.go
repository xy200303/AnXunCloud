package service

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/database"
	"anxuncloud/internal/pkg/logger"
	"go.uber.org/zap"
)

// Scheduler 服务内定时任务：每日生成任务、逾期翻转、分区滚动。
type Scheduler struct {
	db      *gorm.DB
	plans   *PlanService
	getCfg  func(key string) (string, bool)
	stopCh  chan struct{}
	lastGen string // 上次任务生成日期（yyyyMMdd）
	lastFlip string
}

func NewScheduler(db *gorm.DB, plans *PlanService, getCfg func(string) (string, bool)) *Scheduler {
	return &Scheduler{db: db, plans: plans, getCfg: getCfg, stopCh: make(chan struct{})}
}

// Start 启动调度循环（每 30s 检查一次到点任务）。
func (s *Scheduler) Start() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		s.tick() // 启动即执行一次，保证新部署当天任务生成
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}

// Stop 停止调度。
func (s *Scheduler) Stop() { close(s.stopCh) }

func (s *Scheduler) tick() {
	ctx := context.Background()
	now := time.Now()
	today := now.Format("20060102")

	// 每日 00:05 后生成任务（生成范围：今天起 N 天，N 取参数 inspection.task_generate_days）
	if today != s.lastGen && now.Format("15:04") >= "00:05" {
		days := 1
		if s.getCfg != nil {
			if v, ok := s.getCfg("inspection.task_generate_days"); ok {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					days = n
				}
			}
		}
		total := 0
		for i := 0; i <= days; i++ {
			n, be := s.plans.GenerateForDate(ctx, now.AddDate(0, 0, i))
			if be != nil {
				logger.L.Warn("任务生成失败", zap.Error(be))
				continue
			}
			total += n
		}
		s.lastGen = today
		logger.L.Info("每日任务生成完成", zap.Int("created", total))
		// 分区滚动：确保当月与下月分区存在（幂等）
		for _, parent := range []string{"checkin_record", "sys_operation_log"} {
			if err := database.EnsurePartitions(s.db, parent); err != nil {
				logger.L.Warn("分区滚动创建失败", zap.String("table", parent), zap.Error(err))
			}
		}
	}

	// 每日逾期翻转（时间取参数 inspection.overdue_check_time，默认 00:10）
	flipTime := "00:10"
	if s.getCfg != nil {
		if v, ok := s.getCfg("inspection.overdue_check_time"); ok && v != "" {
			flipTime = v
		}
	}
	if today != s.lastFlip && now.Format("15:04") >= flipTime {
		n, err := s.plans.FlipOverdue()
		if err != nil {
			logger.L.Warn("逾期翻转失败", zap.Error(err))
		} else {
			s.lastFlip = today
			logger.L.Info("逾期任务翻转完成", zap.Int64("count", n))
		}
	}
}
