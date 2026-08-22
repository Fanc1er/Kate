package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// Security 组合认证与 RBAC 所需依赖。
type Security struct {
	DB      *gorm.DB
	Tokens  *service.TokenManager
	License *license.Manager
}

// NewSecurity 构造 Security。
func NewSecurity(db *gorm.DB, tokens *service.TokenManager, lic *license.Manager) *Security {
	return &Security{DB: db, Tokens: tokens, License: lic}
}

// LicenseRequired 校验授权有效，非有效时按状态返回对应错误码。
func (s *Security) LicenseRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch s.License.Check() {
		case license.StatusValid:
			c.Next()
		case license.StatusNotYetActive:
			response.FailCode(c, errs.CodeLicenseNotYetActive)
			c.Abort()
		case license.StatusExpired:
			response.FailCode(c, errs.CodeLicenseExpired)
			c.Abort()
		case license.StatusMachineMismatch:
			response.FailCode(c, errs.CodeLicenseMachineMismatch)
			c.Abort()
		default:
			response.FailCode(c, errs.CodeLicenseRequired)
			c.Abort()
		}
	}
}

// AuthRequired 校验 Bearer JWT 与用户状态，注入 user_id / role / username。
func (s *Security) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := BearerToken(c)
		if token == "" {
			response.FailCode(c, errs.CodeAuthFailed)
			c.Abort()
			return
		}
		claims, err := s.Tokens.Parse(token)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			c.Abort()
			return
		}
		// 校验用户状态（禁用用户已签发 JWT 失效）。
		var user models.User
		if err := s.DB.First(&user, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.FailCode(c, errs.CodeAuthFailed)
			} else {
				response.FailCode(c, 500)
			}
			c.Abort()
			return
		}
		if user.Status == models.StatusDisabled {
			response.FailCode(c, errs.CodeUserDisabled)
			c.Abort()
			return
		}
		c.Set("user_id", user.ID)
		c.Set("role", user.Role)
		c.Set("username", user.Username)
		c.Next()
	}
}

// RequireRoles 校验当前角色在允许列表内。
func (s *Security) RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role := c.GetString("role")
		if !allowed[role] {
			response.FailCode(c, errs.CodeForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdmin 仅管理员角色（用户管理、系统配置）。
func (s *Security) RequireAdmin() gin.HandlerFunc {
	return s.RequireRoles(models.RoleAdmin)
}

// RequireWrite 允许任意已认证用户（admin/user）执行业务写操作。
func (s *Security) RequireWrite() gin.HandlerFunc {
	return s.RequireRoles(models.RoleAdmin, models.RoleUser)
}

// AdminOnly 兼容旧命名，等价于 RequireAdmin。
func (s *Security) AdminOnly() gin.HandlerFunc {
	return s.RequireAdmin()
}

// ClientIP 取客户端 IP。
func ClientIP(c *gin.Context) string {
	return strings.TrimSpace(c.ClientIP())
}

var _ = http.MethodGet
