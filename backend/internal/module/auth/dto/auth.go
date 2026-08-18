// Package dto 认证模块请求/响应结构。
package dto

// LoginReq 登录请求。
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// TenantCode 公司代码（P3 多租户：username 命中多个租户时必填，用于消歧；单租户/私有化部署留空即可）
	TenantCode string `json:"tenant_code"`
}

// RegisterReq 开放注册请求。
type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone" binding:"required"`
	// TenantCode 目标公司（注册下拉选择；空 = 默认租户，私有化单租户场景）
	TenantCode string `json:"tenant_code"`
}

// RefreshReq 刷新令牌请求。
type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// TokenResp 双令牌响应（data 结构）。
type TokenResp struct {
	TokenType    string   `json:"token_type"`
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int64    `json:"expires_in"`
	User         *UserBrief `json:"user,omitempty"`
}

// UserBrief 登录响应中的用户摘要。
type UserBrief struct {
	MustChangePassword bool `json:"must_change_password"`
}

// RoleBrief 角色摘要。
type RoleBrief struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// ProjectBrief 用户在职项目摘要（由 project_staff 在职编制推导；名称移动端自行解析）。
type ProjectBrief struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// InfoResp 当前用户信息（GET /auth/info）。
type InfoResp struct {
	ID           string      `json:"id"`
	Username     string      `json:"username"`
	Name         string      `json:"name"`
	Phone        string      `json:"phone"`
	Avatar       string      `json:"avatar"`
	// 手写签名图 URL（月报签字栏用，空=未设置）
	SignatureURL string      `json:"signature_url"`
	Openid       string      `json:"openid"`
	IsBuiltin    bool        `json:"is_builtin"`
	Roles        []RoleBrief `json:"roles"`
	Projects     []ProjectBrief `json:"projects"`
	DataScope    string      `json:"data_scope"`
	Perms        []string    `json:"perms"`
	CreatedAt    string      `json:"created_at"`
	LastLoginAt  string      `json:"last_login_at"`
	LastLoginIP  string      `json:"last_login_ip"`
}

// RouteNode 动态路由菜单节点（GET /auth/routes）。
type RouteNode struct {
	ID       string      `json:"id"`
	ParentID string      `json:"parent_id"`
	Title    string      `json:"title"`
	Path     string      `json:"path"`
	Icon     string      `json:"icon"`
	Type     string      `json:"type"`
	Sort     int         `json:"sort"`
	Children []RouteNode `json:"children"`
}
