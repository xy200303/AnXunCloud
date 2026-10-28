package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"anxuncloud/internal/module/report/model"
	queuepkg "anxuncloud/internal/pkg/queue"
	"anxuncloud/internal/pkg/strutil"
)

type ReportRenderArgs struct {
	JobID    string `json:"job_id" river:"unique"`
	ReportID string `json:"report_id"`
	TenantID string `json:"tenant_id"`
	Stage    string `json:"stage"`
}

func (ReportRenderArgs) Kind() string { return "report.volume.render" }

func (ReportRenderArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "render", MaxAttempts: 4, UniqueOpts: river.UniqueOpts{ByArgs: true, ByQueue: true}}
}

func NewReportRenderWorker(s *ReportService) river.Worker[ReportRenderArgs] {
	return river.WorkFunc(s.workRiverReportRender)
}

func (s *ReportService) BindQueue(queue *queuepkg.Queue) { s.queue = queue }

func (s *ReportService) enqueueRender(ctx context.Context, reportID, stage string) error {
	if s.queue == nil {
		return fmt.Errorf("报告任务队列未配置")
	}
	if stage != model.FileStageFrozen && stage != model.FileStageArchived {
		return fmt.Errorf("报告渲染阶段无效")
	}
	var report model.InspectionReport
	if err := s.db.Select("id", "tenant_id").First(&report, "id = ?", reportID).Error; err != nil || report.TenantID == nil {
		return fmt.Errorf("报告不存在或缺少租户")
	}
	jobID := reportID + ":" + stage
	tx, err := s.queue.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := upsertRenderTask(ctx, tx, report.ID, *report.TenantID, stage); err != nil {
		return err
	}
	if _, err := s.queue.InsertTx(ctx, tx, ReportRenderArgs{JobID: jobID, ReportID: report.ID, TenantID: *report.TenantID, Stage: stage}, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertRenderTask(ctx context.Context, tx pgx.Tx, reportID, tenantID, stage string) error {
	_, err := tx.Exec(ctx, `INSERT INTO report_render_task (id, tenant_id, report_id, stage, status, created_at, updated_at)
		VALUES (md5($1)::uuid, $2, $3, $4, 'pending', now(), now())
		ON CONFLICT (report_id, stage) DO UPDATE SET status='pending', error_message='', updated_at=now()`,
		"render-task:"+reportID+":"+stage, tenantID, reportID, stage)
	return err
}

func (s *ReportService) workRiverReportRender(ctx context.Context, job *river.Job[ReportRenderArgs]) error {
	if job.Args.TenantID == "" || job.Args.ReportID == "" {
		return fmt.Errorf("报告任务缺少租户或报告身份")
	}
	if err := s.db.Model(&model.ReportRenderTask{}).Where("report_id = ? AND stage = ? AND tenant_id = ?", job.Args.ReportID, job.Args.Stage, job.Args.TenantID).
		Updates(map[string]any{"status": "running", "updated_at": time.Now()}).Error; err != nil {
		return err
	}
	var report model.InspectionReport
	if err := s.db.First(&report, "id = ? AND tenant_id = ?", job.Args.ReportID, job.Args.TenantID).Error; err != nil {
		return err
	}
	err := s.generateVolumes(&report, job.Args.Stage)
	status, message := "done", ""
	if err != nil {
		status, message = "failed", strutil.Truncate(err.Error(), 512)
	}
	if updateErr := s.db.Model(&model.ReportRenderTask{}).Where("report_id = ? AND stage = ? AND tenant_id = ?", job.Args.ReportID, job.Args.Stage, job.Args.TenantID).
		Updates(map[string]any{"status": status, "error_message": message, "updated_at": time.Now()}).Error; updateErr != nil {
		return updateErr
	}
	return err
}
