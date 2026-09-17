package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"anxuncloud/internal/pkg/logger"
)

// CtxRequestID gin ctx 键：请求 ID（AccessLog 入口生成，业务/日志排障串联用）。
const CtxRequestID = "request_id"

// accessLogSkip 静默路径（探针高频噪声，不记访问日志）。
var accessLogSkip = map[string]struct{}{
	"/healthz": {},
	"/readyz":  {},
}

// AccessLog 访问日志 + Request-ID：入口生成 X-Request-ID（uuid）写 gin ctx 并回写响应头；
// 请求结束记录 method/path/status/latency_ms/user_id/client_ip。OPTIONS 与探针路径跳过日志。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := uuid.NewString()
		c.Set(CtxRequestID, reqID)
		c.Header("X-Request-ID", reqID)

		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		if _, skip := accessLogSkip[c.Request.URL.Path]; skip {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		logger.L.Info("access",
			zap.String("request_id", reqID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("user_id", CurrentUserID(c)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
