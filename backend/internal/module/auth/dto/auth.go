// Package dto 认证模块请求/响应结构。
package dto

// LoginReq 登录请求。
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
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

// InfoResp 当前用户信息（GET /auth/info）。
type InfoResp struct {
	ID           string      `json:"id"`
	Username     string      `json:"username"`
	Name         string      `json:"name"`
	Phone        string      `json:"phone"`
	Avatar       string      `json:"avatar"`
	Roles        []RoleBrief `json:"roles"`
	CommunityIDs []string    `json:"community_ids"`
	DataScope    string      `json:"data_scope"`
	Perms        []string    `json:"perms"`
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
