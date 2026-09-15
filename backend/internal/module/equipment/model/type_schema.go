package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// EquipmentTypeSchema 设备类型字段方案（《巡检与台账优化方案（9.14实测反馈）》§六 类型化台账）：
// 按设备类型定义 列表列/导出列/表单字段/导入列头 四样配置（config jsonb）；
// TenantID NULL=平台默认方案，租户级方案整体覆盖平台默认（读取租户优先、缺省回落）。
type EquipmentTypeSchema struct {
	types.UUIDModel
	TenantID  *string        `gorm:"type:uuid" json:"tenant_id"` // NULL=平台默认方案
	Type      string         `gorm:"size:64" json:"type"`        // equipment_type 字典 value
	Config    types.JSONMap  `gorm:"type:jsonb;default:'{}'" json:"config"`
	Status    string         `gorm:"size:16;default:enabled" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (EquipmentTypeSchema) TableName() string { return "equipment_type_schema" }
