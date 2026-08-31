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
	SupervisorSignatureKey string `gorm:"size:255" json:"supervisor_signature_key"`
	ManagerSignatureKey    string `gorm:"size:255" json:"manager_signature_key"`
	SealFileKey string         `gorm:"size:255" json:"seal_file_key"`
	FileKey     string         `gorm:"size:255" json:"file_key"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (InspectionReport) TableName() string { return "inspection_report" }
