package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type StylePack struct {
	types.UUIDModel
	Code      string        `gorm:"size:64;uniqueIndex" json:"code"`
	Name      string        `gorm:"size:128" json:"name"`
	Config    types.JSONMap `gorm:"type:jsonb" json:"config"`
	Status    string        `gorm:"size:16" json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

func (StylePack) TableName() string { return "report_style_pack" }
