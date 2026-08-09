// Package dto 系统管理模块请求/响应结构。
package dto

import "anxuncloud/internal/pkg/response"

// ========== 用户 ==========

// UserListQuery 用户列表查询。
type UserListQuery struct {
	response.PageQuery
	Username    string `form:"username"`
	Phone       string `form:"phone"`
	RoleID      string `form:"role_id"`
	CommunityID string `form:"community_id"`
	Status      string `form:"status"`
}

// UserCreateReq 新增用户。
type UserCreateReq struct {
	Username     string  `json:"username" binding:"required"`
	Password     string  `json:"password" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Phone        string  `json:"phone" binding:"required"`
	RoleIDs      []string `json:"role_ids" binding:"required,min=1"`
	CommunityIDs []string `json:"community_ids"`
	Status       *int    `json:"status"`
}

// UserUpdateReq 修改用户（username/password 不可改）。
type UserUpdateReq struct {
	Name         string  `json:"name" binding:"required"`
	Phone        string  `json:"phone" binding:"required"`
	RoleIDs      []string `json:"role_ids" binding:"required,min=1"`
	CommunityIDs []string `json:"community_ids"`
	Status       *int    `json:"status"`
}

// ResetPasswordReq 重置密码。
type ResetPasswordReq struct {
	NewPassword string `json:"new_password" binding:"required"`
}

// StatusReq 启停用请求。
type StatusReq struct {
	Status int `json:"status" binding:"min=0,max=1"`
}

// RoleItem 用户视图中的角色摘要。
type RoleItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UserItem 用户列表项。
type UserItem struct {
	ID             string     `json:"id"`
	Username       string     `json:"username"`
	Name           string     `json:"name"`
	Phone          string     `json:"phone"`
	Avatar         string     `json:"avatar"`
	Openid         string     `json:"openid"`
	Roles          []RoleItem `json:"roles"`
	CommunityIDs   []string   `json:"community_ids"`
	CommunityNames []string   `json:"community_names"`
	Status         int        `json:"status"`
	IsBuiltin      bool       `json:"is_builtin"`
	LastLoginAt    string     `json:"last_login_at"`
	CreatedAt      string     `json:"created_at"`
}

// UserDetail 用户详情。
type UserDetail struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Name         string  `json:"name"`
	Phone        string  `json:"phone"`
	Avatar       string  `json:"avatar"`
	Openid       string  `json:"openid"`
	RoleIDs      []string `json:"role_ids"`
	CommunityIDs []string `json:"community_ids"`
	Status       int     `json:"status"`
	IsBuiltin    bool    `json:"is_builtin"`
	// 手写签名图（月报签字栏用）
	SignatureFileKey string `json:"signature_file_key"`
	SignatureURL     string `json:"signature_url"`
	LastLoginAt  string  `json:"last_login_at"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// ImportResult 导入结果。
type ImportResult struct {
	Total        int          `json:"total"`
	SuccessCount int          `json:"success_count"`
	FailCount    int          `json:"fail_count"`
	FailDetails  []FailDetail `json:"fail_details"`
}

// FailDetail 导入失败明细。
type FailDetail struct {
	Row    int    `json:"row"`
	Phone  string `json:"phone"`
	Reason string `json:"reason"`
}

// ========== 角色 ==========

// RoleListQuery 角色列表查询。
type RoleListQuery struct {
	response.PageQuery
	Name   string `form:"name"`
	Status string `form:"status"`
}

// RoleSaveReq 新增/修改角色。
type RoleSaveReq struct {
	Code      string  `json:"code"`
	Name      string  `json:"name" binding:"required"`
	DataScope string  `json:"data_scope" binding:"required,oneof=all custom"`
	Remark    string  `json:"remark"`
	Status    *int    `json:"status"`
	MenuIDs   []string `json:"menu_ids"`
}

// RoleAssignReq 分配菜单与数据范围。
type RoleAssignReq struct {
	MenuIDs   []string `json:"menu_ids" binding:"required"`
	DataScope string  `json:"data_scope" binding:"required,oneof=all custom"`
}

// RoleListItem 角色列表项。
type RoleListItem struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	DataScope string `json:"data_scope"`
	Remark    string `json:"remark"`
	Status    int    `json:"status"`
	UserCount int64  `json:"user_count"`
	CreatedAt string `json:"created_at"`
}

// RoleDetail 角色详情。
type RoleDetail struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	DataScope string  `json:"data_scope"`
	Remark    string  `json:"remark"`
	Status    int     `json:"status"`
	MenuIDs   []string `json:"menu_ids"`
	CreatedAt string  `json:"created_at"`
}

// ========== 菜单 ==========

// MenuListQuery 菜单树查询。
type MenuListQuery struct {
	Title  string `form:"title"`
	Status string `form:"status"`
}

// MenuSaveReq 新增/修改菜单。
type MenuSaveReq struct {
	ParentID string `json:"parent_id"`
	Title    string `json:"title" binding:"required"`
	Path     string `json:"path"`
	Icon     string `json:"icon"`
	Type     string `json:"type" binding:"required,oneof=dir menu button"`
	Perms    string `json:"perms"`
	Sort     int    `json:"sort"`
	Visible  *int   `json:"visible"`
	Status   *int   `json:"status"`
}

// MenuNode 菜单树节点。
type MenuNode struct {
	ID        string     `json:"id"`
	ParentID  string     `json:"parent_id"`
	Title     string     `json:"title"`
	Path      string     `json:"path"`
	Icon      string     `json:"icon"`
	Type      string     `json:"type"`
	Perms     string     `json:"perms"`
	Sort      int        `json:"sort"`
	Visible   int        `json:"visible"`
	Status    int        `json:"status"`
	Children  []MenuNode `json:"children"`
}

// ========== 字典 ==========

// DictTypeQuery 字典类型查询。
type DictTypeQuery struct {
	response.PageQuery
	Code string `form:"code"`
	Name string `form:"name"`
}

// DictTypeSaveReq 新增/修改字典类型。
type DictTypeSaveReq struct {
	Code   string `json:"code"`
	Name   string `json:"name" binding:"required"`
	Remark string `json:"remark"`
}

// DictDataQuery 字典数据查询。
type DictDataQuery struct {
	response.PageQuery
	TypeCode string `form:"type_code" binding:"required"`
	Label    string `form:"label"`
	Status   string `form:"status"`
}

// DictDataSaveReq 新增/修改字典数据。
type DictDataSaveReq struct {
	TypeCode string `json:"type_code"`
	Label    string `json:"label" binding:"required"`
	Value    string `json:"value" binding:"required"`
	Sort     int    `json:"sort"`
	Status   *int   `json:"status"`
	Remark   string `json:"remark"`
}

// ========== 参数配置 ==========

// ConfigQuery 参数查询。
type ConfigQuery struct {
	response.PageQuery
	Key   string `form:"key"`
	Name  string `form:"name"`
	Group string `form:"group"`
}

// ConfigSaveReq 新增/修改参数。
type ConfigSaveReq struct {
	Key         string `json:"key"`
	Name        string `json:"name" binding:"required"`
	Value       string `json:"value"`
	ConfigGroup string `json:"config_group" binding:"required"`
	Remark      string `json:"remark"`
}

// ========== 签章资产 ==========

// SignAssetQuery 签章资产列表查询。
type SignAssetQuery struct {
	response.PageQuery
	AssetType string `form:"asset_type"`
	OwnerID   string `form:"owner_id"`
	Status    string `form:"status"`
}

// SignAssetCreateReq 新增签章资产（创建即 active，同 type+owner 原 active 自动置 replaced）。
type SignAssetCreateReq struct {
	AssetType string  `json:"asset_type" binding:"required,oneof=user_signature company_seal"`
	OwnerID   *string `json:"owner_id"`
	FileKey   string  `json:"file_key" binding:"required"`
	Remark    string  `json:"remark"`
}

// SignAssetRevokeReq 作废签章资产。
type SignAssetRevokeReq struct {
	Reason string `json:"reason" binding:"required"`
}

// SignAssetItem 签章资产视图。
type SignAssetItem struct {
	ID            string  `json:"id"`
	AssetType     string  `json:"asset_type"`
	OwnerID       *string `json:"owner_id"`
	OwnerName     string  `json:"owner_name"`
	FileKey       string  `json:"file_key"`
	URL           string  `json:"url"`
	SHA256        string  `json:"sha256"`
	SHA256Short   string  `json:"sha256_short"`
	Version       int     `json:"version"`
	Status        string  `json:"status"`
	Remark        string  `json:"remark"`
	CreatedBy     *string `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	RevokedAt     string  `json:"revoked_at"`
	RevokedReason string  `json:"revoked_reason"`
}

// ========== 日志 ==========

// OperationLogQuery 操作日志查询。
type OperationLogQuery struct {
	response.PageQuery
	Username  string `form:"username"`
	Module    string `form:"module"`
	Action    string `form:"action"`
	Status   string `form:"status"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

// LoginLogQuery 登录日志查询。
type LoginLogQuery struct {
	response.PageQuery
	Username  string `form:"username"`
	IP        string `form:"ip"`
	Status   string `form:"status"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}
