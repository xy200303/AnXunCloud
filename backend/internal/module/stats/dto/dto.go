// Package dto 统计模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

// ReportQuery 报表公共筛选参数。
type ReportQuery struct {
	StartDate   string `form:"start_date" binding:"required"`
	EndDate     string `form:"end_date" binding:"required"`
	CommunityID string `form:"community_id"`
}

type PerformanceQuery struct {
	ReportQuery
	response.PageQuery
	SortBy    string `form:"sort_by"`
	SortOrder string `form:"sort_order"`
}

// PatrolRoundsQuery 巡更达成率查询参数（community_id 必填，plan_id 可空=该小区全部带轮次的计划）。
type PatrolRoundsQuery struct {
	CommunityID string `form:"community_id" binding:"required"`
	PlanID      string `form:"plan_id"`
	From        string `form:"from" binding:"required"`
	To          string `form:"to" binding:"required"`
}

type ExportReq struct {
	ReportType  string `json:"report_type" binding:"required,oneof=coverage timeliness performance monthly operation_log login_log"`
	Format      string `json:"format" binding:"required,oneof=excel pdf"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date" binding:"required"`
	CommunityID string `json:"community_id"`
}
