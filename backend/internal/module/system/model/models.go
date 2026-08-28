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

	ScopeAll     = "all"
	ScopeProject = "project"
	ScopeSelf    = "self"

	MenuTypeDir    = "dir"
	MenuTypeMenu   = "menu"
	MenuTypeButton = "button"

	// SuperAdminCode 超级管理员角色编码（拥有全部权限与数据范围）
	SuperAdminCode = "super_admin"

	// TenantAdminCode 租户管理员角色编码（开通租户时初始管理员挂该角色）
	TenantAdminCode = "tenant_admin"

	// ProjectAdminCode 项目管理员角色编码（主管级/项目经理岗位默认绑该角色）
	ProjectAdminCode = "project_admin"

	// FieldStaffCode 一线人员角色编码（一线岗位默认绑该角色）
	FieldStaffCode = "field_staff"

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

// SysUser 系统用户（后台与移动端账号；项目归属与岗位职责由 project_staff 表达）
type SysUser struct {
	types.UUIDModel
	TenantID           string         `gorm:"type:uuid" json:"tenant_id"` // 所属租户（登录账号租户内唯一）
	Username           string         `gorm:"size:64" json:"username"`
	Password           string         `gorm:"size:128" json:"-"`
	Name               string         `gorm:"size:64" json:"name"`
	Phone              string         `gorm:"size:20" json:"phone"`
	Openid             *string        `gorm:"size:64" json:"openid"`
	Avatar             string         `gorm:"size:512" json:"avatar"`
	RoleIDs            types.IDArray  `gorm:"type:jsonb" json:"role_ids"`
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

// SysRole 角色（tenant_id 为空 = 内置角色，全平台共享、只读不可改；租户自建角色归属租户）
type SysRole struct {
	types.UUIDModel
	TenantID  *string        `gorm:"type:uuid" json:"tenant_id"`
	Code      string         `gorm:"size:64" json:"code"`
	Name      string         `gorm:"size:64" json:"name"`
	DataScope string         `gorm:"size:16" json:"data_scope"`
	Remark    string         `gorm:"size:255" json:"remark"`
	Status    string         `gorm:"size:16" json:"status"`
	IsBuiltin bool           `json:"is_builtin"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (SysRole) TableName() string { return "sys_role" }

// SysMenu 菜单与按钮权限点（树形；ParentID 为空字符串表示根，落库为 NULL）
type SysMenu struct {
	types.UUIDModel
	ParentID  *string        `gorm:"type:uuid" json:"parent_id"`
	Title     string         `gorm:"size:64" json:"title"`
	Path      string         `gorm:"size:255" json:"path"`
	Icon      string         `gorm:"size:64" json:"icon"`
	Type      string         `gorm:"size:8" json:"type"`
	Perms     string         `gorm:"size:128" json:"perms"`
	Sort      int            `json:"sort"`
	Visible   bool           `json:"visible"`
	Status    string         `gorm:"size:16" json:"status"`
	IsBuiltin bool           `json:"is_builtin"`
	// IsPlatform 平台级菜单（平台管理目录整棵子树）：仅超管可见可授权，租户角色不可分配
	IsPlatform bool          `json:"is_platform"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
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
	TypeCode  string         `gorm:"size:64" json:"type_code"`
	Label     string         `gorm:"size:64" json:"label"`
	Value     string         `gorm:"size:64" json:"value"`
	Sort      int            `json:"sort"`
	Status    string         `gorm:"size:16" json:"status"`
	Remark    string         `gorm:"size:255" json:"remark"`
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
	Key         string         `gorm:"size:64" json:"key"`
	Name        string         `gorm:"size:64" json:"name"`
	Value       string         `gorm:"type:text" json:"value"`
	ConfigGroup string         `gorm:"size:50;default:system" json:"config_group"`
	Remark      string         `gorm:"size:255" json:"remark"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (SysConfig) TableName() string { return "sys_config" }

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

// 平台模板岗位代码（post_dict.code，内置固定保留；数据范围推导与默认槽位绑定引用）
const (
	PostProjectManager = "project_manager" // 项目经理（每项目至多一人，保存时校验）
	PostInspector      = "inspector"       // 巡检员
	PostRepairman      = "repairman"       // 维修工
)

// 岗位业务线（post_dict.line，扁平结构 + 按业务线分组展示，不做 parent_id 树）
const (
	PostLineSafety      = "safety"      // 安全
	PostLineEngineering = "engineering" // 工程
	PostLineEnvironment = "environment" // 环境
	PostLineService     = "service"     // 客服
	PostLineGeneral     = "general"     // 综合
)

// PostLineNames 业务线展示名（前端分组标题用）。
var PostLineNames = map[string]string{
	PostLineSafety:      "安全",
	PostLineEngineering: "工程",
	PostLineEnvironment: "环境",
	PostLineService:     "客服",
	PostLineGeneral:     "综合",
}

// ValidPostLine 校验业务线枚举（与 chk_post_dict_line 约束一致）。
func ValidPostLine(line string) bool {
	_, ok := PostLineNames[line]
	return ok
}

// 职责槽位代码（duty_binding.slot，系统固定枚举，见设计方案 §3.2；绑定岗位可配，代码只认槽位）
const (
	SlotReportSignSupervisor = "report_sign_supervisor" // 月报主管级签字
	SlotReportSignManager    = "report_sign_manager"    // 月报经理级终审
	SlotPatrolExecute        = "patrol_execute"         // 巡查任务执行
	SlotPatrolReportLine     = "patrol_report_line"     // 巡查汇报关系（通用兜底）

	// 巡查汇报关系·业务线维度槽位（《汇报线与审批链扩展设计方案》§2：<family>.<dimension> 命名，
	// 解析时先查维度槽位、未配置回落通用槽位 SlotPatrolReportLine，每级内部仍按 项目→租户→平台 回落。
	// 《专项巡检与专项检查报告设计方案》§3.1 起维度槽位按 patrol_type 字典约定衍生（patrol_report_line.<value>），
	// 下列 4 个常量仅为存量静态目录；新类型无需再加常量）
	SlotPatrolReportLineSafety      = "patrol_report_line.safety"      // 安全巡查汇报线
	SlotPatrolReportLineEquipment   = "patrol_report_line.equipment"   // 设备专项巡查汇报线
	SlotPatrolReportLineEnvironment = "patrol_report_line.environment" // 环境巡查汇报线
	SlotPatrolReportLineBuilding    = "patrol_report_line.building"    // 楼栋巡查汇报线

	SlotProjectReview = "project_review" // 项目经理复核（审批链第二环节用，扩展方案 §3.2）
)

// 审批流程 code（approval_flow.flow_code，系统固定枚举；步骤内容可配，代码只认流程 code）
const (
	FlowCheckinReview = "checkin_review" // 打卡审核链
)

// DutySlot 槽位定义（系统固定枚举，名称用于前端展示）。
type DutySlot struct {
	Slot string
	Name string
}

// DutySlots 全部职责槽位（顺序即前端展示顺序；项目级覆盖页与租户/平台默认绑定页共用）。
// 巡查汇报关系组：通用槽位居上作兜底，维度槽位随后（未配置维度槽位时回落通用绑定）。
var DutySlots = []DutySlot{
	{SlotReportSignSupervisor, "月报主管级签字"},
	{SlotReportSignManager, "月报经理级终审"},
	{SlotPatrolExecute, "巡查任务执行"},
	{SlotPatrolReportLine, "巡查打卡审核（默认）"},
	{SlotPatrolReportLineSafety, "巡查打卡审核 · 安全巡查"},
	{SlotPatrolReportLineEquipment, "巡查打卡审核 · 设备专项"},
	{SlotPatrolReportLineEnvironment, "巡查打卡审核 · 环境巡查"},
	{SlotPatrolReportLineBuilding, "巡查打卡审核 · 楼栋巡查"},
	{SlotProjectReview, "项目经理复核"},
}

// PostDict 岗位字典（tenant_id 为空 = 平台内置模板岗位；UNIQUE(tenant_id, code)）
type PostDict struct {
	types.UUIDModel
	TenantID     *string   `gorm:"type:uuid" json:"tenant_id"`
	Code         string    `gorm:"size:64" json:"code"`
	Name         string    `gorm:"size:64" json:"name"`
	Line         string    `gorm:"size:32" json:"line"`           // 业务线（safety/engineering/environment/service/general）
	IsSupervisor bool      `json:"is_supervisor"`                 // 主管级（数据范围推导：主管级岗位 → project 档）
	RoleID       *string   `gorm:"type:uuid" json:"role_id"`      // 岗位绑定角色（有效角色实时并集的一个来源，可空）
	Sort         int       `json:"sort"`                          // 业务线内排序
	Status       string    `gorm:"size:16" json:"status"`
	Remark       string    `gorm:"size:255" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (PostDict) TableName() string { return "post_dict" }

// ProjectStaff 项目岗位编制（UNIQUE(project_id, user_id)；posts 引用 post_dict.code，一人多岗）
type ProjectStaff struct {
	types.UUIDModel
	TenantID    *string           `gorm:"type:uuid" json:"tenant_id"`
	ProjectID   string            `gorm:"type:uuid" json:"project_id"`
	UserID      string            `gorm:"type:uuid" json:"user_id"`
	Posts       types.StringArray `gorm:"type:jsonb" json:"posts"`
	BuildingIDs types.IDArray     `gorm:"type:jsonb" json:"building_ids"` // 责任楼栋（仅楼管员用，空=全部楼栋）
	Status      string            `gorm:"size:16" json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func (ProjectStaff) TableName() string { return "project_staff" }

// DutyBinding 职责槽位绑定（project_id 为空 = 平台/租户级默认，非空 = 项目级覆盖；post_codes 空 = 该环节跳过）
type DutyBinding struct {
	types.UUIDModel
	TenantID  *string           `gorm:"type:uuid" json:"tenant_id"`
	ProjectID *string           `gorm:"type:uuid" json:"project_id"`
	Slot      string            `gorm:"size:64" json:"slot"`
	PostCodes types.StringArray `gorm:"type:jsonb" json:"post_codes"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

func (DutyBinding) TableName() string { return "duty_binding" }

// ApprovalFlow 审批链配置（迁移 00002；tenant_id/project_id 均空 = 平台默认，
// 解析按 项目级 → 租户级 → 平台默认 回落，未配置时代码内置单步默认链兜底）。
type ApprovalFlow struct {
	types.UUIDModel
	TenantID  *string             `gorm:"type:uuid" json:"tenant_id"`
	ProjectID *string             `gorm:"type:uuid" json:"project_id"`
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
	TenantID       *string    `gorm:"type:uuid" json:"tenant_id"` // 冗余列（访问控制按 scene/归属判定）
	FileKey        string     `gorm:"size:255" json:"file_key"`
	Scene          string     `gorm:"size:16" json:"scene"`
	UserID         string     `gorm:"type:uuid" json:"user_id"` // 零 UUID = 服务端生成文件（月报/导出包）
	MimeType       string     `gorm:"size:128" json:"mime_type"`
	URL            string     `gorm:"size:512" json:"url"`
	WatermarkedURL string     `gorm:"size:512" json:"watermarked_url"`
	Name           string     `gorm:"size:255" json:"name"`   // 原始文件名
	MD5            string     `gorm:"size:64" json:"md5"`     // 内容摘要（完整性校验/去重）
	Storage        string     `gorm:"size:16" json:"storage"` // 存储驱动：local/oss/cos
	ExifTime       *time.Time `json:"exif_time"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (UploadFile) TableName() string { return "upload_file" }

// AppRelease App 发布物（迁移 v15 建表）：Android APK / 鸿蒙 HAP / iOS IPA / 微信小程序码。
// 官网 /download 按平台取最新一条展示；文件统一走文件层（scene=app）。
type AppRelease struct {
	types.UUIDModel
	Platform  string    `gorm:"size:16;index:idx_app_release_platform,priority:1" json:"platform"` // android/harmony/ios/wechat_mp
	Version   string    `gorm:"size:64" json:"version"`
	FileKey   string    `gorm:"size:255" json:"file_key"`
	Name      string    `gorm:"size:255" json:"name"`
	Size      int64     `json:"size"`
	Note      string    `gorm:"size:255" json:"note"`
	CreatedAt time.Time `gorm:"index:idx_app_release_platform,priority:2,sort:desc" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AppRelease) TableName() string { return "app_release" }

// SignAsset 签章资产（迁移 v16 建表）：手写签名/公章版本链，换签名/换章不删旧记录。
// 同 tenant_id+asset_type+owner_id 仅一条 active（部分唯一索引 uk_sign_asset_active 保证，迁移 v22 起含租户维度）。
type SignAsset struct {
	types.UUIDModel
	TenantID      *string    `gorm:"type:uuid" json:"tenant_id"` // 所属租户（P3 起查询/创建/作废均按租户隔离）
	AssetType     string     `gorm:"size:20" json:"asset_type"`
	OwnerID       *string    `gorm:"type:uuid" json:"owner_id"`
	FileKey       string     `gorm:"size:255" json:"file_key"`
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
func StatusInt(status string) int {	if status == StatusEnabled {
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

// TenantConfig 租户配置覆盖（UNIQUE(tenant_id, key)）：读取规则「租户值→平台默认」（ConfigService.Resolve）。
// 仅白名单 key 可写（品牌类先行，见 TenantService）；密钥类配置（ai.* 等）永不下放租户。
type TenantConfig struct {
	types.UUIDModel
	TenantID  string    `gorm:"type:uuid" json:"tenant_id"`
	Key       string    `gorm:"size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (TenantConfig) TableName() string { return "tenant_config" }
