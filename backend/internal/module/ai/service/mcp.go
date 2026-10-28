package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"anxuncloud/internal/middleware"
	"anxuncloud/internal/module/ai/model"
	"anxuncloud/internal/pkg/authz"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/types"
)

type MCPService struct {
	db      *gorm.DB
	summary func(*gin.Context, time.Time, time.Time) (gin.H, error)
}

func NewMCPService(db *gorm.DB, summary func(*gin.Context, time.Time, time.Time) (gin.H, error)) *MCPService {
	return &MCPService{db: db, summary: summary}
}

func ToolCatalog() []gin.H {
	return []gin.H{{
		"name": "inspection.summary", "title": "巡检任务汇总", "read_only": true,
		"description":  "按日期范围统计任务、完成、逾期和异常数，继承当前用户数据权限",
		"input_schema": gin.H{"type": "object", "properties": gin.H{"start_date": gin.H{"type": "string", "format": "date"}, "end_date": gin.H{"type": "string", "format": "date"}, "community_id": gin.H{"type": "string", "format": "uuid"}}},
	}}
}

func (s *MCPService) Issue(actor Actor, name string, scopes []string, days int) (gin.H, *errs.Error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 128 || days < 1 || days > 365 || len(scopes) == 0 {
		return nil, errs.ErrParam.WithMsg("名称、有效期和工具范围必填")
	}
	for _, scope := range scopes {
		if scope != "inspection.summary" {
			return nil, errs.ErrParam.WithMsg("包含未注册的工具范围")
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, errs.ErrInternal
	}
	secret := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(secret))
	expires := time.Now().AddDate(0, 0, days)
	token := model.MCPToken{TenantID: actor.TenantID, UserID: actor.UserID, Name: name, Prefix: secret[:12], SecretHash: hex.EncodeToString(digest[:]), Scopes: types.StringArray(scopes), ExpiresAt: &expires}
	if err := s.db.Create(&token).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return gin.H{"id": token.ID, "name": token.Name, "token": secret, "prefix": token.Prefix, "scopes": token.Scopes, "expires_at": token.ExpiresAt}, nil
}

func (s *MCPService) Tokens(actor Actor) ([]model.MCPToken, *errs.Error) {
	var tokens []model.MCPToken
	if err := s.db.Where("tenant_id = ?", actor.TenantID).Order("created_at DESC").Limit(100).Find(&tokens).Error; err != nil {
		return nil, errs.ErrInternal
	}
	return tokens, nil
}

func (s *MCPService) Revoke(actor Actor, id string) *errs.Error {
	now := time.Now()
	result := s.db.Model(&model.MCPToken{}).Where("id = ? AND tenant_id = ? AND revoked_at IS NULL", id, actor.TenantID).Update("revoked_at", now)
	if result.Error != nil {
		return errs.ErrInternal
	}
	if result.RowsAffected != 1 {
		return errs.ErrConflict.WithMsg("token 不存在或已撤销")
	}
	return nil
}

func (s *MCPService) authenticate(secret string) (*model.MCPToken, *middleware.Identity, *errs.Error) {
	digest := sha256.Sum256([]byte(secret))
	var token model.MCPToken
	if err := s.db.Where("secret_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", hex.EncodeToString(digest[:]), time.Now()).First(&token).Error; err != nil {
		return nil, nil, errs.ErrUnauthorized
	}
	identity, err := middleware.IdentityForUser(s.db, token.UserID)
	if err != nil || identity.TenantID != token.TenantID {
		return nil, nil, errs.ErrUnauthorized
	}
	now := time.Now()
	s.db.Model(&token).Update("last_used_at", &now)
	return &token, identity, nil
}

func (s *MCPService) Call(secret, tool string, params map[string]any) (any, *errs.Error) {
	token, identity, be := s.authenticate(secret)
	if be != nil {
		return nil, be
	}
	return s.execute(token.TenantID, tool, identity, params)
}

func (s *MCPService) Execute(actor Actor, tool string, params map[string]any) (any, *errs.Error) {
	identity, err := middleware.IdentityForUser(s.db, actor.UserID)
	if err != nil || identity.TenantID != actor.TenantID {
		return nil, errs.ErrUnauthorized
	}
	if !(types.IDArray{"inspection.summary"}).Contains(tool) {
		return nil, errs.ErrNoPerm.WithMsg("未注册的工具范围")
	}
	return s.execute(actor.TenantID, tool, identity, params)
}

func (s *MCPService) execute(tenantID, tool string, identity *middleware.Identity, params map[string]any) (any, *errs.Error) {
	if tool != "inspection.summary" {
		return nil, errs.ErrNoPerm.WithMsg("未注册的工具范围")
	}
	allowed, err := authz.EnforceAny(identity.UserID, "inspection:task:monitor")
	if err != nil || !allowed {
		return nil, errs.ErrNoPerm
	}
	start, end := time.Now(), time.Now().AddDate(0, 0, 1)
	if value, _ := params["start_date"].(string); value != "" {
		start, err = time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("start_date 格式无效")
		}
	}
	if value, _ := params["end_date"].(string); value != "" {
		end, err = time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			return nil, errs.ErrParam.WithMsg("end_date 格式无效")
		}
	}
	if !end.After(start) || end.Sub(start) > 32*24*time.Hour {
		return nil, errs.ErrParam.WithMsg("查询范围须为开始日期之后且不超过 32 天")
	}
	c := &gin.Context{}
	middleware.SetIdentity(c, identity)
	c.Request = &http.Request{Header: http.Header{"X-Tenant-Id": []string{tenantID}}}
	result, err := s.summary(c, start, end)
	if err != nil {
		return nil, errs.ErrInternal
	}
	return result, nil
}

func (s *MCPService) Validate(secret string) *errs.Error {
	_, _, be := s.authenticate(secret)
	return be
}
