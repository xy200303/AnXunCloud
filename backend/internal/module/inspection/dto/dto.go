// Package dto 巡检模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

// ========== 点位 ==========

type PointListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	BuildingID  string `form:"building_id"`
	Name        string `form:"name"`
	Type        string `form:"type"`
	CheckinMode string `form:"checkin_mode"`
	Status      *int   `form:"status"`
}

type PointSaveReq struct {
	CommunityID        string   `json:"community_id" binding:"required"`
	BuildingID         *string  `json:"building_id"`
	Name               string   `json:"name" binding:"required"`
	Type               string   `json:"type" binding:"required"`
	Longitude          float64  `json:"longitude" binding:"required"`
	Latitude           float64  `json:"latitude" binding:"required"`
	FenceRadius        int      `json:"fence_radius"`
	CheckinMode        string   `json:"checkin_mode"`
	RequiredPhotoItems []string `json:"required_photo_items"`
	Sort               int      `json:"sort"`
	Status             *int     `json:"status"`
	Remark             string   `json:"remark"`
}

type QRCodeBatchReq struct {
	PointIDs  []string `json:"point_ids" binding:"required,min=1"`
	WithTitle *bool   `json:"with_title"`
}

// ========== 计划 ==========

type PlanListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Name        string `form:"name"`
	CycleType   string `form:"cycle_type"`
	Status      *int   `form:"status"`
}

type PlanSaveReq struct {
	CommunityID  string         `json:"community_id" binding:"required"`
	Name         string         `json:"name" binding:"required"`
	PointIDs     []string       `json:"point_ids" binding:"required,min=1"`
	CycleType    string         `json:"cycle_type" binding:"required,oneof=daily weekly monthly"`
	CycleConfig  map[string]any `json:"cycle_config"`
	InspectorIDs []string       `json:"inspector_ids" binding:"required,min=1"`
	StartDate    string         `json:"start_date" binding:"required"`
	EndDate      string         `json:"end_date"`
	TimeWindow   string         `json:"time_window" binding:"required"`
	Status       *int           `json:"status"`
	Remark       string         `json:"remark"`
}

// ========== 任务 ==========

type TaskListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	InspectorID string `form:"inspector_id"`
	PlanID      string `form:"plan_id"`
	TaskDate    string `form:"task_date"`
	StartDate   string `form:"start_date"`
	EndDate     string `form:"end_date"`
	Status      string `form:"status"`
	Filter      string `form:"filter"`
}

type GenerateTaskReq struct {
	Date string `json:"date"` // 缺省为今天
}

// ========== 打卡记录 ==========

type CheckinListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	PointID     string `form:"point_id"`
	InspectorID string `form:"inspector_id"`
	TaskID      string `form:"task_id"`
	Result      string `form:"result"`
	CheckinType string `form:"checkin_type"`
	IsSuspect   *bool  `form:"is_suspect"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}
