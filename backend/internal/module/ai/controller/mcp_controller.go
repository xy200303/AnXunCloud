package controller

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/ai/service"
	"anxuncloud/internal/pkg/bind"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/response"
)

type MCPController struct {
	svc *service.MCPService
	db  *gorm.DB
}

func NewMCPController(svc *service.MCPService, db *gorm.DB) *MCPController {
	return &MCPController{svc: svc, db: db}
}

func (ctl *MCPController) actor(c *gin.Context) (service.Actor, *errs.Error) {
	tenantID, be := middleware.EffectiveTenantID(c, ctl.db)
	if be != nil {
		return service.Actor{}, be
	}
	return service.Actor{TenantID: tenantID, UserID: middleware.CurrentUserID(c)}, nil
}

func (ctl *MCPController) Tokens(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	tokens, be := ctl.svc.Tokens(actor)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, tokens)
}

func (ctl *MCPController) Tools(c *gin.Context) {
	response.OK(c, service.ToolCatalog())
}

func (ctl *MCPController) Issue(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	var req struct {
		Name   string   `json:"name" binding:"required"`
		Scopes []string `json:"scopes" binding:"required"`
		Days   int      `json:"days"`
	}
	if be := bind.JSON(c, &req); be != nil {
		response.Fail(c, be)
		return
	}
	data, be := ctl.svc.Issue(actor, req.Name, req.Scopes, req.Days)
	if be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, data)
}

func (ctl *MCPController) Revoke(c *gin.Context) {
	actor, be := ctl.actor(c)
	if be != nil {
		response.Fail(c, be)
		return
	}
	if be := ctl.svc.Revoke(actor, c.Param("id")); be != nil {
		response.Fail(c, be)
		return
	}
	response.OK(c, nil)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (ctl *MCPController) RPC(c *gin.Context) {
	secret := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	var req rpcRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.JSONRPC != "2.0" || secret == "" {
		rpcError(c, 400, req.ID, -32600, "无效的 JSON-RPC 请求")
		return
	}
	switch req.Method {
	case "initialize":
		if be := ctl.svc.Validate(secret); be != nil {
			rpcError(c, 401, req.ID, -32001, be.Msg)
			return
		}
		c.JSON(200, gin.H{"jsonrpc": "2.0", "id": req.ID, "result": gin.H{"protocolVersion": "2025-03-26", "serverInfo": gin.H{"name": "AI云管家", "version": "1.0"}, "capabilities": gin.H{"tools": gin.H{}}}})
	case "tools/list":
		if be := ctl.svc.Validate(secret); be != nil {
			rpcError(c, 401, req.ID, -32001, be.Msg)
			return
		}
		c.JSON(200, gin.H{"jsonrpc": "2.0", "id": req.ID, "result": gin.H{"tools": service.ToolCatalog()}})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal(req.Params, &params) != nil {
			rpcError(c, 400, req.ID, -32602, "工具参数无效")
			return
		}
		result, be := ctl.svc.Call(secret, params.Name, params.Arguments)
		if be != nil {
			status := be.HTTP
			if status == 0 {
				status = 403
			}
			rpcError(c, status, req.ID, -32003, be.Msg)
			return
		}
		content, _ := json.Marshal(result)
		c.JSON(200, gin.H{"jsonrpc": "2.0", "id": req.ID, "result": gin.H{"content": []gin.H{{"type": "text", "text": string(content)}}, "isError": false}})
	default:
		rpcError(c, 404, req.ID, -32601, "方法不存在")
	}
}

func rpcError(c *gin.Context, status int, id json.RawMessage, code int, message string) {
	c.JSON(status, gin.H{"jsonrpc": "2.0", "id": id, "error": gin.H{"code": code, "message": message}})
}
