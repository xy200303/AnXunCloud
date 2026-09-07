// Package dto 月度报告模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

type ReportListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Period      string `form:"period"` // YYYY-MM
	Status      string `form:"status"`
	PendingMine string `form:"pending_mine"` // 1/true = 只看待我签（当前用户在当前级签字人名单内）
	// 1/true = 我签过的 + 已归档；doing = 我签过但流程未走完（进行中）
	SignedMine string `form:"signed_mine"`
	// 巡查类型筛选：空=全部；none=综合月报（patrol_type 为空）；其余=字典 patrol_type 的 value
	PatrolType string `form:"patrol_type"`
}

// ReportRecordsQuery 报告打卡明细分页（仅分页参数）。
type ReportRecordsQuery struct {
	response.PageQuery
}

// GenerateReq 手动生成报告：period（月度 YYYY-MM）或 start_date+end_date（任意期间）二选一
// （start_date/end_date 同时提供时优先于 period，此时 period 仅作展示兜底，可传空）。
type GenerateReq struct {
	CommunityID string `json:"community_id" binding:"required"`
	Period      string `json:"period"`      // YYYY-MM（未给日期范围时必填）
	StartDate   string `json:"start_date"`  // 任意期间起（含，YYYY-MM-DD）
	EndDate     string `json:"end_date"`    // 任意期间止（含，YYYY-MM-DD）
	DetailMode  string `json:"detail_mode"` // 明细策略：full/abnormal（缺省 full）
	// 巡查类型：空=综合月报（现状）；非空=该类型专项检查报告（须为字典 patrol_type 的启用项）
	PatrolType string `json:"patrol_type"`
	PlanID     string `json:"plan_id"` // 溯源计划（可空；非空须为该小区下的巡检计划）
	// sign_steps 为空时按报告审核链自动圈选；非空时按步骤指定候选人。
	SignSteps []GenerateSignStep `json:"sign_steps"`
}

type GenerateSignStep struct {
	Slot         string   `json:"slot"`
	CandidateIDs []string `json:"candidate_ids"`
}

// SignReq 动态审核步骤签批请求（action=approve 通过 / reject 驳回，驳回 reason 必填）。
// signature_file_id：未配置手写签名时随请求提交的一次性签名（须本人 scene=signature 上传文件，仅本次签字快照用）。
type SignReq struct {
	Remark          string `json:"remark"`
	Action          string `json:"action" binding:"required,oneof=approve reject"`
	Reason          string `json:"reason"`
	SignatureFileID string `json:"signature_file_id"`
	ProxyFor        string `json:"proxy_for"`
}

// ReportPlanReq 报告生成计划创建/更新请求。
// cycle_config 按 cycle_type 取：monthly={day:1~28}；weekly={weekday:1~7（1=周一）}；daily={}。
type ReportPlanReq struct {
	CommunityID string         `json:"community_id" binding:"required"`
	Name        string         `json:"name" binding:"required,max=64"`
	PatrolType  string         `json:"patrol_type"` // 空=综合
	CycleType   string         `json:"cycle_type" binding:"required,oneof=daily weekly monthly"`
	CycleConfig map[string]any `json:"cycle_config"`
	GenTime     string         `json:"gen_time" binding:"required"` // HH:MM
	DetailMode  string         `json:"detail_mode"`                 // 明细策略：full=全量点位 / abnormal=仅异常点位（缺省 full）
	Status      string         `json:"status"`                      // 更新时可传 enabled/disabled
	Remark      string         `json:"remark" binding:"max=255"`
}
