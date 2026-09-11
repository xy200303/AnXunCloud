// Package model 设备台账与维保周期管理数据模型（《设备台账与维保周期管理设计方案》v1.4）。
package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// 设备状态
const (
	StatusInService   = "in_service"  // 在用
	StatusMaintaining = "maintaining" // 维保中
	StatusStopped     = "stopped"     // 停用
	StatusScrapped    = "scrapped"    // 报废
)

// 维保类型
const (
	MaintenanceRepair  = "repair"     // 维修充粉
	MaintenanceKeep    = "maintain"   // 保养
	MaintenanceInspect = "inspect"    // 检测
	MaintenanceReplace = "replace"    // 更换
	MaintenanceLedger  = "ledger_fix" // 台账补录（首轮巡检补齐缺失日期，走同一确认链）
)

// 确认状态
const (
	ConfirmPending   = "pending"
	ConfirmConfirmed = "confirmed"
	ConfirmRejected  = "rejected"
)

// AI 预检结论
const (
	AIVerdictPass   = "pass"
	AIVerdictReview = "review"
)

// 维保流水来源
const (
	SourceForm    = "form"    // 登记表单（mp/admin Register）
	SourceCheckin = "checkin" // 打卡拍新标签自动生成
)

// 确认方式
const (
	ConfirmModeManual = "manual" // 经理人工确认
	ConfirmModeAI     = "ai"     // AI 核验可信自动确认（confirmed_by 置空）
	ConfirmModeAuto   = "auto"   // 空审核链默认直通（confirmed_by 置空）
)

// Equipment 设备台账：一具设备一行记当前状态；到期判定只看 NextDueDate。
// 台账只被 confirmed 维保流水回写（唯一写入口），或后台直接编辑/人工覆盖到期日。
type Equipment struct {
	types.UUIDModel
	TenantID           *string        `gorm:"type:uuid" json:"tenant_id"` // 冗余列（=所属小区租户）
	CommunityID        string         `gorm:"type:uuid" json:"community_id"`
	BuildingID         *string        `gorm:"type:uuid" json:"building_id"`
	PointID            *string        `gorm:"type:uuid" json:"point_id"` // 关联巡检点位（一点多具；NULL=未关联，不参与打卡判定）
	Type               string         `gorm:"size:32" json:"type"`       // equipment_type 字典值
	Code               string         `gorm:"size:64" json:"code"`       // 设备编号（租户内唯一，按具/台登记）
	Name               string         `gorm:"size:128" json:"name"`
	ManufactureDate    *time.Time     `gorm:"type:date" json:"manufacture_date"`      // 出厂日期（瓶体钢印），可空
	LastMaintenanceDate  *time.Time     `gorm:"type:date" json:"last_maintenance_date"` // 最近维保日期（仅 confirmed 流水回写）
	NextDueDate        *time.Time     `gorm:"type:date" json:"next_due_date"`         // 下次到期日（规则计算，可人工覆盖；NULL=无规则/无日期数据）
	ScrapDate          *time.Time     `gorm:"type:date" json:"scrap_date"`            // 报废日期（自动计算展示）
	WarnDays           *int           `json:"warn_days"`                              // 临期阈值覆盖（NULL 用全局配置）
	LastNotifiedAt     *time.Time     `json:"last_notified_at"`                       // 上次提醒时间（防打扰）
	LabelMissing       bool           `gorm:"default:false" json:"label_missing"`     // 标签缺失/无法辨认：确认链打上/清除；true=退出自动到期判定与抽查
	Status             string         `gorm:"size:16;default:in_service" json:"status"`
	Extra              types.JSONMap  `gorm:"type:jsonb;default:'{}'" json:"extra"` // 类型特有属性口袋（充装量/载重等）
	Remark             string         `gorm:"size:255" json:"remark"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-"`
}

func (Equipment) TableName() string { return "equipment" }

// EquipmentMaintenance 维保流水：一次维保一行，只增不改；台账唯一写入口（confirmed 才回写）。
type EquipmentMaintenance struct {
	types.UUIDModel
	TenantID        *string       `gorm:"type:uuid" json:"tenant_id"`
	EquipmentID     string        `gorm:"type:uuid" json:"equipment_id"`
	MaintenanceType string        `gorm:"size:16;default:repair" json:"maintenance_type"` // repair/maintain/inspect/replace/ledger_fix
	MaintenanceDate time.Time     `gorm:"type:date" json:"maintenance_date"`
	Vendor          *string       `gorm:"size:128" json:"vendor"`
	OperatorName    string        `gorm:"size:64" json:"operator_name"` // 经办人（默认登记人）
	Note            string        `gorm:"size:255" json:"note"`
	FileIDs         types.IDArray `gorm:"type:jsonb;default:'[]'" json:"file_ids"` // 新维修标签照片（必传）
	LabelMissing    bool          `gorm:"default:false" json:"label_missing"`      // 标签缺失登记（照片拍设备本体，日期免填；确认后设备打标）
	ConfirmStatus   string        `gorm:"size:16;default:pending" json:"confirm_status"`
	ConfirmedBy     *string       `gorm:"type:uuid" json:"confirmed_by"`
	ConfirmedAt     *time.Time    `json:"confirmed_at"`
	RejectReason    *string       `gorm:"size:255" json:"reject_reason"`
	AIVerdict       *string       `gorm:"size:16" json:"ai_verdict"` // pass/review（只标记不拦截）
	AIReason        *string       `gorm:"size:512" json:"ai_reason"`
	Payload         types.JSONMap `gorm:"type:jsonb;default:'{}'" json:"payload"` // 随单附加数据（ledger_fix 补录日期等）
	// Source 流水来源：form=登记表单 / checkin=打卡拍新标签自动生成（打卡融合 v2.0）
	Source string `gorm:"size:16;default:form" json:"source"`
	// CheckinRecordID 来源打卡记录（source=checkin 时非空；checkin_record 为分区表不加 FK）
	CheckinRecordID *string `gorm:"type:uuid" json:"checkin_record_id"`
	// ConfirmMode 确认方式：manual=人工确认 / ai=AI 核验可信自动确认（confirmed_by 置空）/ auto=空审核链默认直通
	ConfirmMode string `gorm:"size:8;default:manual" json:"confirm_mode"`
	// ConfirmStep 维保审核链当前环节下标（approval_flow flow_code=maint_review；与打卡链 audit_step 同语义）
	ConfirmStep int16         `gorm:"default:0" json:"confirm_step"`
	CreatedBy   string        `gorm:"type:uuid" json:"created_by"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

func (EquipmentMaintenance) TableName() string { return "equipment_maintenance" }
