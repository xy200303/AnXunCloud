package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"anxuncloud/internal/middleware"
	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/types"
)

type AIItemJobArgs struct {
	JobID   string          `json:"job_id" river:"unique"`
	Payload json.RawMessage `json:"payload"`
}

func (AIItemJobArgs) Kind() string { return "ai.item.inspect" }

func (AIItemJobArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "ai", MaxAttempts: 3, UniqueOpts: river.UniqueOpts{ByArgs: true, ByQueue: true}}
}

func NewAIItemWorker(s *CheckinService) river.Worker[AIItemJobArgs] {
	return river.WorkFunc(s.workRiverAIItem)
}

func (s *CheckinService) enqueueAIItemJob(ctx context.Context, draft *insmodel.CheckinItemDraft, payload aiItemJobPayload) error {
	if s.queue == nil {
		return fmt.Errorf("AI 任务队列未配置")
	}
	if payload.TenantID == nil || *payload.TenantID == "" {
		return fmt.Errorf("AI 任务缺少租户")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := s.queue.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := upsertAIItemDraftTx(ctx, tx, draft); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ai_item_job (id, tenant_id, user_id, status, payload, created_at, updated_at)
		VALUES ($1,$2,$3,'pending',$4,$5,$5)`, payload.JobID, *payload.TenantID, payload.UserID, raw, time.Now()); err != nil {
		return err
	}
	if _, err := s.queue.InsertTx(ctx, tx, AIItemJobArgs{JobID: payload.JobID, Payload: raw}, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertAIItemDraftTx(ctx context.Context, tx pgx.Tx, draft *insmodel.CheckinItemDraft) error {
	var existingID string
	err := tx.QueryRow(ctx, `SELECT id FROM checkin_item_draft
		WHERE task_id=$1 AND point_id=$2 AND item_code=$3
		  AND (($4::uuid IS NULL AND template_id IS NULL) OR template_id=$4)
		  AND (($5::uuid IS NULL AND equipment_id IS NULL) OR equipment_id=$5)
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, draft.TaskID, draft.PointID, draft.ItemCode, draft.TemplateID, draft.EquipmentID).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	values := []any{draft.TenantID, draft.InspectorID, draft.CommunityID, draft.RequestID, draft.RetakeSeq, draft.PhotoSetHash,
		draft.DraftKind, draft.JobID, draft.FileIDs, draft.AIStatus, draft.ShootLng, draft.ShootLat, draft.ShootAt, time.Now()}
	if existingID != "" {
		_, err = tx.Exec(ctx, `UPDATE checkin_item_draft SET tenant_id=$1, inspector_id=$2, community_id=$3,
			request_id=$4, retake_seq=$5, photo_set_hash=$6, draft_kind=$7, job_id=$8, file_ids=$9,
			ai_status=$10, shoot_lng=$11, shoot_lat=$12, shoot_at=$13, updated_at=$14 WHERE id=$15`, append(values, existingID)...)
		return err
	}
	if draft.ID == "" {
		draft.ID = types.NewID()
	}
	_, err = tx.Exec(ctx, `INSERT INTO checkin_item_draft
		(id, tenant_id, task_id, point_id, inspector_id, community_id, template_id, item_code, equipment_id,
		 request_id, retake_seq, photo_set_hash, draft_kind, job_id, file_ids, ai_status,
		 shoot_lng, shoot_lat, shoot_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$20)`,
		draft.ID, draft.TenantID, draft.TaskID, draft.PointID, draft.InspectorID, draft.CommunityID,
		draft.TemplateID, draft.ItemCode, draft.EquipmentID, draft.RequestID, draft.RetakeSeq, draft.PhotoSetHash,
		draft.DraftKind, draft.JobID, draft.FileIDs, draft.AIStatus, draft.ShootLng, draft.ShootLat, draft.ShootAt, time.Now())
	return err
}

func (s *CheckinService) workRiverAIItem(ctx context.Context, job *river.Job[AIItemJobArgs]) error {
	var payload aiItemJobPayload
	if err := json.Unmarshal(job.Args.Payload, &payload); err != nil {
		return err
	}
	if payload.TenantID == nil || *payload.TenantID == "" {
		return fmt.Errorf("AI 任务缺少租户")
	}
	enabled, err := middleware.TenantEnabled(s.db, *payload.TenantID)
	if err != nil || !enabled {
		return fmt.Errorf("AI 任务租户不可用")
	}
	if err := s.db.Model(&jobProjection{}).Where("id = ? AND tenant_id = ?", payload.JobID, *payload.TenantID).
		Updates(map[string]any{"status": "running", "updated_at": time.Now()}).Error; err != nil {
		return err
	}
	return s.processAIItemJob(ctx, payload)
}

type jobProjection struct {
	ID string `gorm:"column:id"`
}

func (jobProjection) TableName() string { return "ai_item_job" }
