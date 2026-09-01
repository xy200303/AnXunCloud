package service

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"

	reportsvc "anxuncloud/internal/module/report/service"
	"anxuncloud/internal/pkg/database"
	"anxuncloud/internal/pkg/logger"
	"go.uber.org/zap"
)

// PlanJob 统一计划引擎的任务单元：每种计划类型实现一个 Run，调度器按注册表遍历执行。
// 新增计划类型（设备保养、培训、保洁等）只需 NewScheduler 处加一行 Register，调度器零改动。
// Run 内部自带幂等（上次运行键判重/业务唯一约束），返回 error 仅记日志不影响其他任务。
type PlanJob struct {
	Name string
	Run  func(now time.Time) error
}

// Scheduler 统一计划引擎：每日任务生成、逾期翻转、分区滚动、报告生成计划（注册式）。
type Scheduler struct {
	db      *gorm.DB
	plans   *PlanService
	reports *reportsvc.ReportService
	getCfg  func(key string) (string, bool)
	jobs    []PlanJob
	stopCh  chan struct{}
}

func NewScheduler(db *gorm.DB, plans *PlanService, reports *reportsvc.ReportService, getCfg func(string) (string, bool)) *Scheduler {
	s := &Scheduler{db: db, plans: plans, reports: reports, getCfg: getCfg, stopCh: make(chan struct{})}
	s.registerBuiltin()
	return s
}

// Register 注册计划任务（启动时调用；同一任务重复注册会重复执行，调用方保证唯一）。
func (s *Scheduler) Register(j PlanJob) { s.jobs = append(s.jobs, j) }

// registerBuiltin 内置任务：任务生成 / 逾期翻转 / 分区滚动 / 报告生成计划。
func (s *Scheduler) registerBuiltin() {
	// 每日 00:05 后生成任务（生成范围：今天起 N 天，N 取参数 inspection.task_generate_days）
	lastGen := ""
	s.Register(PlanJob{Name: "task_generate", Run: func(now time.Time) error {
		today := now.Format("20060102")
		if today == lastGen || now.Format("15:04") < "00:05" {
			return nil
		}
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
			n, _, be := s.plans.GenerateForDate(context.Background(), now.AddDate(0, 0, i))
			if be != nil {
				logger.L.Warn("任务生成失败", zap.Error(be))
				continue
			}
			total += n
		}
		lastGen = today
		logger.L.Info("每日任务生成完成", zap.Int("created", total))
		return nil
	}})

	// 分区滚动：确保当月与下月分区存在（幂等，每天随任务生成后执行）
	lastPart := ""
	s.Register(PlanJob{Name: "partition_roll", Run: func(now time.Time) error {
		today := now.Format("20060102")
		if today == lastPart {
			return nil
		}
		for _, parent := range []string{"checkin_record", "sys_operation_log"} {
			if err := database.EnsurePartitions(s.db, parent); err != nil {
				logger.L.Warn("分区滚动创建失败", zap.String("table", parent), zap.Error(err))
			}
		}
		lastPart = today
		return nil
	}})

	// 每日逾期翻转（时间取参数 inspection.overdue_check_time，默认 00:10）
	lastFlip := ""
	s.Register(PlanJob{Name: "overdue_flip", Run: func(now time.Time) error {
		today := now.Format("20060102")
		flipTime := "00:10"
		if s.getCfg != nil {
			if v, ok := s.getCfg("inspection.overdue_check_time"); ok && v != "" {
				flipTime = v
			}
		}
		if today == lastFlip || now.Format("15:04") < flipTime {
			return nil
		}
		n, err := s.plans.FlipOverdue()
		if err != nil {
			logger.L.Warn("逾期翻转失败", zap.Error(err))
			return nil
		}
		lastFlip = today
		logger.L.Info("逾期任务翻转完成", zap.Int64("count", n))
		return nil
	}})

	// 报告生成计划：扫描启用的报告计划，到期生成上一份完整周期报告（月/周/日）
	if s.reports != nil {
		s.Register(PlanJob{Name: "report_plan", Run: func(now time.Time) error {
			n, err := s.reports.RunDueReportPlans(now)
			if err != nil {
				return err
			}
			if n > 0 {
				logger.L.Info("报告计划扫描完成", zap.Int("created", n))
			}
			return nil
		}})
	}
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
	now := time.Now()
	for _, j := range s.jobs {
		if err := j.Run(now); err != nil {
			logger.L.Warn("计划任务执行失败", zap.String("job", j.Name), zap.Error(err))
		}
	}
}
