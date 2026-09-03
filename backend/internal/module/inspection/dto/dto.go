// Package dto 巡检模块请求结构。
package dto

import "anxuncloud/internal/pkg/response"

// ========== 检查项模板 ==========

type TemplateListQuery struct {
	response.PageQuery
	Name      string `form:"name"`
	PointType string `form:"point_type"`
	Status    string `form:"status"`
}

type TemplateSaveReq struct {
	Name      string `json:"name" binding:"required"`
	PointType string `json:"point_type"` // 空为通用模板
	Sort      int    `json:"sort"`
	Status    *int   `json:"status"`
	Remark    string `json:"remark"`
}

// TemplateItemSaveReq 项级粒度新增/修改单个检查项。
type TemplateItemSaveReq struct {
	Name         string `json:"name" binding:"required"`
	Required     bool   `json:"required"`
	Requirement  string `json:"requirement"`
	AIHint       string `json:"ai_hint"` // AI 识别要点（可空；空=该项不带识别要点）
	PhotoRequired string `json:"photo_required"`
	// JudgeType 判定类型（非法值服务端归一为 general）；JudgeConfig 判定参数（metric 需 metric/unit/min/max）
	JudgeType   string         `json:"judge_type"`
	JudgeConfig map[string]any `json:"judge_config"`
	// Sort 排序号；新增时缺省（nil）追加到末尾，修改时缺省保持不变
	Sort *int `json:"sort"`
}

// ========== 点位 ==========

type PointListQuery struct {
	response.PageQuery
	TenantID    string `form:"tenant_id"` // 租户过滤仅超管生效，由数据权限中间件校验上下文
	CommunityID string `form:"community_id"`
	BuildingID  string `form:"building_id"`
	Name        string `form:"name"`
	Type        string `form:"type"`
	Credential  string `form:"credential"`
	Status      string `form:"status"`
}

type PointSaveReq struct {
	CommunityID        string   `json:"community_id" binding:"required"`
	BuildingID         *string  `json:"building_id"`
	UnitNo             *int     `json:"unit_no"`     // 单元号（nil=不分单元/非楼栋点位）
	Floor              *int     `json:"floor"`       // 楼层（负数=地下层；nil=非楼栋点位）
	Name               string   `json:"name" binding:"required"`
	Type               string   `json:"type" binding:"required"`
	Longitude          float64  `json:"longitude" binding:"required"`
	Latitude           float64  `json:"latitude" binding:"required"`
	FenceRadius        int      `json:"fence_radius"`
	Credential         string   `json:"credential"`
	RequireFence       bool     `json:"require_fence"`
	TemplateID         *string  `json:"template_id"`
	NfcID              string   `json:"nfc_id"`
	Sort               int      `json:"sort"`
	Status             *int     `json:"status"`
	Remark             string   `json:"remark"`
}

type QRCodeBatchReq struct {
	PointIDs  []string `json:"point_ids" binding:"required,min=1"`
	WithTitle *bool    `json:"with_title"`
}

// PointBatchReq 批量建点：按 楼栋×楼层×每层数量 生成点位（楼栋可空=整个小区下不挂楼栋）。
type PointBatchReq struct {
	CommunityID string   `json:"community_id" binding:"required"`
	BuildingIDs []string `json:"building_ids"`
	UnitFrom    int      `json:"unit_from"`  // 单元起（缺省 1；UnitTo 缺省同 UnitFrom，即单单元）
	UnitTo      int      `json:"unit_to"`
	FloorFrom   int      `json:"floor_from"` // 支持负数（地下层，-1 渲染为 B1）
	FloorTo     int      `json:"floor_to"`
	PerFloor    int      `json:"per_floor"`    // 每层数量，缺省 1
	NamePattern string   `json:"name_pattern" binding:"required"` // 占位符：{building} {unit} {floor} {seq}
	Type        string   `json:"type" binding:"required"`         // 字典 point_type 启用项
	Credential  string   `json:"credential"`
	TemplateID  *string  `json:"template_id"`
	Longitude   float64  `json:"longitude"` // 小区无坐标字段，缺省 0（扫码凭证不依赖围栏）
	Latitude    float64  `json:"latitude"`
}

// PointBatchResult 批量建点结果。
type PointBatchResult struct {
	Created int              `json:"created"`
	Skipped []PointBatchSkip `json:"skipped"`
}

// PointBatchSkip 批量建点跳过明细（同楼栋下同名视为已存在）。
type PointBatchSkip struct {
	Building string `json:"building"`
	Name     string `json:"name"`
	Reason   string `json:"reason"`
}

// PointImportResult 点位导入结果。
type PointImportResult struct {
	Total        int               `json:"total"`
	SuccessCount int               `json:"success_count"`
	FailCount    int               `json:"fail_count"`
	FailDetails  []PointImportFail `json:"fail_details"`
}

// PointImportFail 点位导入失败明细。
type PointImportFail struct {
	Row    int    `json:"row"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ========== 计划 ==========

type PlanListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	Name        string `form:"name"`
	CycleType   string `form:"cycle_type"`
	PatrolType  string `form:"patrol_type"`
	Status      string `form:"status"`
}

type PlanSaveReq struct {
	CommunityID  string         `json:"community_id" binding:"required"`
	Name         string         `json:"name" binding:"required"`
	PatrolType   string         `json:"patrol_type"` // 巡查类型，缺省 safety（安全巡查）
	PointIDs     []string       `json:"point_ids"`   // explicit 模式必填（service 校验）；by_point_types 模式可空
	CycleType    string         `json:"cycle_type" binding:"required,oneof=daily weekly monthly"`
	CycleConfig  map[string]any `json:"cycle_config"`
	InspectorIDs []string       `json:"inspector_ids" binding:"required,min=1"`
	StartDate    string         `json:"start_date" binding:"required"`
	EndDate      string         `json:"end_date"`
	TimeWindow   string         `json:"time_window"` // 未配 rounds 时必填（service 校验）；配了 rounds 可留空
	// SelectionMode 点位圈选模式（缺省 explicit）；PointTypes 为 by_point_types 模式的圈选类型
	SelectionMode string   `json:"selection_mode"`
	PointTypes    []string `json:"point_types"`
	// AssignMode 点位分配方式（缺省 all；split=按执行日均分，仅 weekly/monthly 合法）
	AssignMode string `json:"assign_mode" binding:"omitempty,oneof=all split"`
	Status     *int   `json:"status"`
	Remark     string `json:"remark"`
}

// ========== 任务 ==========

type TaskListQuery struct {
	response.PageQuery
	CommunityID string `form:"community_id"`
	InspectorID string `form:"inspector_id"`
	PlanID      string `form:"plan_id"`
	PatrolType  string `form:"patrol_type"`
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
	// ExceptionType 逐项异常类型过滤（device_missing=设备缺失 / unable_to_capture=无法拍摄；EXISTS 逐项匹配）
	ExceptionType string `form:"exception_type"`
	CheckinType string `form:"checkin_type"`
	IsSuspect   string `form:"is_suspect"`
	AuditStatus string `form:"audit_status"`
	ForceSubmit string `form:"force_submit"`
	AIVerdict   string `form:"ai_verdict"` // 支持逗号多值（如 review,error 查"AI 存疑"合集）
	Keyword     string `form:"keyword"`    // 点位名/备注模糊匹配
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
}


// ========== 记录审核 ==========

type ReviewListQuery struct {
	response.PageQuery
	AuditStatus string `form:"audit_status"`
	CommunityID string `form:"community_id"`
	InspectorID string `form:"inspector_id"`
	StartTime   string `form:"start_time"`
	EndTime     string `form:"end_time"`
	// ID 精确查单条（App 消息深链直达详情用；与其他过滤条件叠加）
	ID string `form:"id"`
}

type ReviewRejectReq struct {
	Reason string `json:"reason" binding:"required"`
}

// BatchPassReq 批量通过：仅 pending 记录会被置为 pass，其余计入 skipped。
type BatchPassReq struct {
	IDs []string `json:"ids" binding:"required,min=1,max=200,dive,uuid"`
}

// SpotcheckReq 抽查请求：random 按比例随机抽取，full 范围内全量（上限 500 条）。
type SpotcheckReq struct {
	CommunityID string `json:"community_id"`
	InspectorID string `json:"inspector_id"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Mode        string `json:"mode" binding:"required,oneof=random full"`
	Ratio       int    `json:"ratio" binding:"omitempty,min=1,max=100"`
	Handler     string `json:"handler" binding:"required,oneof=manual ai"`
}
