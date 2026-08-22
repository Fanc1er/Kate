package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// Security 组合认证与 RBAC 所需依赖。
type Security struct {
	DB     *gorm.DB
	Tokens *service.TokenManager
}

// NewSecurity 构造 Security。
func NewSecurity(db *gorm.DB, tokens *service.TokenManager) *Security {
	return &Security{DB: db, Tokens: tokens}
}

// AuthRequired 校验 Bearer JWT 与用户状态，注入 user_id / is_super_admin。
// 仅依赖 JWT，不需要 X-Org-Id（me 接口使用）。
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
		c.Set("is_super_admin", user.IsSuperAdmin)
		c.Set("username", user.Username)
		c.Next()
	}
}

// OrgRequired 校验 X-Org-Id 与成员关系，注入 org_id / role。
// super_admin 允许 org_id=0 走平台查询通道。
func (s *Security) OrgRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgIDStr := c.GetHeader("X-Org-Id")
		if orgIDStr == "" {
			response.FailCode(c, errs.CodeOrgRequired)
			c.Abort()
			return
		}
		orgID, err := strconv.ParseInt(orgIDStr, 10, 64)
		if err != nil || orgID < 0 {
			response.FailMsg(c, errs.CodeValidationFailed, "X-Org-Id 不合法")
			c.Abort()
			return
		}
		uid := c.GetInt64("user_id")
		isSuper := c.GetBool("is_super_admin")

		if isSuper {
			// 平台超管可访问全局通道（org_id=0）及任意组织视角。
			c.Set("org_id", orgID)
			c.Set("role", models.RoleSuperAdmin)
			c.Next()
			return
		}
		// 校验组织状态。
		var org models.Organization
		if err := s.DB.First(&org, orgID).Error; err != nil {
			response.FailCode(c, errs.CodeNotFound)
			c.Abort()
			return
		}
		if org.Status == models.StatusDisabled {
			response.FailCode(c, errs.CodeOrgDisabled)
			c.Abort()
			return
		}
		// 校验成员关系与状态。
		var uo models.UserOrg
		if err := s.DB.Where("user_id = ? AND org_id = ?", uid, orgID).First(&uo).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.FailCode(c, errs.CodeForbidden)
			} else {
				response.FailCode(c, 500)
			}
			c.Abort()
			return
		}
		if uo.Status == models.StatusDisabled {
			response.FailMsg(c, errs.CodeUserDisabled, "成员已被禁用")
			c.Abort()
			return
		}
		c.Set("org_id", orgID)
		c.Set("role", uo.Role)
		c.Set("org_name", org.Name)
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
		if role == models.RoleSuperAdmin {
			// 超管经 org_id=0 全局通道；若带 org 上下文由 OrgRequired 已拦截。
			if c.GetInt64("org_id") == 0 {
				c.Next()
				return
			}
		}
		if !allowed[role] {
			response.FailCode(c, errs.CodeForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireWrite 只读用户（viewer）禁止任何写操作。
func (s *Security) RequireWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role == models.RoleSuperAdmin {
			c.Next()
			return
		}
		if role == models.RoleViewer {
			response.FailCode(c, errs.CodeForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireSuperAdmin 平台管理通道（仅 super_admin）。
func (s *Security) RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !c.GetBool("is_super_admin") {
			response.FailCode(c, errs.CodeForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminOnly 仅 org_admin（管理类配置写操作）。
func (s *Security) AdminOnly() gin.HandlerFunc {
	return s.RequireRoles(models.RoleOrgAdmin)
}

// ClientIP 取客户端 IP。
func ClientIP(c *gin.Context) string {
	return strings.TrimSpace(c.ClientIP())
}

var _ = http.MethodGet
