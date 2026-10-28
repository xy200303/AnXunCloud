package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type ReviewVote struct {
	types.UUIDModel
	TenantID  string    `gorm:"type:uuid" json:"tenant_id"`
	BatchID   string    `gorm:"type:uuid" json:"batch_id"`
	StepIndex int       `json:"step_index"`
	UserID    string    `gorm:"type:uuid" json:"user_id"`
	Action    string    `gorm:"size:16" json:"action"`
	Reason    string    `gorm:"size:512" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (ReviewVote) TableName() string { return "acceptance_review_vote" }

type SignSubject struct {
	types.UUIDModel
	TenantID string `gorm:"type:uuid" json:"tenant_id"`
	BatchID  string `gorm:"type:uuid" json:"batch_id"`
	Code     string `gorm:"size:64" json:"code"`
	Name     string `gorm:"size:128" json:"name"`
	RoleCode string `gorm:"size:64" json:"role_code"`
	External bool   `json:"external"`
	Required bool   `json:"required"`
	Sort     int    `json:"sort"`
}

func (SignSubject) TableName() string { return "acceptance_sign_subject" }

type Signature struct {
	types.UUIDModel
	TenantID       string    `gorm:"type:uuid" json:"tenant_id"`
	BatchID        string    `gorm:"type:uuid" json:"batch_id"`
	Revision       int       `json:"revision"`
	SubjectCode    string    `gorm:"size:64" json:"subject_code"`
	Representative string    `gorm:"size:128" json:"representative"`
	UserID         string    `gorm:"type:uuid" json:"user_id"`
	FileID         string    `gorm:"type:uuid" json:"file_id"`
	ReportSHA256   string    `gorm:"size:64" json:"report_sha256"`
	Status         string    `gorm:"size:16" json:"status"`
	InvalidReason  string    `gorm:"size:512" json:"invalid_reason"`
	SignedAt       time.Time `json:"signed_at"`
}

func (Signature) TableName() string { return "acceptance_signature" }

type ReportFile struct {
	types.UUIDModel
	TenantID     string    `gorm:"type:uuid" json:"tenant_id"`
	BatchID      string    `gorm:"type:uuid" json:"batch_id"`
	Revision     int       `json:"revision"`
	VolumeCode   string    `gorm:"size:64" json:"volume_code"`
	VolumeName   string    `gorm:"size:128" json:"volume_name"`
	Stage        string    `gorm:"size:24" json:"stage"`
	Format       string    `gorm:"size:16" json:"format"`
	FileID       *string   `gorm:"type:uuid" json:"file_id"`
	SHA256       string    `gorm:"size:64" json:"sha256"`
	Status       string    `gorm:"size:16" json:"status"`
	ErrorMessage string    `gorm:"size:512" json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ReportFile) TableName() string { return "acceptance_report_file" }

type HandoverLog struct {
	types.UUIDModel
	TenantID   string    `gorm:"type:uuid" json:"tenant_id"`
	BatchID    string    `gorm:"type:uuid" json:"batch_id"`
	Action     string    `gorm:"size:32" json:"action"`
	ActorID    string    `gorm:"type:uuid" json:"actor_id"`
	FromStatus string    `gorm:"size:24" json:"from_status"`
	ToStatus   string    `gorm:"size:24" json:"to_status"`
	Reason     string    `gorm:"size:512" json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (HandoverLog) TableName() string { return "acceptance_handover_log" }

type ReopenRequest struct {
	types.UUIDModel
	TenantID       string     `gorm:"type:uuid" json:"tenant_id"`
	BatchID        string     `gorm:"type:uuid" json:"batch_id"`
	RequesterID    string     `gorm:"type:uuid" json:"requester_id"`
	ApproverID     *string    `gorm:"type:uuid" json:"approver_id"`
	Reason         string     `gorm:"size:512" json:"reason"`
	Status         string     `gorm:"size:16" json:"status"`
	DecisionReason string     `gorm:"size:512" json:"decision_reason"`
	DecidedAt      *time.Time `json:"decided_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (ReopenRequest) TableName() string { return "acceptance_reopen_request" }
