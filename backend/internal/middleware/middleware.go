package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"property-inspection/internal/module/system/model"
	"property-inspection/internal/pkg/logger"
	"go.uber.org/zap"
)

// 敏感字段脱敏（密码类参数不落日志）
var sensitiveRe = regexp.MustCompile(`("(?:new_)?password"\s*:\s*")[^"]*(")`)

// OperLog 操作日志中间件：请求结束后异步写入 sys_operation_log。
// module/action 语义见接口文档 §2.9（如 system / create）。
func OperLog(db *gorm.DB, module, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// 读取请求体用于留痕，随后恢复供后续绑定使用；仅 JSON 请求体留痕（multipart 体大且为二进制，跳过）
		var params []byte
		isJSON := strings.HasPrefix(c.GetHeader("Content-Type"), "application/json")
		if c.Request.Body != nil && c.Request.Method != http.MethodGet && isJSON {
			params, _ = io.ReadAll(io.LimitReader(c.Request.Body, 4096))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(params))
		}
		c.Next()

		status := "success"
		if c.Writer.Status() >= 400 {
			status = "fail"
		}
		masked := sensitiveRe.ReplaceAllString(string(params), "${1}******${2}")
		log := model.SysOperationLog{
			Username: "-",
			Module:   module,
			Action:   action,
			Method:   c.Request.Method,
			Path:     c.Request.URL.Path,
			Params:   masked,
			IP:       c.ClientIP(),
			Status:   status,
			CostMs:   int(time.Since(start).Milliseconds()),
		}
		if identity := CurrentIdentity(c); identity != nil {
			log.UserID = &identity.UserID
			log.Username = identity.Username
		}
		// 异步落库，避免阻塞请求；用 Background 防止请求结束取消写入
		go func(rec model.SysOperationLog) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := db.WithContext(ctx).Create(&rec).Error; err != nil {
				logger.L.Warn("写入操作日志失败", zap.Error(err), zap.String("path", rec.Path))
			}
		}(log)
	}
}

// CORS 跨域中间件。
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowAll := len(allowOrigins) == 0
	for _, o := range allowOrigins {
		if o == "*" {
			allowAll = true
		}
	}
	allowed := map[string]struct{}{}
	for _, o := range allowOrigins {
		allowed[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Token")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Recovery 统一 panic 恢复：记录堆栈并返回 50000。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.L.Error("请求发生 panic",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
					zap.Stack("stack"),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    50000,
					"message": "服务器内部错误",
					"data":    nil,
				})
			}
		}()
		c.Next()
	}
}
