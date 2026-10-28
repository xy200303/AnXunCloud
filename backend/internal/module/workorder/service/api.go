package service

import "anxuncloud/internal/pkg/types"

type CreateFromInspectionReq struct {
	TenantID       string
	CommunityID    string
	RecordID       string
	SourceRevision int
	OperatorID     string
	Location       string
	Items          []CreateFromInspectionItem
}

type CreateFromInspectionItem struct {
	ItemID      string
	EquipmentID *string
	Title       string
	Problem     string
	Photos      types.StringArray
}
