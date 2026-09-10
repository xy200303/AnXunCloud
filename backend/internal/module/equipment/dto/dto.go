// Package dto 设备台账模块请求结构（《设备台账与维保周期管理设计方案》v1.5）。
package dto

import "anxuncloud/internal/pkg/response"

// ========== 设备台账 ==========

// ListQuery 台账分页查询：due_state 为到期状态筛选（normal/warning/overdue/none）。
type ListQuery struct {
	response.PageQuery
	Type        string `form:"type"`
	CommunityID string `form:"community_id"`
	PointID     string `form:"point_id"` // 按关联点位反查（一点多具；点位详情「关联设备」区块用）
	Status      string `form:"status"`    // in_service/maintaining/stopped/scrapped
	DueState    string `form:"due_state"` // normal/warning/overdue/none（无到期日）
	Keyword     string `form:"keyword"`   // 设备编号或名称模糊
}

// SaveReq 设备新增/修改请求。
// ManufactureDate/NextDueDate 为 YYYY-MM-DD（可空）；NextDueDate 留空：新增按类型规则计算默认值，修改保持原值。
type SaveReq struct {
	CommunityID     string         `json:"community_id" binding:"required"`
	BuildingID      *string        `json:"building_id"`
	PointID         *string        `json:"point_id"` // 关联巡检点位（可空；类型不匹配只警告不拦截）
	Type            string         `json:"type" binding:"required"`
	Code            string         `json:"code" binding:"required"` // 设备编号（租户内唯一）
	Name            string         `json:"name" binding:"required"`
	ManufactureDate string         `json:"manufacture_date"`
	NextDueDate     string         `json:"next_due_date"` // 人工覆盖到期日（空=规则默认/保持原值）
	WarnDays        *int           `json:"warn_days"`     // 临期阈值覆盖（nil 用全局配置）
	Status          string         `json:"status"`
	Extra           map[string]any `json:"extra"`
	Remark          string         `json:"remark"`
}

// BatchDeleteReq 批量删除（软删除，维保流水保留）：
// ids 逐台删（勾选模式，上限 500）；all=true 按筛选条件删全部（跨页全选，上限 2000，租户隔离由筛选查询自带）。
type BatchDeleteReq struct {
	IDs         []string `json:"ids"`
	All         bool     `json:"all"`
	Type        string   `json:"type"`
	CommunityID string   `json:"community_id"`
	PointID     string   `json:"point_id"`
	Status      string   `json:"status"`
	DueState    string   `json:"due_state"`
	Keyword     string   `json:"keyword"`
}

// ExportIdsReq 按 id 集合导出（勾选导出）。
type ExportIdsReq struct {
	IDs []string `json:"ids" binding:"required"`
}

// ========== 维保登记 ==========

// MaintenanceRegisterReq 维保登记（一键+一拍；台账补录 ledger_fix 可同时提交补录日期，确认时一并回写）。
type MaintenanceRegisterReq struct {
	EquipmentID     string   `json:"equipment_id" binding:"required"`
	MaintenanceType string   `json:"maintenance_type"` // 缺省 repair；ledger_fix=台账补录
	MaintenanceDate string   `json:"maintenance_date"` // YYYY-MM-DD，缺省今天
	Vendor          string   `json:"vendor"`
	OperatorName    string   `json:"operator_name"` // 缺省当前用户
	Note            string   `json:"note"`
	FileIDs         []string `json:"file_ids" binding:"required,min=1"` // 新维修标签照片（必传；标签缺失时拍设备本体作证）
	// LabelMissing 标签缺失/无法辨认（钢印磨损）：勾选后日期免填，确认后设备打标退出自动判定
	LabelMissing bool `json:"label_missing"`
	// 仅 ledger_fix 有效：台账缺失日期补录（确认后回写 equipment）
	ManufactureDate     string `json:"manufacture_date"`
	LastMaintenanceDate string `json:"last_maintenance_date"`
}

// ConfirmReq 维保登记批量确认（已 confirmed/rejected 的记录跳过，幂等）。
type ConfirmReq struct {
	IDs []string `json:"ids" binding:"required,min=1,max=200"`
}

// RejectReq 维保登记驳回（须填理由，通知登记人）。
type RejectReq struct {
	ID     string `json:"id" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

// ConfirmListQuery 待确认列表分页查询。
type ConfirmListQuery struct {
	response.PageQuery
}

// ========== 导入导出 ==========

// ImportResult 台账导入结果（同编号按更新处理，幂等可重导）。
type ImportResult struct {
	Total        int          `json:"total"`
	CreatedCount int          `json:"created_count"`
	UpdatedCount int          `json:"updated_count"`
	FailCount    int          `json:"fail_count"`
	// AutoBound 自动推理绑定点位的条数（歧义/无对应不逐条报，计数即可）
	AutoBound   int          `json:"auto_bound"`
	FailDetails []ImportFail `json:"fail_details"`
}

// ImportFail 导入失败明细行。
type ImportFail struct {
	Row    int    `json:"row"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}
