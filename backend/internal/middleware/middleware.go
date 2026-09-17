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

	"anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/safe"
	"anxuncloud/internal/pkg/logger"
	"go.uber.org/zap"
)

// actionNames 操作日志动作中文名映射（key 为 action；新增操作类型在此补充，漏配回退原始 key）。
var actionNames = map[string]string{
	"create":          "新增",
	"update":          "修改",
	"delete":          "删除",
	"update_status":   "启停用",
	"reset_password":  "重置密码",
	"import":          "导入",
	"export":          "导出",
	"assign_menus":    "分配权限",
	"qrcode":          "生成二维码",
	"generate":        "生成任务",
	"assign":          "派单",
	"finish":          "处理反馈",
	"review":          "复核",
	"update_profile":  "修改资料",
	"change_password": "修改密码",
	"push_send":       "消息推送",
}

// ActionName 操作动作中文名（操作日志列表展示用）。
func ActionName(action string) string {
	if name, ok := actionNames[action]; ok {
		return name
	}
	return action
}

// 敏感字段脱敏（密钥类参数不落日志；覆盖 *password*/*secret*/*api_key*/*apikey*/*token*/*credential* 键，大小写不敏感）
var sensitiveRe = regexp.MustCompile(`(?i)("\w*(?:password|secret|api_?key|token|credential)\w*"\s*:\s*")[^"]*(")`)

// configValueRe 系统配置（/system/configs）的 value 特判：value 可能是 AI key 等密钥，日志只留 key 不留值。
var configValueRe = regexp.MustCompile(`("value"\s*:\s*")[^"]*(")`)

// maskParams 操作日志参数脱敏：通用敏感键打码；/system/configs 的 value 键一律打码（密钥不区分键名）。
func maskParams(path string, params []byte) string {
	masked := sensitiveRe.ReplaceAllString(string(params), "${1}******${2}")
	if strings.Contains(path, "/system/configs") {
		masked = configValueRe.ReplaceAllString(masked, "${1}******${2}")
	}
	return masked
}

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
		masked := maskParams(c.Request.URL.Path, params)
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
			// 租户隔离：日志管理按租户上下文过滤，写入即带操作者所属租户
			log.TenantID = &identity.TenantID
		}
		// 异步落库，避免阻塞请求；用 Background 防止请求结束取消写入
		safe.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := db.WithContext(ctx).Create(&log).Error; err != nil {
				logger.L.Warn("写入操作日志失败", zap.Error(err), zap.String("path", log.Path))
			}
		})
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
		// 响应随 Origin 变化（白名单命中才反射 Origin），声明 Vary 防止共享缓存串源
		c.Header("Vary", "Origin")
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
