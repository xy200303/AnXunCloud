package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

const (
	ScopePlatform    = "platform"
	ScopeTenant      = "tenant"
	PlatformScopeID  = "00000000-0000-0000-0000-000000000000"
	StatusDraft      = "draft"
	StatusInspecting = "inspecting"
	StatusReviewing  = "reviewing"
	StatusHandover   = "handover"
	StatusDone       = "done"
	Enabled          = "enabled"
	Disabled         = "disabled"
)

type ListSchema struct {
	types.UUIDModel
	Scope      string    `gorm:"size:16" json:"scope"`
	ScopeID    string    `gorm:"type:uuid" json:"scope_id"`
	Code       string    `gorm:"size:64" json:"code"`
	Category   string    `gorm:"size:32" json:"category"`
	Name       string    `gorm:"size:128" json:"name"`
	TableKind  string    `gorm:"size:24" json:"table_kind"`
	TemplateID *string   `gorm:"type:uuid" json:"template_id"`
	Status     string    `gorm:"size:16" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ListSchema) TableName() string { return "acceptance_list_schema" }

type ListRow struct {
	types.UUIDModel
	Scope         string    `gorm:"size:16" json:"scope"`
	ScopeID       string    `gorm:"type:uuid" json:"scope_id"`
	TableCode     string    `gorm:"size:64" json:"table_code"`
	Code          string    `gorm:"size:64" json:"code"`
	Name          string    `gorm:"size:128" json:"name"`
	Specification string    `gorm:"size:255" json:"specification"`
	Unit          string    `gorm:"size:16" json:"unit"`
	Required      bool      `json:"required"`
	Sort          int       `json:"sort"`
	Status        string    `gorm:"size:16" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ListRow) TableName() string { return "acceptance_list_row" }

type Batch struct {
	types.UUIDModel
	TenantID         string              `gorm:"type:uuid" json:"tenant_id"`
	CommunityID      string              `gorm:"type:uuid" json:"community_id"`
	Name             string              `gorm:"size:128" json:"name"`
	LedgerType       string              `gorm:"size:32" json:"ledger_type"`
	PeriodStart      time.Time           `json:"period_start"`
	PeriodEnd        time.Time           `json:"period_end"`
	ManagerID        string              `gorm:"type:uuid" json:"manager_id"`
	Status           string              `gorm:"size:24" json:"status"`
	Revision         int                 `json:"revision"`
	ReviewFlow       types.FlowStepArray `gorm:"type:jsonb" json:"review_flow"`
	ReviewStep       int                 `json:"review_step"`
	TemplateSnapshot types.JSONMap       `gorm:"type:jsonb" json:"template_snapshot"`
	LockedAt         *time.Time          `json:"locked_at"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

func (Batch) TableName() string { return "acceptance_batch" }

type Target struct {
	types.UUIDModel
	TenantID          string            `gorm:"type:uuid" json:"tenant_id"`
	BatchID           string            `gorm:"type:uuid" json:"batch_id"`
	TargetType        string            `gorm:"size:24" json:"target_type"`
	RefID             string            `gorm:"size:64" json:"ref_id"`
	BuildingID        *string           `gorm:"type:uuid" json:"building_id"`
	CategoryCode      string            `gorm:"size:64" json:"category_code"`
	TableCodes        types.StringArray `gorm:"type:jsonb" json:"table_codes"`
	Name              string            `gorm:"size:128" json:"name"`
	Location          string            `gorm:"size:255" json:"location"`
	CoverageConfirmed bool              `json:"coverage_confirmed"`
	CoveredBy         *string           `gorm:"type:uuid" json:"covered_by"`
	CoveredAt         *time.Time        `json:"covered_at"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

func (Target) TableName() string { return "acceptance_target" }

type Member struct {
	types.UUIDModel
	TenantID      string            `gorm:"type:uuid" json:"tenant_id"`
	BatchID       string            `gorm:"type:uuid" json:"batch_id"`
	UserID        string            `gorm:"type:uuid" json:"user_id"`
	WorkScope     string            `gorm:"size:16" json:"work_scope"`
	CategoryCodes types.StringArray `gorm:"type:jsonb" json:"category_codes"`
	BuildingIDs   types.StringArray `gorm:"type:jsonb" json:"building_ids"`
	ExternalName  string            `gorm:"size:128" json:"external_name"`
	CreatedAt     time.Time         `json:"created_at"`
}

func (Member) TableName() string { return "acceptance_member" }

type Record struct {
	types.UUIDModel
	TenantID     string            `gorm:"type:uuid" json:"tenant_id"`
	BatchID      string            `gorm:"type:uuid" json:"batch_id"`
	TargetID     string            `gorm:"type:uuid" json:"target_id"`
	TableCode    string            `gorm:"size:64" json:"table_code"`
	ItemCode     string            `gorm:"size:64" json:"item_code"`
	CategoryCode string            `gorm:"size:64" json:"category_code"`
	Result       string            `gorm:"size:24" json:"result"`
	ValueText    string            `gorm:"type:text" json:"value_text"`
	Values       types.JSONMap     `gorm:"column:value_map;type:jsonb" json:"values"`
	Note         string            `gorm:"type:text" json:"note"`
	Photos       types.StringArray `gorm:"type:jsonb" json:"photos"`
	Revision     int               `json:"revision"`
	CreatedBy    string            `gorm:"type:uuid" json:"created_by"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func (Record) TableName() string { return "acceptance_record" }

type Problem struct {
	types.UUIDModel
	TenantID         string            `gorm:"type:uuid" json:"tenant_id"`
	BatchID          string            `gorm:"type:uuid" json:"batch_id"`
	RecordID         string            `gorm:"type:uuid;unique" json:"record_id"`
	Fact             string            `gorm:"type:text" json:"fact"`
	Location         string            `gorm:"size:255" json:"location"`
	Photos           types.StringArray `gorm:"type:jsonb" json:"photos"`
	RiskLevel        string            `gorm:"size:16;default:normal" json:"risk_level"`
	SafetyMeasure    string            `gorm:"type:text" json:"safety_measure"`
	MeasureApproved  bool              `json:"measure_approved"`
	Accepted         bool              `json:"accepted"`
	AcceptedBy       string            `gorm:"size:128" json:"accepted_by"`
	AcceptedAt       *time.Time        `json:"accepted_at"`
	AcceptanceFileID *string           `gorm:"type:uuid" json:"acceptance_file_id"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

func (Problem) TableName() string { return "acceptance_problem" }
