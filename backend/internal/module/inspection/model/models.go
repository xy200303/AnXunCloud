// Package model 巡检业务域 GORM 模型（表结构见《数据库设计文档》§3.2；主键为应用层 UUIDv7）。
package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// 点位打卡方式
const (
	ModeQRCode = "qrcode"
	ModeFence  = "fence"
	ModeEither = "either"
	ModeBoth   = "both"
	ModeNFC    = "nfc"
)

// 任务状态
const (
	TaskPending = "pending"
	TaskDoing   = "doing"
	TaskDone    = "done"
	TaskOverdue = "overdue"
)

// 打卡结果
const (
	ResultNormal   = "normal"
	ResultAbnormal = "abnormal"
)

// 记录审核状态（上传默认通过，打回不级联任务进度）
const (
	AuditAutoPass = "auto_pass"
	AuditPending  = "pending"
	AuditPass     = "pass"
	AuditRejected = "rejected"
)

// 大模型审核结论
const (
	AIVerdictPass   = "pass"
	AIVerdictReview = "review"
	AIVerdictError  = "error"
)

// CheckTemplate 检查项模板（point_type 空为通用）。
type CheckTemplate struct {
	types.UUIDModel
	Name      string `gorm:"size:128" json:"name"`
	PointType string `gorm:"size:32" json:"point_type"`
	// Items 已废弃（v18 起检查项拆分为 check_template_item 独立表；列保留不删，仅存量数据搬迁读取）
	Items     types.TemplateItemArray `gorm:"type:jsonb" json:"items"`
	Sort      int                     `json:"sort"`
	Status    string                  `gorm:"size:16" json:"status"`
	Remark    string                  `gorm:"size:255" json:"remark"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
	DeletedAt gorm.DeletedAt          `json:"-"`
}

func (CheckTemplate) TableName() string { return "check_template" }

// CheckTemplateItem 模板检查项（v18 起独立表；更新模板=事务内整表替换项行）。
type CheckTemplateItem struct {
	types.UUIDModel
	TemplateID string `gorm:"type:uuid" json:"template_id"`
	Name       string `gorm:"size:128" json:"name"`
	// Requirement 检查标准要求文本（可空）
	Requirement   *string   `gorm:"type:text" json:"requirement"`
	Required      bool      `json:"required"`
	PhotoRequired string    `gorm:"size:16" json:"photo_required"` // none/optional/required
	Sort          int       `json:"sort"`
	CreatedAt     time.Time `json:"created_at"`
}

func (CheckTemplateItem) TableName() string { return "check_template_item" }

// Building 楼栋/区域
type Building struct {
	types.UUIDModel
	CommunityID string         `gorm:"type:uuid" json:"community_id"`
	Name        string         `gorm:"size:128" json:"name"`
	Type        string         `gorm:"size:16" json:"type"`
	Sort        int            `json:"sort"`
	Status      string         `gorm:"size:16" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (Building) TableName() string { return "building" }

// InspectionPoint 巡检点位
type InspectionPoint struct {
	types.UUIDModel
	CommunityID        string            `gorm:"type:uuid" json:"community_id"`
	BuildingID         *string           `gorm:"type:uuid" json:"building_id"`
	Name               string            `gorm:"size:128" json:"name"`
	Type               string            `gorm:"size:32" json:"type"`
	QRCodeNo           string            `gorm:"column:qrcode_no;size:64" json:"qrcode_no"`
	NfcID              string            `gorm:"size:64" json:"nfc_id"`
	TemplateID         *string           `gorm:"type:uuid" json:"template_id"`
	Longitude          float64           `gorm:"type:numeric(10,7)" json:"longitude"`
	Latitude           float64           `gorm:"type:numeric(10,7)" json:"latitude"`
	FenceRadius        int               `json:"fence_radius"`
	CheckinMode        string            `gorm:"size:16" json:"checkin_mode"`
	RequiredPhotoItems types.StringArray `gorm:"type:jsonb" json:"required_photo_items"`
	Sort               int               `json:"sort"`
	Status             string            `gorm:"size:16" json:"status"`
	Remark             string            `gorm:"size:255" json:"remark"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
	DeletedAt          gorm.DeletedAt    `json:"-"`
}

func (InspectionPoint) TableName() string { return "inspection_point" }

// InspectionPlan 巡检计划
type InspectionPlan struct {
	types.UUIDModel
	CommunityID  string        `gorm:"type:uuid" json:"community_id"`
	Name         string        `gorm:"size:128" json:"name"`
	PointIDs     types.IDArray `gorm:"type:jsonb" json:"point_ids"`
	CycleType    string        `gorm:"size:16" json:"cycle_type"`
	CycleConfig  types.JSONMap `gorm:"type:jsonb" json:"cycle_config"`
	InspectorIDs types.IDArray `gorm:"type:jsonb" json:"inspector_ids"`
	StartDate    time.Time     `gorm:"type:date" json:"start_date"`
	EndDate      *time.Time    `gorm:"type:date" json:"end_date"`
	TimeWindow   string        `gorm:"size:32" json:"time_window"`
	Status       string        `gorm:"size:16" json:"status"`
	Remark       string        `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-"`
}

func (InspectionPlan) TableName() string { return "inspection_plan" }

// InspectionTask 巡检任务
type InspectionTask struct {
	types.UUIDModel
	PlanID      string         `gorm:"type:uuid" json:"plan_id"`
	CommunityID string         `gorm:"type:uuid" json:"community_id"`
	InspectorID string         `gorm:"type:uuid" json:"inspector_id"`
	TaskDate    time.Time      `gorm:"type:date" json:"task_date"`
	Status      string         `gorm:"size:16" json:"status"`
	TotalPoints int            `json:"total_points"`
	DonePoints  int            `json:"done_points"`
	StartedAt   *time.Time     `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (InspectionTask) TableName() string { return "inspection_task" }

// CheckinRecord 打卡记录（按月分区；主键 id+created_at；id 支持客户端 UUIDv7 幂等写入）
type CheckinRecord struct {
	types.UUIDModel
	TaskID          string              `gorm:"type:uuid" json:"task_id"`
	PointID         string              `gorm:"type:uuid" json:"point_id"`
	InspectorID     string              `gorm:"type:uuid" json:"inspector_id"`
	CommunityID     string              `gorm:"type:uuid" json:"community_id"`
	CheckinTime     time.Time           `json:"checkin_time"`
	ClientTime      *time.Time          `json:"client_time"`
	Longitude       *float64            `gorm:"type:numeric(10,7)" json:"longitude"`
	Latitude        *float64            `gorm:"type:numeric(10,7)" json:"latitude"`
	DistanceToPoint *float64            `gorm:"type:numeric(10,2)" json:"distance_to_point"`
	CheckinType     string              `gorm:"size:16" json:"checkin_type"`
	Photos          types.PhotoArray    `gorm:"type:jsonb" json:"photos"`
	// CheckItems 已废弃（v18 起逐项结果拆分为 checkin_record_item 独立表；列保留不删，仅存量数据搬迁读取）
	CheckItems      types.CheckItemArray `gorm:"type:jsonb" json:"check_items"`
	Result          string              `gorm:"size:16" json:"result"`
	Remark          string              `gorm:"size:512" json:"remark"`
	IsOfflineSync   bool                `json:"is_offline_sync"`
	IsSuspect       bool                `json:"is_suspect"`
	SuspectReason   string              `gorm:"size:255" json:"suspect_reason"`
	AuditStatus     string              `gorm:"size:16" json:"audit_status"`
	AuditBy         *string             `gorm:"type:uuid" json:"audit_by"`
	AuditAt         *time.Time          `json:"audit_at"`
	AuditRemark     string              `gorm:"size:512" json:"audit_remark"`
	AIVerdict       string              `gorm:"size:16" json:"ai_verdict"`
	AIReason        string              `gorm:"size:512" json:"ai_reason"`
	CreatedAt       time.Time           `gorm:"primaryKey" json:"created_at"`
}

func (CheckinRecord) TableName() string { return "checkin_record" }

// CheckinRecordItem 打卡逐项结果快照（v18 起独立表）。
// 快照语义：name/requirement/photo_required 在打卡当时从模板项复制，历史记录内容绝不依赖 join 模板表；
// template_item_id 仅作可空血缘字段（统计用）。record_id 不加 FK（checkin_record 为按月分区表，
// 其主键含分区键 created_at，FK 无法仅引用 id），以普通索引 + 应用层同事务写入保证一致。
type CheckinRecordItem struct {
	types.UUIDModel
	RecordID       string  `gorm:"type:uuid" json:"record_id"`
	TemplateItemID *string `gorm:"type:uuid" json:"template_item_id"`
	Name           string  `gorm:"size:128" json:"name"`
	Requirement    *string `gorm:"type:text" json:"requirement"`
	PhotoRequired  string  `gorm:"size:16" json:"photo_required"` // none/optional/required
	Pass           bool    `json:"pass"`
	Note           string  `gorm:"size:512" json:"note"`
	// Photos 该项照片 file_key 数组（JSONB，不再拆表）
	Photos types.StringArray `gorm:"type:jsonb" json:"photos"`
	// AIVerdict/AIReason 逐项大模型结论（预留；模型未返回逐项结论时为空）
	AIVerdict *string   `gorm:"size:16" json:"ai_verdict"`
	AIReason  *string   `gorm:"size:512" json:"ai_reason"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
}

func (CheckinRecordItem) TableName() string { return "checkin_record_item" }
