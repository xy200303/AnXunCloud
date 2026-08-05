// Package jwtutil 封装 JWT 双令牌（access + refresh）的签发与解析。
package jwtutil

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType 令牌用途。
const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

// Claims 自定义载荷。
type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"username"`
	Type     string `json:"type"` // access / refresh
	jwt.RegisteredClaims
}

// Manager JWT 签发与校验。
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(secret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// AccessTTL 返回 access token 有效期。
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL 返回 refresh token 有效期。
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// Generate 签发指定类型令牌，返回 token 串与 jti。
func (m *Manager) Generate(userID string, username, tokenType string) (token string, jti string, err error) {
	ttl := m.accessTTL
	if tokenType == TypeRefresh {
		ttl = m.refreshTTL
	}
	jti = uuid.NewString()
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return token, jti, err
}

// errExpired 供上层区分「过期」与「无效」。
var ErrExpired = errors.New("token expired")

// Parse 解析并校验令牌；过期返回 ErrExpired。
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return claims, ErrExpired
		}
		return nil, err
	}
	return claims, nil
}
