package contract

import "time"

const (
	ResultAbnormal     = "abnormal"
	ItemResultNormal   = "normal"
	ItemResultAbnormal = "abnormal"
	ItemResultEscaped  = "escaped"
	AuditPending       = "pending"
	AuditPass          = "pass"
	AuditRejected      = "rejected"
	AuditAutoPass      = "auto_pass"
)

type ApprovedFactQuery struct {
	TenantID    string
	CommunityID string
	PatrolType  string
	Start       time.Time
	End         time.Time
}

type PointFact struct {
	ID         string
	Name       string
	Type       string
	QRCodeNo   string
	NfcID      string
	BuildingID *string
	UnitNo     *int
	Floor      *int
}

type RecordFact struct {
	ID          string
	PointID     string
	InspectorID string
	CheckinTime time.Time
	Result      string
	Remark      string
	AuditStatus string
}

type ItemFact struct {
	RecordID      string
	TemplateID    *string
	ItemCode      string
	EquipmentID   *string
	Name          string
	Result        string
	Photos        []string
	AbnormalTags  []string
	ValueNum      *float64
	ValueDate     *time.Time
	ValueText     *string
	Note          string
	ExceptionType string
}

type TaskFact struct {
	ID       string
	PlanID   string
	PointIDs []string
}

type ApprovedFactPage struct {
	Points  []PointFact
	Tasks   []TaskFact
	Records []RecordFact
	Items   []ItemFact
}
