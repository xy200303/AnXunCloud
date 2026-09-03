// Package model 月度报告域 GORM 模型（迁移 v10 建表；主键为应用层 UUIDv7）。
package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// 报告状态：pending_inspector → pending_supervisor → pending_manager → approved；驳回回 pending_inspector
const (
	StatusPendingInspector  = "pending_inspector"
	StatusPendingSupervisor = "pending_supervisor"
	StatusPendingManager    = "pending_manager"
	StatusApproved          = "approved"
)

// InspectionReport 月度巡检报告（三级电子确认签字 + PDF 归档）。
type InspectionReport struct {
	types.UUIDModel
	TenantID    *string `gorm:"type:uuid" json:"tenant_id"` // 冗余列（=所属小区租户）
	CommunityID string  `gorm:"type:uuid" json:"community_id"`
	Period      string  `gorm:"size:7" json:"period"`
	// PatrolType 巡查类型（字典 patrol_type 的 value）；空=综合月报（全类型口径），非空=该类型专项检查报告
	PatrolType      string          `gorm:"size:32" json:"patrol_type"`
	PlanID          *string         `gorm:"type:uuid" json:"plan_id"` // 溯源生成它的巡检计划（综合月报为空）
	// ReportPlanID 溯源生成它的报告生成计划（手动生成的报告为空）
	ReportPlanID *string `gorm:"type:uuid" json:"report_plan_id"`
	// DetailMode 明细策略：full=全量点位（每点位一行）/ abnormal=仅异常点位（报告厚度控制）
	DetailMode string `gorm:"size:16;default:full" json:"detail_mode"`
	// PeriodStart/PeriodEnd 报告期间（含头含尾；月报=整月，周报=周一~周日，日报=当天）。
	// period 字符串保留作展示（月 2026-08 / 周 2026-08-31~09-06 / 日 2026-09-01）。
	PeriodStart *time.Time `json:"period_start"`
	PeriodEnd   *time.Time `json:"period_end"`
	Title           string          `gorm:"size:128" json:"title"`
	Status          string          `gorm:"size:24" json:"status"`
	Stats           types.JSONMap   `gorm:"type:jsonb" json:"stats"`
	InspectorIDs    types.IDArray   `gorm:"type:jsonb" json:"inspector_ids"`
	InspectorSigned types.SignArray `gorm:"type:jsonb" json:"inspector_signed"`
	// 指定签字人名单（v17；生成时圈定，空=该级无人可签自动跳过，PDF 签字栏留空）
	SupervisorIDs    types.IDArray `gorm:"type:jsonb" json:"supervisor_ids"`
	ManagerIDs       types.IDArray `gorm:"type:jsonb" json:"manager_ids"`
	SupervisorBy     *string       `gorm:"type:uuid" json:"supervisor_by"`
	SupervisorAt     *time.Time    `json:"supervisor_at"`
	SupervisorRemark string        `gorm:"size:512" json:"supervisor_remark"`
	ManagerBy        *string       `gorm:"type:uuid" json:"manager_by"`
	ManagerAt        *time.Time    `json:"manager_at"`
	ManagerRemark    string        `gorm:"size:512" json:"manager_remark"`
	RejectReason     string        `gorm:"size:512" json:"reject_reason"`
	// 主管/经理签字时的手写签名图快照（巡检员快照在 InspectorSigned 元素内）
	// default:null——空串走数据库 DEFAULT NULL，避免 '' 插入 uuid 列报错（00032 迁移后这些列是 uuid）
	SupervisorSignatureID string `gorm:"type:uuid;default:null" json:"supervisor_signature_id"`
	ManagerSignatureID    string `gorm:"type:uuid;default:null" json:"manager_signature_id"`
	SealFileID string         `gorm:"type:uuid;default:null" json:"seal_file_id"`
	FileID     string         `gorm:"type:uuid;default:null" json:"file_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (InspectionReport) TableName() string { return "inspection_report" }

// ReportPlan 报告生成计划：周期驱动自动生成巡检报告（cycle 定义与巡检计划同构）。
// 生成期间一律取「上一个完整周期」：monthly=上月（cycle_config.day=生成日 1~28）、
// weekly=上周一至周日（cycle_config.weekday=生成星期几 1~7）、daily=昨日。
type ReportPlan struct {
	types.UUIDModel
	TenantID    *string       `gorm:"type:uuid" json:"tenant_id"` // 冗余列（=所属小区租户）
	CommunityID string        `gorm:"type:uuid" json:"community_id"`
	Name        string        `gorm:"size:64" json:"name"`
	PatrolType  string        `gorm:"size:32" json:"patrol_type"` // 空=综合
	CycleType   string        `gorm:"size:16" json:"cycle_type"`  // daily/weekly/monthly
	CycleConfig types.JSONMap `gorm:"type:jsonb" json:"cycle_config"`
	GenTime     string        `gorm:"size:8" json:"gen_time"` // 生成时点 HH:MM（默认 06:00）
	DetailMode  string        `gorm:"size:16;default:full" json:"detail_mode"` // 明细策略：full/abnormal
	Status      string        `gorm:"size:16" json:"status"`
	LastPeriod  string        `gorm:"size:32" json:"last_period"` // 上次生成的期间展示串
	LastError   string        `gorm:"size:255" json:"last_error"`
	Remark      string        `gorm:"size:255" json:"remark"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (ReportPlan) TableName() string { return "report_plan" }
