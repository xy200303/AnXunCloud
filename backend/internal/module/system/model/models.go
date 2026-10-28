// Package model 系统管理域 GORM 模型（表结构见《数据库设计文档》第三章；主键为应用层 UUIDv7）。
package model

import (
	"time"

	"gorm.io/gorm"

	"anxuncloud/internal/pkg/types"
)

// 常量定义（取值与数据库 CHECK 约束一致）
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"

	ScopePlatform   = "platform"
	ScopeTenant     = "tenant"
	ScopeProject    = "project"
	PlatformScopeID = "00000000-0000-0000-0000-000000000000"

	MenuTypeDir    = "dir"
	MenuTypeMenu   = "menu"
	MenuTypeButton = "button"

	// SuperAdminCode 超级管理员角色编码（拥有全部权限与数据范围）
	SuperAdminCode = "super_admin"

	// TenantAdminCode 租户管理员角色编码（开通租户时初始管理员挂该角色）
	TenantAdminCode = "tenant_admin"

	ProjectManagerCode     = "project_manager"
	QualitySupervisorCode  = "quality_supervisor"
	EngineerSupervisorCode = "engineer_supervisor"
	InspectorCode          = "inspector"
	MaintainerCode         = "maintainer"
	BuildingManagerCode    = "building_manager"
	FireDutyCode           = "fire_duty"

	// DefaultTenantCode 默认租户代码（私有化部署 = 只有默认租户的同一套系统；存量数据全部归入）
	DefaultTenantCode = "default"

	// 签章资产类型
	SignAssetTypeUserSignature = "user_signature" // 用户手写签名（owner_id=用户）
	SignAssetTypeCompanySeal   = "company_seal"   // 公章（owner_id 为 NULL，每租户仅一条 active）

	// 签章资产状态
	SignAssetStatusActive   = "active"   // 当前生效
	SignAssetStatusReplaced = "replaced" // 被新版本替换
	SignAssetStatusRevoked  = "revoked"  // 作废（需填原因）
)

// SysUser 系统用户：一个账号仅持有一个角色；同一自然人的多个账号用 PersonKey 关联。
type SysUser struct {
	types.UUIDModel
	TenantID           string         `gorm:"type:uuid" json:"tenant_id"`
	Username           string         `gorm:"size:64" json:"username"`
	Password           string         `gorm:"size:128" json:"-"`
	Name               string         `gorm:"size:64" json:"name"`
	PersonKey          string         `gorm:"size:128" json:"person_key"`
	Phone              string         `gorm:"size:20" json:"phone"`
	Openid             *string        `gorm:"size:64" json:"openid"`
	Avatar             string         `gorm:"size:512" json:"avatar"`
	RoleID             *string        `gorm:"type:uuid" json:"role_id"`
	Status             string         `gorm:"size:16" json:"status"`
	IsBuiltin          bool           `json:"is_builtin"`
	MustChangePassword bool           `json:"must_change_password"`
	LastLoginAt        *time.Time     `json:"last_login_at"`
	Remark             string         `gorm:"size:255" json:"remark"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-"`
}

func (SysUser) TableName() string { return "sys_user" }

// SysRole 角色即岗位；scope_level 决定范围计算方式，主管标志决定可见深度。
type SysRole struct {
	types.UUIDModel
	TenantID       *string           `gorm:"type:uuid" json:"tenant_id"`
	Code           string            `gorm:"size:64" json:"code"`
	Name           string            `gorm:"size:64" json:"name"`
	ScopeLevel     string            `gorm:"size:16" json:"scope_level"`
	IsSupervisor   bool              `json:"is_supervisor"`
	ClaimWorkTypes types.StringArray `gorm:"type:jsonb" json:"claim_work_types"`
	Remark         string            `gorm:"size:255" json:"remark"`
	Status         string            `gorm:"size:16" json:"status"`
	IsBuiltin      bool              `json:"is_builtin"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	DeletedAt      gorm.DeletedAt    `json:"-"`
}

func (SysRole) TableName() string { return "sys_role" }

// SysMenu 菜单与按钮权限点（树形；ParentID 为空字符串表示根，落库为 NULL）
type SysMenu struct {
	types.UUIDModel
	ParentID  *string `gorm:"type:uuid" json:"parent_id"`
	Title     string  `gorm:"size:64" json:"title"`
	Path      string  `gorm:"size:255" json:"path"`
	Icon      string  `gorm:"size:64" json:"icon"`
	Type      string  `gorm:"size:8" json:"type"`
	Perms     string  `gorm:"size:128" json:"perms"`
	Sort      int     `json:"sort"`
	Visible   bool    `json:"visible"`
	Status    string  `gorm:"size:16" json:"status"`
	IsBuiltin bool    `json:"is_builtin"`
	// IsPlatform 平台级菜单（平台管理目录整棵子树）：仅超管可见可授权，租户角色不可分配
	IsPlatform bool           `json:"is_platform"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-"`
}

// ParentIDStr 父菜单 ID（根返回空字符串）。
func (m *SysMenu) ParentIDStr() string {
	if m.ParentID == nil {
		return ""
	}
	return *m.ParentID
}

func (SysMenu) TableName() string { return "sys_menu" }

// SysRoleMenu 角色-菜单关联
type SysRoleMenu struct {
	RoleID string `gorm:"type:uuid;primaryKey" json:"role_id"`
	MenuID string `gorm:"type:uuid;primaryKey" json:"menu_id"`
}

func (SysRoleMenu) TableName() string { return "sys_role_menu" }

// SysDictType 字典类型
type SysDictType struct {
	types.UUIDModel
	Code      string         `gorm:"size:64" json:"code"`
	Name      string         `gorm:"size:64" json:"name"`
	Remark    string         `gorm:"size:255" json:"remark"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (SysDictType) TableName() string { return "sys_dict_type" }

// SysDictData 字典数据
type SysDictData struct {
	types.UUIDModel
	TypeCode string `gorm:"size:64" json:"type_code"`
	Label    string `gorm:"size:64" json:"label"`
	Value    string `gorm:"size:64" json:"value"`
	Sort     int    `json:"sort"`
	Status   string `gorm:"size:16" json:"status"`
	Remark   string `gorm:"size:255" json:"remark"`
	// Attrs 通用扩展属性（迁移 00005；patrol_type 用 attrs.category 标记大类 daily_patrol/special）
	Attrs     types.JSONMap  `gorm:"type:jsonb" json:"attrs"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (SysDictData) TableName() string { return "sys_dict_data" }

// SysConfig 系统参数
type SysConfig struct {
	types.UUIDModel
	Scope               string         `gorm:"size:16;default:platform" json:"scope"`
	ScopeID             string         `gorm:"type:uuid;default:00000000-0000-0000-0000-000000000000" json:"scope_id"`
	Key                 string         `gorm:"size:64" json:"key"`
	Name                string         `gorm:"size:64" json:"name"`
	Value               string         `gorm:"type:text" json:"value"`
	ConfigGroup         string         `gorm:"size:50;default:system" json:"config_group"`
	Remark              string         `gorm:"size:255" json:"remark"`
	AllowTenantOverride bool           `json:"allow_tenant_override"`
	IsSecret            bool           `json:"is_secret"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `json:"-"`
}

func (SysConfig) TableName() string { return "config" }

// SysLoginLog 登录日志（只增不改）
type SysLoginLog struct {
	types.UUIDModel
	TenantID  *string   `gorm:"type:uuid" json:"tenant_id"` // 所属租户（日志管理按租户上下文过滤；无法识别用户时归默认租户）
	UserID    *string   `gorm:"type:uuid" json:"user_id"`
	Username  string    `gorm:"size:64" json:"username"`
	Channel   string    `gorm:"size:16" json:"channel"`
	IP        string    `gorm:"size:64" json:"ip"`
	UA        string    `gorm:"size:512" json:"ua"`
	Status    string    `gorm:"size:16" json:"status"`
	Msg       string    `gorm:"size:255" json:"msg"`
	CreatedAt time.Time `json:"created_at"`
}

func (SysLoginLog) TableName() string { return "sys_login_log" }

// SysOperationLog 操作日志（按月分区，只增不改；主键 id+created_at）
type SysOperationLog struct {
	types.UUIDModel
	TenantID  *string   `gorm:"type:uuid" json:"tenant_id"` // 操作者所属租户（日志管理按租户上下文过滤）
	UserID    *string   `gorm:"type:uuid" json:"user_id"`
	Username  string    `gorm:"size:64" json:"username"`
	Module    string    `gorm:"size:64" json:"module"`
	Action    string    `gorm:"size:64" json:"action"`
	Method    string    `gorm:"size:8" json:"method"`
	Path      string    `gorm:"size:255" json:"path"`
	Params    string    `gorm:"type:text" json:"params"`
	IP        string    `gorm:"size:64" json:"ip"`
	Status    string    `gorm:"size:16" json:"status"`
	CostMs    int       `json:"cost_ms"`
	CreatedAt time.Time `gorm:"primaryKey" json:"created_at"`
}

func (SysOperationLog) TableName() string { return "sys_operation_log" }

// Community 小区/项目（负责人 manager_id 保留；签字/派单等业务身份走 duty_binding 槽位 + project_staff 编制）
type Community struct {
	types.UUIDModel
	TenantID  string         `gorm:"type:uuid" json:"tenant_id"` // 所属租户（业务表经 community 链路天然隔离）
	Name      string         `gorm:"size:128" json:"name"`
	Address   string         `gorm:"size:255" json:"address"`
	ManagerID *string        `gorm:"type:uuid" json:"manager_id"`
	Status    string         `gorm:"size:16" json:"status"`
	Remark    string         `gorm:"size:255" json:"remark"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (Community) TableName() string { return "community" }

// RoleCodeQualitySupervisor 是原安全主管岗位的目标角色，任务执行按巡检员角色筛选。
const RoleCodeQualitySupervisor = QualitySupervisorCode

// 审批流程 code（approval_flow.flow_code，系统固定枚举；步骤内容可配，代码只认流程 code）
const (
	FlowCheckinReview    = "checkin_review"    // 打卡审核链
	FlowReportReview     = "report_review"     // 报告审核链
	FlowMaintReview      = "maint_review"      // 维保登记审核链
	FlowWorkorderReview  = "workorder_review"  // 工单整改后的主管审核链
	FlowAcceptanceReview = "acceptance_review" // 承接查验报告审核与签署链
)

func ValidApprovalFlowCode(code string) bool {
	switch code {
	case FlowCheckinReview, FlowReportReview, FlowMaintReview, FlowWorkorderReview, FlowAcceptanceReview:
		return true
	}
	return false
}

// 审批链环节类型（FlowStep.Kind；”=人工环节）
const (
	FlowStepKindAI = "ai" // AI 闸门环节：按审核结果路由（OnPass/OnFail）
)

// AI 环节结果路由（FlowStep.OnPass/OnAbnormal/OnReview）
const (
	FlowRouteFinish = "finish" // 直接生效（整单通过，流程结束）
	FlowRouteNext   = "next"   // 进入下一环节（无下一环节时转汇报线兜底人工）
	FlowRouteReject = "reject" // 直接打回（OnPass 不允许）
	// FlowRouteGotoPrefix 跳到指定环节："goto:N"（N 为 1 起环节序号，仅可向后跳，校验保证无环）
	FlowRouteGotoPrefix = "goto:"
)

// AI 判定三分支（RouteAIStep outcome 入参）
const (
	AIGatePass     = "pass"     // 无异常
	AIGateAbnormal = "abnormal" // 有异常（巡检成果）
	AIGateReview   = "review"   // 存疑/识别失败/AI 不可用/强制人工（如存在异常项）
)

// ProjectStaff 任职：一行一个小区，只承载授权范围与分派属性。
type ProjectStaff struct {
	types.UUIDModel
	TenantID         *string       `gorm:"type:uuid" json:"tenant_id"`
	ProjectID        string        `gorm:"type:uuid" json:"project_id"`
	UserID           string        `gorm:"type:uuid" json:"user_id"`
	BuildingIDs      types.IDArray `gorm:"type:jsonb" json:"building_ids"`
	EffectiveFrom    *time.Time    `json:"effective_from"`
	EffectiveTo      *time.Time    `json:"effective_to"`
	DelegateUserID   *string       `gorm:"type:uuid" json:"delegate_user_id"`
	DelegateUntil    *time.Time    `json:"delegate_until"`
	TemporaryOrderID *string       `gorm:"type:uuid" json:"temporary_order_id"`
	Status           string        `gorm:"size:16" json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

func (ProjectStaff) TableName() string { return "project_staff" }

// ApprovalFlow 使用统一配置作用域，按 project → tenant → platform 回落。
type ApprovalFlow struct {
	types.UUIDModel
	Scope     string              `gorm:"size:16;default:platform" json:"scope"`
	ScopeID   string              `gorm:"type:uuid;default:00000000-0000-0000-0000-000000000000" json:"scope_id"`
	FlowCode  string              `gorm:"size:64" json:"flow_code"`
	Steps     types.FlowStepArray `gorm:"type:jsonb" json:"steps"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

func (ApprovalFlow) TableName() string { return "approval_flow" }

// SysNotice 通知公告（第二阶段补充表，status：0 草稿 / 1 已发布 / 2 已下线）
type SysNotice struct {
	types.UUIDModel
	TenantID      *string               `gorm:"type:uuid" json:"tenant_id"` // 冗余列（查询按接收人链路隔离）
	Title         string                `gorm:"size:64" json:"title"`
	Content       string                `gorm:"type:text" json:"content"`
	Status        int                   `json:"status"`
	Attachments   types.AttachmentArray `gorm:"type:jsonb" json:"attachments"`
	PublishAt     *time.Time            `json:"publish_at"`
	CreatedBy     *string               `gorm:"type:uuid" json:"created_by"`
	CreatedByName string                `gorm:"size:64" json:"created_by_name"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	DeletedAt     gorm.DeletedAt        `json:"-"`
}

func (SysNotice) TableName() string { return "sys_notice" }

// SysMessage 站内消息（第二阶段补充表）
type SysMessage struct {
	types.UUIDModel
	TenantID  *string   `gorm:"type:uuid" json:"tenant_id"` // 冗余列（查询按 user_id 接收人隔离）
	UserID    string    `gorm:"type:uuid" json:"user_id"`
	Type      string    `gorm:"size:16" json:"type"`
	Title     string    `gorm:"size:128" json:"title"`
	Content   string    `gorm:"size:512" json:"content"`
	BizID     *string   `gorm:"type:uuid" json:"biz_id"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func (SysMessage) TableName() string { return "sys_message" }

// UserPushDevice App 推送设备绑定（uniPush 2.0 / 个推 V2；cid 全局唯一，换账号登录即改绑）。
type UserPushDevice struct {
	types.UUIDModel
	UserID    string    `gorm:"type:uuid;index" json:"user_id"`
	CID       string    `gorm:"column:cid;size:128;uniqueIndex" json:"cid"`
	Platform  string    `gorm:"size:16" json:"platform"` // android / ios
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserPushDevice) TableName() string { return "user_push_device" }

// UploadFile 上传文件记录（第二阶段补充表；v21 起补 name/md5/storage 统一文件元数据）
type UploadFile struct {
	types.UUIDModel
	TenantID       *string       `gorm:"type:uuid" json:"tenant_id"` // 冗余列（访问控制按 scene/归属判定）
	StorageKey     string        `gorm:"column:storage_key;size:255" json:"-"`
	Scene          string        `gorm:"size:16" json:"scene"`
	UserID         string        `gorm:"type:uuid" json:"user_id"` // 零 UUID = 服务端生成文件（月报/导出包）
	MimeType       string        `gorm:"size:128" json:"mime_type"`
	URL            string        `gorm:"size:512" json:"url"`
	WatermarkedURL string        `gorm:"size:512" json:"watermarked_url"`
	Name           string        `gorm:"size:255" json:"name"`   // 原始文件名
	MD5            string        `gorm:"size:64" json:"md5"`     // 内容摘要（完整性校验/去重）
	Size           int64         `json:"size"`                   // 文件字节数
	Storage        string        `gorm:"size:16" json:"storage"` // 存储驱动：local/oss/cos
	ExifTime       *time.Time    `json:"exif_time"`
	ParentFileID   *string       `gorm:"type:uuid" json:"parent_file_id"`
	EvidenceRole   string        `gorm:"size:24;default:original" json:"evidence_role"`
	SHA256         string        `gorm:"size:64" json:"sha256"`
	DerivativeMeta types.JSONMap `gorm:"type:jsonb;default:'{}'" json:"derivative_meta"`
	CreatedAt      time.Time     `json:"created_at"`
}

func (UploadFile) TableName() string { return "upload_file" }

// AppRelease App 发布物（迁移 v15 建表）：Android APK / 鸿蒙 HAP / iOS IPA / 微信小程序码。
// 官网 /download 按平台取最新一条展示；文件统一走文件层（scene=app）。
type AppRelease struct {
	types.UUIDModel
	Platform string `gorm:"size:16;index:idx_app_release_platform,priority:1" json:"platform"` // android/harmony/ios/wechat_mp
	Version  string `gorm:"size:64" json:"version"`
	FileID   string `gorm:"type:uuid" json:"file_id"`
	Name     string `gorm:"size:255" json:"name"`
	Size     int64  `json:"size"`
	Note     string `gorm:"size:255" json:"note"`
	// ForceUpdate 强制更新标记：true=App 启动弹窗不可跳过；false=弱更新可「以后再说」
	ForceUpdate bool      `gorm:"not null;default:false" json:"force_update"`
	CreatedAt   time.Time `gorm:"index:idx_app_release_platform,priority:2,sort:desc" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AppRelease) TableName() string { return "app_release" }

// SignAsset 签章资产（迁移 v16 建表）：手写签名/公章版本链，换签名/换章不删旧记录。
// 同 tenant_id+asset_type+owner_id 仅一条 active（部分唯一索引 uk_sign_asset_active 保证，迁移 v22 起含租户维度）。
type SignAsset struct {
	types.UUIDModel
	TenantID      *string    `gorm:"type:uuid" json:"tenant_id"` // 所属租户（P3 起查询/创建/作废均按租户隔离）
	AssetType     string     `gorm:"size:20" json:"asset_type"`
	OwnerID       *string    `gorm:"type:uuid" json:"owner_id"`
	FileID        string     `gorm:"type:uuid" json:"file_id"`
	SHA256        string     `gorm:"size:64" json:"sha256"`
	Version       int        `json:"version"`
	Status        string     `gorm:"size:20" json:"status"`
	Remark        string     `gorm:"size:255" json:"remark"`
	CreatedBy     *string    `gorm:"type:uuid" json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	RevokedReason string     `gorm:"size:255" json:"revoked_reason"`
}

func (SignAsset) TableName() string { return "sign_asset" }

// StatusInt 状态字符串转接口约定的 1/0。
func StatusInt(status string) int {
	if status == StatusEnabled {
		return 1
	}
	return 0
}

// StatusStr 接口约定的 1/0 转状态字符串。
func StatusStr(status int) string {
	if status == 1 {
		return StatusEnabled
	}
	return StatusDisabled
}

// Tenant 租户（物业公司，P3 多租户）：数据隔离基本单位；code 用于登录时多租户消歧（公司代码）。
type Tenant struct {
	types.UUIDModel
	Code         string    `gorm:"size:32" json:"code"`
	Name         string    `gorm:"size:128" json:"name"`
	ContactName  string    `gorm:"size:64" json:"contact_name"`
	ContactPhone string    `gorm:"size:20" json:"contact_phone"`
	Status       string    `gorm:"size:16" json:"status"` // disabled 停用后该租户全部账号无法登录
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Tenant) TableName() string { return "tenant" }
