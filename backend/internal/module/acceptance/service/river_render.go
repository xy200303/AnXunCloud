package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"anxuncloud/internal/module/acceptance/model"
	queuepkg "anxuncloud/internal/pkg/queue"
	"anxuncloud/internal/pkg/strutil"
)

type AcceptanceRenderArgs struct {
	JobID    string `json:"job_id" river:"unique"`
	BatchID  string `json:"batch_id"`
	TenantID string `json:"tenant_id"`
	Revision int    `json:"revision"`
	Stage    string `json:"stage"`
}

func (AcceptanceRenderArgs) Kind() string { return "acceptance.report.render" }

func (AcceptanceRenderArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "render", MaxAttempts: 4, UniqueOpts: river.UniqueOpts{ByArgs: true, ByQueue: true}}
}

func NewAcceptanceRenderWorker(s *BatchService) river.Worker[AcceptanceRenderArgs] {
	return river.WorkFunc(s.workRiverAcceptanceRender)
}

func (s *BatchService) BindQueue(queue *queuepkg.Queue) { s.queue = queue }

func (s *BatchService) enqueueRender(ctx context.Context, batchID, stage string) error {
	if s.queue == nil {
		return fmt.Errorf("承接渲染队列未配置")
	}
	var batch model.Batch
	if err := s.db.First(&batch, "id = ?", batchID).Error; err != nil {
		return err
	}
	jobID := fmt.Sprintf("%s:%d:%s", batchID, batch.Revision, stage)
	tx, err := s.queue.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := upsertAcceptanceRenderTask(ctx, tx, batch.ID, batch.TenantID, stage); err != nil {
		return err
	}
	if _, err := s.queue.InsertTx(ctx, tx, AcceptanceRenderArgs{JobID: jobID, BatchID: batch.ID, TenantID: batch.TenantID, Revision: batch.Revision, Stage: stage}, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertAcceptanceRenderTask(ctx context.Context, tx pgx.Tx, batchID, tenantID, stage string) error {
	_, err := tx.Exec(ctx, `INSERT INTO acceptance_render_task (id, tenant_id, batch_id, stage, status, created_at, updated_at)
		VALUES (md5($1)::uuid, $2, $3, $4, 'pending', now(), now())
		ON CONFLICT (batch_id, stage) DO UPDATE SET status='pending', error_message='', updated_at=now()`,
		"acceptance-render:"+batchID+":"+stage, tenantID, batchID, stage)
	return err
}

func (s *BatchService) workRiverAcceptanceRender(ctx context.Context, job *river.Job[AcceptanceRenderArgs]) error {
	if job.Args.TenantID == "" || job.Args.BatchID == "" {
		return fmt.Errorf("承接渲染任务缺少范围")
	}
	if err := s.db.Model(&model.RenderTask{}).Where("batch_id = ? AND stage = ? AND tenant_id = ?", job.Args.BatchID, job.Args.Stage, job.Args.TenantID).
		Updates(map[string]any{"status": "running", "updated_at": time.Now()}).Error; err != nil {
		return err
	}
	var batch model.Batch
	if err := s.db.First(&batch, "id = ? AND tenant_id = ?", job.Args.BatchID, job.Args.TenantID).Error; err != nil {
		return err
	}
	if job.Args.Revision != batch.Revision {
		return nil
	}
	var targets []model.Target
	var records []model.Record
	if err := s.db.Where("batch_id = ?", batch.ID).Order("created_at, id").Find(&targets).Error; err != nil {
		return err
	}
	if err := s.db.Where("batch_id = ?", batch.ID).Order("updated_at, id").Find(&records).Error; err != nil {
		return err
	}
	var signatures []model.Signature
	if job.Args.Stage == "archived" {
		if err := s.db.Where("batch_id = ? AND revision = ? AND status = ?", batch.ID, batch.Revision, "valid").Order("signed_at, id").Find(&signatures).Error; err != nil {
			return err
		}
	}
	_, err := s.createReportFile(s.db, &batch, job.Args.Stage, signatures)
	status, message := "done", ""
	if err != nil {
		status, message = "failed", strutil.Truncate(err.Error(), 512)
	}
	if updateErr := s.db.Model(&model.RenderTask{}).Where("batch_id = ? AND stage = ? AND tenant_id = ?", batch.ID, job.Args.Stage, job.Args.TenantID).
		Updates(map[string]any{"status": status, "error_message": message, "updated_at": time.Now()}).Error; updateErr != nil {
		return updateErr
	}
	if err != nil {
		return err
	}
	if job.Args.Stage == "archived" {
		return s.db.Model(&model.Batch{}).Where("id = ? AND status = ?", batch.ID, model.StatusHandover).
			Updates(map[string]any{"status": model.StatusDone, "locked_at": time.Now(), "revision": batch.Revision + 1}).Error
	}
	return nil
}
