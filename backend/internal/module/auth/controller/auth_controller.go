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
	var req dto.LoginReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	resp, be := ctl.svc.Login(c.Request.Context(), &req, c.ClientIP(), c.GetHeader("User-Agent"))
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
	var req dto.RefreshReq
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	resp, be := ctl.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, resp)
}

// Logout POST /auth/logout
func (ctl *AuthController) Logout(c *gin.Context) {
	identity := middleware.CurrentIdentity(c)
	if identity == nil {
		response.Fail(c, errs.ErrUnauthorized)
		return
	}
	if be := ctl.svc.Logout(c.Request.Context(), identity, identity.AccessExpiresAt); be != nil {
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
