package contract

import (
	"time"

	"gorm.io/gorm"
)

type TaskProjection struct {
	ID              string
	TenantID        string
	PlanID          string
	PlanName        string
	CommunityID     string
	CommunityName   string
	InspectorID     string
	InspectorName   string
	PatrolType      string
	TaskDate        time.Time
	RoundName       string
	TimeWindow      string
	DueDate         *time.Time
	Status          string
	PointIDs        []string
	TotalPoints     int
	DonePoints      int
	BlockedPoints   int
	StartedAt       *time.Time
	FinishedAt      *time.Time
	DailyMinRounds  *int
	SourceUpdatedAt time.Time
}

type CheckinProjection struct {
	ID              string
	TaskID          string
	TenantID        string
	CommunityID     string
	CommunityName   string
	PointID         string
	PointName       string
	InspectorID     string
	InspectorName   string
	CheckinTime     time.Time
	Result          string
	IsSuspect       bool
	Remark          string
	SupersededBy    *string
	SourceUpdatedAt time.Time
}

type ConfigAlertProjection struct {
	ID              string
	TenantID        string
	CommunityID     string
	CommunityName   string
	PointID         string
	PointName       string
	Reason          string
	StartedAt       time.Time
	EndedAt         *time.Time
	SourceUpdatedAt time.Time
}

type ProjectionSink interface {
	SyncTask(*gorm.DB, string) error
	DeleteTask(*gorm.DB, string) error
	SyncConfigAlert(*gorm.DB, string) error
}

type ProjectionLoader func(*gorm.DB, string) (TaskProjection, []CheckinProjection, error)
type ConfigAlertLoader func(*gorm.DB, string) (ConfigAlertProjection, error)
