// Package dto 统计模块请求结构。
package dto

import "property-inspection/internal/pkg/response"

// ReportQuery 报表公共筛选参数。
type ReportQuery struct {
	StartDate   string `form:"start_date" binding:"required"`
	EndDate     string `form:"end_date" binding:"required"`
	CommunityID int64  `form:"community_id"`
}

type PerformanceQuery struct {
	ReportQuery
	response.PageQuery
	SortBy    string `form:"sort_by"`
	SortOrder string `form:"sort_order"`
}

type ExportReq struct {
	ReportType  string `json:"report_type" binding:"required,oneof=coverage timeliness performance monthly operation_log login_log"`
	Format      string `json:"format" binding:"required,oneof=excel pdf"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date" binding:"required"`
	CommunityID int64  `json:"community_id"`
}
