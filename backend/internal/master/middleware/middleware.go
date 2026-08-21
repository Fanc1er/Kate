// Package middleware 提供请求追踪、认证、RBAC、组织隔离等 Gin 中间件。
package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/masker"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// RequestID 生成并注入 request_id，逐请求记录结构化访问日志。
func RequestID(logger func(level, msg string, fields map[string]any)) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		start := time.Now()
		c.Next()
		latency := time.Since(start).Milliseconds()
		fields := map[string]any{
			"ts":         time.Now().Format(time.RFC3339),
			"level":      "info",
			"path":       c.Request.URL.Path,
			"method":     c.Request.Method,
			"status":     c.Writer.Status(),
			"latency_ms": latency,
			"ip":         c.ClientIP(),
			"ua":         LogSensitive(c.Request.UserAgent()),
		}
		if uid, ok := c.Get("user_id"); ok {
			fields["user_id"] = uid
		}
		if org, ok := c.Get("org_id"); ok {
			fields["org_id"] = org
		}
		fields["request_id"] = rid
		if logger != nil {
			logger("info", "request", fields)
		}
	}
}

// Recovery 捕获 panic 并返回统一错误响应。
func Recovery(logger func(level, msg string, fields map[string]any)) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				if logger != nil {
					logger("error", "panic recovered", map[string]any{"panic": r, "request_id": c.GetString("request_id")})
				}
				response.FailCode(c, 500)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// CORS 允许前端跨域（代理场景其实无需，但保留兼容）。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Org-Id, X-Request-ID, If-Match")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// SensitiveHeader 敏感响应头（安全加固）。
func SensitiveHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// LogSensitive 记录日志前对敏感字段脱敏。
func LogSensitive(v string) string {
	return masker.MaskString(v)
}

// BearerToken 从 Authorization 头提取 token。
func BearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// MakeAPIError 构造 APIError（供中间件使用）。
func MakeAPIError(code int) *errs.APIError {
	return errs.New(code, "")
}
