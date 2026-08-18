// Package controller 认证接口 HTTP 层。
package controller

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/auth/dto"
	"anxuncloud/internal/module/auth/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

// AuthController 认证接口。
type AuthController struct {
	svc *service.AuthService
}

func NewAuthController(svc *service.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

// Login POST /auth/login
func (ctl *AuthController) Login(c *gin.Context) {
	ctl.login(c, service.ChannelAdmin)
}

// LoginApp POST /api/app/login（APP 三端账号密码登录，渠道 app）
func (ctl *AuthController) LoginApp(c *gin.Context) {
	ctl.login(c, service.ChannelApp)
}

func (ctl *AuthController) login(c *gin.Context, channel string) {
	var req dto.LoginReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	resp, be := ctl.svc.LoginChannel(c.Request.Context(), &req, channel, c.ClientIP(), c.GetHeader("User-Agent"))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, resp)
}

// RegisterConfig GET /auth/register-config（免登录）
func (ctl *AuthController) RegisterConfig(c *gin.Context) {
	response.OK(c, ctl.svc.RegisterConfig())
}

// RegisterTenants GET /auth/register-tenants（免登录；注册下拉公司列表，仅注册开启时返回）
func (ctl *AuthController) RegisterTenants(c *gin.Context) {
	data, be := ctl.svc.RegisterTenants()
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

// Register POST /auth/register（免登录，操作日志记操作人为匿名注册者）
func (ctl *AuthController) Register(c *gin.Context) {
	var req dto.RegisterReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Register(c.Request.Context(), &req, c.ClientIP()); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// Refresh POST /auth/refresh
func (ctl *AuthController) Refresh(c *gin.Context) {
	ctl.refresh(c, service.ChannelAdmin)
}

// RefreshApp POST /api/app/refresh
func (ctl *AuthController) RefreshApp(c *gin.Context) {
	ctl.refresh(c, service.ChannelApp)
}

func (ctl *AuthController) refresh(c *gin.Context, channel string) {
	var req dto.RefreshReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	resp, be := ctl.svc.RefreshChannel(c.Request.Context(), channel, req.RefreshToken)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, resp)
}

// Logout POST /auth/logout
func (ctl *AuthController) Logout(c *gin.Context) {
	ctl.logout(c, service.ChannelAdmin)
}

// LogoutApp POST /api/app/auth/logout
func (ctl *AuthController) LogoutApp(c *gin.Context) {
	ctl.logout(c, service.ChannelApp)
}

func (ctl *AuthController) logout(c *gin.Context, channel string) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	if be := ctl.svc.LogoutChannel(c.Request.Context(), identity, identity.AccessExpiresAt, channel); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

// Info GET /auth/info
func (ctl *AuthController) Info(c *gin.Context) {
	resp, be := ctl.svc.Info(middleware.CurrentIdentity(c))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, resp)
}

// Routes GET /auth/routes
func (ctl *AuthController) Routes(c *gin.Context) {
	nodes, be := ctl.svc.Routes(middleware.CurrentIdentity(c))
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nodes)
}
