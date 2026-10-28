package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

const (
	TaskPending    = "pending"
	TaskDoing      = "doing"
	TaskDone       = "done"
	TaskOverdue    = "overdue"
	ResultAbnormal = "abnormal"
)

type Task struct {
	ID              string        `gorm:"primaryKey;type:uuid" json:"id"`
	TenantID        string        `gorm:"type:uuid" json:"tenant_id"`
	PlanID          string        `gorm:"type:uuid" json:"plan_id"`
	PlanName        string        `json:"plan_name"`
	CommunityID     string        `gorm:"type:uuid" json:"community_id"`
	CommunityName   string        `json:"community_name"`
	InspectorID     string        `gorm:"type:uuid" json:"inspector_id"`
	InspectorName   string        `json:"inspector_name"`
	PatrolType      string        `json:"patrol_type"`
	TaskDate        time.Time     `gorm:"type:date" json:"task_date"`
	RoundName       string        `json:"round_name"`
	TimeWindow      string        `json:"time_window"`
	DueDate         *time.Time    `gorm:"type:date" json:"due_date"`
	Status          string        `json:"status"`
	PointIDs        types.IDArray `gorm:"type:jsonb" json:"point_ids"`
	TotalPoints     int           `json:"total_points"`
	DonePoints      int           `json:"done_points"`
	BlockedPoints   int           `json:"blocked_points"`
	StartedAt       *time.Time    `json:"started_at"`
	FinishedAt      *time.Time    `json:"finished_at"`
	DailyMinRounds  *int          `json:"daily_min_rounds"`
	SourceUpdatedAt time.Time     `json:"source_updated_at"`
	ProjectedAt     time.Time     `json:"projected_at"`
}

func (Task) TableName() string { return "stats_task" }

type Checkin struct {
	ID              string    `gorm:"primaryKey;type:uuid" json:"id"`
	TaskID          string    `gorm:"type:uuid" json:"task_id"`
	TenantID        string    `gorm:"type:uuid" json:"tenant_id"`
	CommunityID     string    `gorm:"type:uuid" json:"community_id"`
	CommunityName   string    `json:"community_name"`
	PointID         string    `gorm:"type:uuid" json:"point_id"`
	PointName       string    `json:"point_name"`
	InspectorID     string    `gorm:"type:uuid" json:"inspector_id"`
	InspectorName   string    `json:"inspector_name"`
	CheckinTime     time.Time `json:"checkin_time"`
	Result          string    `json:"result"`
	IsSuspect       bool      `json:"is_suspect"`
	Remark          string    `json:"remark"`
	SupersededBy    *string   `gorm:"type:uuid" json:"superseded_by"`
	SourceUpdatedAt time.Time `json:"source_updated_at"`
	ProjectedAt     time.Time `json:"projected_at"`
}

func (Checkin) TableName() string { return "stats_checkin" }

type ConfigAlert struct {
	ID              string     `gorm:"primaryKey;type:uuid" json:"id"`
	TenantID        string     `gorm:"type:uuid" json:"tenant_id"`
	CommunityID     string     `gorm:"type:uuid" json:"community_id"`
	CommunityName   string     `json:"community_name"`
	PointID         string     `gorm:"type:uuid" json:"point_id"`
	PointName       string     `json:"point_name"`
	Reason          string     `json:"reason"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	SourceUpdatedAt time.Time  `json:"source_updated_at"`
	ProjectedAt     time.Time  `json:"projected_at"`
}

func (ConfigAlert) TableName() string { return "stats_config_alert" }

type ProjectionMeta struct {
	Source          string    `gorm:"primaryKey;size:32" json:"source"`
	LastRebuildAt   time.Time `json:"last_rebuild_at"`
	SourceUpdatedAt time.Time `json:"source_updated_at"`
}

func (ProjectionMeta) TableName() string { return "stats_projection_meta" }
