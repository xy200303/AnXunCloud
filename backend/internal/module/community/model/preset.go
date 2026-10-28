package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type CommunityPresetPack struct {
	types.UUIDModel
	Code        string        `gorm:"size:64;uniqueIndex" json:"code"`
	Name        string        `gorm:"size:128" json:"name"`
	Description string        `gorm:"size:500" json:"description"`
	Payload     types.JSONMap `gorm:"type:jsonb" json:"payload"`
	Status      string        `gorm:"size:16" json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

func (CommunityPresetPack) TableName() string { return "community_preset_pack" }
