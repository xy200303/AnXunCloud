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

	ScopeAll    = "all"
	ScopeCustom = "custom"

	MenuTypeDir    = "dir"
	MenuTypeMenu   = "menu"
	MenuTypeButton = "button"

	// SuperAdminCode 超级管理员角色编码（拥有全部权限与数据范围）
	SuperAdminCode = "super_admin"

	// 签章资产类型
	SignAssetTypeUserSignature = "user_signature" // 用户手写签名（owner_id=用户）
	SignAssetTypeCompanySeal   = "company_seal"   // 公章（owner_id 为 NULL，全局仅一条 active）

	// 签章资产状态
	SignAssetStatusActive   = "active"   // 当前生效
	SignAssetStatusReplaced = "replaced" // 被新版本替换
	SignAssetStatusRevoked  = "revoked"  // 作废（需填原因）
)

// SysUser 系统用户（后台管理员/巡检员/维修工）
type SysUser struct {
	types.UUIDModel
	Username           string        `gorm:"size:64" json:"username"`
	Password           string        `gorm:"size:128" json:"-"`
	Name               string        `gorm:"size:64" json:"name"`
	Phone              string        `gorm:"size:20" json:"phone"`
	Openid             *string       `gorm:"size:64" json:"openid"`
	Avatar             string        `gorm:"size:512" json:"avatar"`
	RoleIDs            types.IDArray `gorm:"type:jsonb" json:"role_ids"`
	CommunityIDs       types.IDArray `gorm:"type:jsonb" json:"community_ids"`
	UserType           string        `gorm:"size:16" json:"user_type"`
	Status             string        `gorm:"size:16" json:"status"`
	IsBuiltin          bool          `json:"is_builtin"`
	MustChangePassword bool          `json:"must_change_password"`
	LastLoginAt        *time.Time    `json:"last_login_at"`
	Remark             string        `gorm:"size:255" json:"remark"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-"`
}

func (SysUser) TableName() string { return "sys_user" }

// SysRole 角色
type SysRole struct {
	types.UUIDModel
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
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (SysConfig) TableName() string { return "sys_config" }

// SysLoginLog 登录日志（只增不改）
type SysLoginLog struct {
	types.UUIDModel
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

// Community 小区（本阶段仅用于数据权限与用户导入校验，业务模块后续实现）
type Community struct {
	types.UUIDModel
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

// SysNotice 通知公告（第二阶段补充表，status：0 草稿 / 1 已发布 / 2 已下线）
type SysNotice struct {
	types.UUIDModel
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
	UserID    string    `gorm:"type:uuid" json:"user_id"`
	Type      string    `gorm:"size:16" json:"type"`
	Title     string    `gorm:"size:128" json:"title"`
	Content   string    `gorm:"size:512" json:"content"`
	BizID     *string   `gorm:"type:uuid" json:"biz_id"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func (SysMessage) TableName() string { return "sys_message" }

// UploadFile 上传文件记录（第二阶段补充表；v21 起补 name/md5/storage 统一文件元数据）
type UploadFile struct {
	types.UUIDModel
	FileKey        string     `gorm:"size:255" json:"file_key"`
	Scene          string     `gorm:"size:16" json:"scene"`
	UserID         string     `gorm:"type:uuid" json:"user_id"` // 零 UUID = 服务端生成文件（月报/导出包）
	Size           int64      `json:"size"`
	MimeType       string     `gorm:"size:128" json:"mime_type"`
	URL            string     `gorm:"size:512" json:"url"`
	WatermarkedURL string     `gorm:"size:512" json:"watermarked_url"`
	Name           string     `gorm:"size:255" json:"name"`    // 原始文件名
	MD5            string     `gorm:"size:64" json:"md5"`      // 内容摘要（完整性校验/去重）
	Storage        string     `gorm:"size:16" json:"storage"`  // 存储驱动：local/oss/cos
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
// 同 asset_type+owner_id 仅一条 active（部分唯一索引 uk_sign_asset_active 保证）。
type SignAsset struct {
	types.UUIDModel
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
