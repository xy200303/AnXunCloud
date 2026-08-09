// Package dto 月度报告模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type ReportListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Period      string `form:"period"` // YYYY-MM
	Status      string `form:"status"`
}

type GenerateReq struct {
	CommunityID string `json:"community_id" binding:"required"`
	Period      string `json:"period" binding:"required"` // YYYY-MM
}

// SignReq 主管/经理签批请求（action=approve 通过 / reject 驳回，驳回 reason 必填）。
type SignReq struct {
	Remark string `json:"remark"`
	Action string `json:"action" binding:"required,oneof=approve reject"`
	Reason string `json:"reason"`
}
