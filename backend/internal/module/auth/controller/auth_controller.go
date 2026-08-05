// Package controller 认证接口 HTTP 层。
package controller

import (
	"github.com/gin-gonic/gin"

	"property-inspection/internal/middleware"
	"property-inspection/internal/module/auth/dto"
	"property-inspection/internal/module/auth/service"
	"property-inspection/internal/pkg/bind"
	"property-inspection/internal/pkg/errs"
	"property-inspection/internal/pkg/response"
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
