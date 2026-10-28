package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type ObjectRef struct {
	types.UUIDModel
	TenantID    string    `gorm:"type:uuid" json:"tenant_id"`
	CommunityID string    `gorm:"type:uuid" json:"community_id"`
	FileID      string    `gorm:"type:uuid" json:"file_id"`
	ObjectType  string    `gorm:"size:48" json:"object_type"`
	ObjectID    string    `gorm:"type:uuid" json:"object_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (ObjectRef) TableName() string { return "file_object_ref" }
