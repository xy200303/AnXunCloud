package model

import (
	"time"

	"anxuncloud/internal/pkg/types"
)

type PointConfigBlock struct {
	types.UUIDModel
	TenantID    string     `gorm:"type:uuid" json:"tenant_id"`
	CommunityID string     `gorm:"type:uuid" json:"community_id"`
	PointID     string     `gorm:"type:uuid" json:"point_id"`
	Reason      string     `gorm:"size:255" json:"reason"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
}

func (PointConfigBlock) TableName() string { return "inspection_point_config_block" }

func PointConfigError(point *InspectionPoint) string {
	if point == nil {
		return "点位不存在"
	}
	if point.RequireFence {
		if point.Longitude == 0 && point.Latitude == 0 {
			return "点位已启用围栏但未配置坐标"
		}
		if point.FenceRadius < 10 || point.FenceRadius > 2000 {
			return "点位围栏半径须在 10-2000 米之间"
		}
	}
	switch point.Credential {
	case "qrcode":
		if point.QRCodeNo == "" {
			return "点位使用二维码凭证但未配置二维码"
		}
	case "nfc":
		if point.NfcID == "" {
			return "点位使用 NFC 凭证但未配置 NFC ID"
		}
	case "any":
		if point.QRCodeNo == "" && point.NfcID == "" {
			return "点位允许二维码或 NFC，但未配置任何凭证"
		}
	case "none":
	default:
		return "点位凭证类型配置无效"
	}
	return ""
}
