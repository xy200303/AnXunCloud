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
}

type GenerateReq struct {
	CommunityID string `json:"community_id" binding:"required"`
	Period      string `json:"period" binding:"required"` // YYYY-MM
	// 指定签字人：nil=自动圈选全部候选人；显式数组（含空数组）=按所给名单，空名单该级自动跳过
	SupervisorIDs []string `json:"supervisor_ids"`
	ManagerIDs    []string `json:"manager_ids"`
}

// SignReq 主管/经理签批请求（action=approve 通过 / reject 驳回，驳回 reason 必填）。
// signature_file_key：未配置手写签名时随请求提交的一次性签名（须本人 scene=signature 上传文件，仅本次签字快照用）。
type SignReq struct {
	Remark           string `json:"remark"`
	Action           string `json:"action" binding:"required,oneof=approve reject"`
	Reason           string `json:"reason"`
	SignatureFileKey string `json:"signature_file_key"`
}

// InspectorSignReq 巡检员确认请求：空 body 为本人确认；proxy_for 非空为代签（须 report:sign:proxy 权限，reason 必填）。
// signature_file_key 同 SignReq（本人签/代签人均可用一次性签名）。
type InspectorSignReq struct {
	ProxyFor         string `json:"proxy_for"`
	Reason           string `json:"reason"`
	SignatureFileKey string `json:"signature_file_key"`
}
