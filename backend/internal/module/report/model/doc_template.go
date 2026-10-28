package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type DocTemplate struct {
	types.UUIDModel
	TenantID       *string       `gorm:"type:uuid" json:"tenant_id"`
	Code           string        `gorm:"size:64" json:"code"`
	Name           string        `gorm:"size:128" json:"name"`
	DocKind        string        `gorm:"size:24" json:"doc_kind"`
	Engine         string        `gorm:"size:16" json:"engine"`
	Format         string        `gorm:"size:16" json:"format"`
	Layout         types.JSONMap `gorm:"type:jsonb" json:"layout"`
	StylePackID    *string       `gorm:"type:uuid" json:"style_pack_id"`
	SkeletonFileID *string       `gorm:"type:uuid" json:"skeleton_file_id"`
	VolumeCode     string        `gorm:"size:64" json:"volume_code"`
	SystemFilter   string        `gorm:"size:64" json:"system_filter"`
	Status         string        `gorm:"size:16" json:"status"`
	PublishedAt    *time.Time    `json:"published_at"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (DocTemplate) TableName() string { return "doc_template" }

type TemplateOutput struct {
	types.UUIDModel
	TenantID      string        `gorm:"type:uuid" json:"tenant_id"`
	DocTemplateID string        `gorm:"type:uuid" json:"doc_template_id"`
	TemplateID    string        `gorm:"type:uuid" json:"template_id"`
	Sort          int           `json:"sort"`
	Columns       types.JSONMap `gorm:"type:jsonb" json:"columns"`
	RowMode       string        `gorm:"size:16" json:"row_mode"`
}

func (TemplateOutput) TableName() string { return "template_output" }
