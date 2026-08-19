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

// UserCreateReq 新增用户（项目归属在小区「岗位编制」维护，不再随用户表单提交）。
type UserCreateReq struct {
	Username string   `json:"username" binding:"required"`
	Password string   `json:"password" binding:"required"`
	Name     string   `json:"name" binding:"required"`
	Phone    string   `json:"phone" binding:"required"`
	RoleIDs  []string `json:"role_ids"` // 可空：权限可由岗位绑定角色并集获得（方案 §3 有效角色实时并集）
	Status   *int     `json:"status"`
}

// UserUpdateReq 修改用户（username/password 不可改）。
type UserUpdateReq struct {
	Name    string   `json:"name" binding:"required"`
	Phone   string   `json:"phone" binding:"required"`
	RoleIDs []string `json:"role_ids"` // 可空：权限可由岗位绑定角色并集获得
	Status  *int     `json:"status"`
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
	Code string `json:"code"`
	Name string `json:"name"`
}

// UserItem 用户列表项（community_names 为项目编制推导的所属项目名称，导出用）。
type UserItem struct {
	ID       string     `json:"id"`
	Username string     `json:"username"`
	Name     string     `json:"name"`
	Phone    string     `json:"phone"`
	Avatar   string     `json:"avatar"`
	Openid   string     `json:"openid"`
	Roles    []RoleItem `json:"roles"`
	// PostRoles 岗位带入角色（在职编制岗位绑定角色，剔除已手动分配的部分；仅列表页填充，只读展示用）
	PostRoles      []RoleItem `json:"post_roles"`
	CommunityNames []string   `json:"community_names"`
	Status         int        `json:"status"`
	IsBuiltin      bool       `json:"is_builtin"`
	LastLoginAt    string     `json:"last_login_at"`
	CreatedAt      string     `json:"created_at"`
}

// UserDetail 用户详情。
type UserDetail struct {
	ID        string   `json:"id"`
	Username  string   `json:"username"`
	Name      string   `json:"name"`
	Phone     string   `json:"phone"`
	Avatar    string   `json:"avatar"`
	Openid    string   `json:"openid"`
	RoleIDs   []string `json:"role_ids"`
	Status    int      `json:"status"`
	IsBuiltin bool     `json:"is_builtin"`
	// 手写签名图（月报签字栏用）
	SignatureFileKey string `json:"signature_file_key"`
	SignatureURL     string `json:"signature_url"`
	LastLoginAt      string `json:"last_login_at"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
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
	Code      string   `json:"code"`
	Name      string   `json:"name" binding:"required"`
	DataScope string   `json:"data_scope" binding:"required,oneof=all project self"`
	Remark    string   `json:"remark"`
	Status    *int     `json:"status"`
	MenuIDs   []string `json:"menu_ids"`
}

// RoleAssignReq 分配菜单与数据范围。
type RoleAssignReq struct {
	MenuIDs   []string `json:"menu_ids" binding:"required"`
	DataScope string   `json:"data_scope" binding:"required,oneof=all project self"`
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
	ID        string   `json:"id"`
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	DataScope string   `json:"data_scope"`
	Remark    string   `json:"remark"`
	Status    int      `json:"status"`
	MenuIDs   []string `json:"menu_ids"`
	CreatedAt string   `json:"created_at"`
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
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Title    string `json:"title"`
	Path     string `json:"path"`
	Icon     string `json:"icon"`
	Type     string `json:"type"`
	Perms    string `json:"perms"`
	Sort     int    `json:"sort"`
	Visible  int    `json:"visible"`
	Status   int    `json:"status"`
	// IsPlatform 平台级菜单（仅超管可见可授权）；菜单管理界面展示用
	IsPlatform bool       `json:"is_platform"`
	Children   []MenuNode `json:"children"`
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

// SignAssetCreateReq 新增签章资产（创建即 active，同租户+type+owner 原 active 自动置 replaced）。
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
	Status    string `form:"status"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

// LoginLogQuery 登录日志查询。
type LoginLogQuery struct {
	response.PageQuery
	Username  string `form:"username"`
	IP        string `form:"ip"`
	Status    string `form:"status"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

// ========== 租户管理（P3 多租户） ==========

// TenantListQuery 租户列表查询。
type TenantListQuery struct {
	response.PageQuery
	Name   string `form:"name"`
	Status string `form:"status"` // enabled / disabled（原始字符串，与 tenant.status 一致）
}

// TenantItem 租户列表项。
type TenantItem struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Status       int    `json:"status"`
	UserCount    int64  `json:"user_count"`
	Remark       string `json:"remark"`
	CreatedAt    string `json:"created_at"`
}

// TenantCreateReq 开通租户（同时创建初始管理员账号，挂内置 tenant_admin 角色）。
type TenantCreateReq struct {
	Code          string `json:"code" binding:"required"` // 公司代码（登录多租户消歧用，创建后不可改）
	Name          string `json:"name" binding:"required"` // 物业公司名称
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	Remark        string `json:"remark"`
	AdminUsername string `json:"admin_username" binding:"required"` // 初始管理员登录名
	AdminPassword string `json:"admin_password" binding:"required"` // 初始密码（首次登录强制改密）
	AdminName     string `json:"admin_name"`                        // 初始管理员姓名（缺省「租户名+管理员」）
}

// TenantUpdateReq 修改租户基础信息（code 不可改）。
type TenantUpdateReq struct {
	Name         string `json:"name" binding:"required"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Remark       string `json:"remark"`
}

// TenantConfigItem 租户配置项视图（白名单 key：租户覆盖值 + 平台默认值 + 生效值）。
type TenantConfigItem struct {
	Key       string `json:"key"`
	Value     string `json:"value"`     // 租户覆盖值（空=未覆盖）
	Platform  string `json:"platform"`  // 平台默认值（sys_config）
	Effective string `json:"effective"` // 生效值（租户值→平台默认）
}

// ========== 岗位管理 / 岗位模板库（方案第三章） ==========

// PostSaveReq 岗位新增/修改（code 创建后不可改；role_id 绑角色，空=不绑）。
type PostSaveReq struct {
	Code         string `json:"code"`
	Name         string `json:"name" binding:"required"`
	Line         string `json:"line" binding:"required,oneof=safety engineering environment service general"`
	RoleID       string `json:"role_id"`
	IsSupervisor bool   `json:"is_supervisor"`
	Sort         int    `json:"sort"`
	Status       *int   `json:"status"`
	Remark       string `json:"remark"`
}

// PostItem 岗位列表项（含业务线与绑定角色名称，按 line+sort 排序）。
type PostItem struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Line         string `json:"line"`
	LineName     string `json:"line_name"`
	IsSupervisor bool   `json:"is_supervisor"`
	RoleID       string `json:"role_id"`
	RoleName     string `json:"role_name"`
	Sort         int    `json:"sort"`
	Status       int    `json:"status"`
	Remark       string `json:"remark"`
	CreatedAt    string `json:"created_at"`
}

// PostDutyBindingItem 单条槽位默认绑定（post_codes 空数组 = 该环节跳过/降级）。
type PostDutyBindingItem struct {
	Slot      string   `json:"slot" binding:"required"`
	PostCodes []string `json:"post_codes"`
}

// PostDutyBindingsSaveReq 租户级/平台级职责槽位默认绑定整体保存。
type PostDutyBindingsSaveReq struct {
	Bindings []PostDutyBindingItem `json:"bindings" binding:"required"`
}
