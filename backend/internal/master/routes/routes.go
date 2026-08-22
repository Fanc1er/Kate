// Package routes 组装 Master HTTP 路由与控制器。
package routes

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/Fanc1er/Kate/backend/internal/master/routes/docs" // swag 生成文档

	"github.com/Fanc1er/Kate/backend/internal/master/middleware"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// Deps 路由依赖集合。
type Deps struct {
	DB        *gorm.DB
	Auth      *service.AuthService
	Seed      *service.SeedService
	Asset     *service.AssetService
	Task      *service.TaskService
	Policy    *service.PolicyService
	Triage    *service.TriageService
	Evidence  *service.EvidenceService
	Worker    *service.WorkerService
	Dashboard *service.DashboardService
	Member    *service.MemberService
	Report    *service.ReportService
	Tokens    *service.TokenManager
	Security  *middleware.Security
	Hub       *service.Hub
	Logger    func(level, msg string, fields map[string]any)
}

// Setup 构建完整路由树。
func Setup(d *Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS(), middleware.SensitiveHeader())
	if d.Logger != nil {
		r.Use(middleware.RequestID(d.Logger))
	} else {
		r.Use(middleware.RequestID(func(level, msg string, fields map[string]any) {
			log.Printf("[%s] %s %v", level, msg, fields)
		}))
	}
	r.Use(middleware.Recovery(d.Logger))

	// 健康检查。
	r.GET("/api/health", func(c *gin.Context) {
		response.OK(c, map[string]any{"name": "cinsight-master", "status": "ok", "version": "0.1.0"})
	})

	api := r.Group("/api/v1")
	registerAuth(api, d)
	registerWorker(api, d)

	// 认证后（仅 JWT，可无 org）：
	authed := api.Group("", d.Security.AuthRequired())
	authed.GET("/auth/me", func(c *gin.Context) {
		u, err := d.Auth.Me(c.GetInt64("user_id"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, u)
	})
	authed.POST("/auth/select-org", func(c *gin.Context) {
		var req struct {
			OrgID int64 `json:"org_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.OrgID <= 0 {
			response.FailMsg(c, errs.CodeValidationFailed, "org_id 必填")
			return
		}
		res, err := d.Auth.SelectOrg(c.GetInt64("user_id"), req.OrgID)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, res)
	})

	// 平台管理（仅超管，无 org 上下文）。
	registerAdmin(authed, d)

	// 组织上下文（X-Org-Id）+ RBAC。
	org := api.Group("", d.Security.AuthRequired(), d.Security.OrgRequired())
	registerAssets(org, d)
	registerTasks(org, d)
	registerPolicies(org, d)
	registerTriage(org, d)
	registerReports(org, d)
	registerEvidence(org, d)
	registerDashboard(org, d)
	registerMembers(org, d)
	registerWorkerNodes(org, d)

	if d.Seed != nil && !d.Seed.IsInitialized() {
		// 未初始化时平台管理路由单独暴露（无 org 上下文）。
		initGroup := api.Group("/init")
		initGroup.POST("", d.Security.AuthRequired(), func(c *gin.Context) {
			if !c.GetBool("is_super_admin") {
				response.FailCode(c, errs.CodeForbidden)
				return
			}
			initialized, pwd, err := d.Seed.EnsureSuperAdmin()
			if err != nil {
				response.Fail(c, errs.FromError(err))
				return
			}
			_ = d.Seed.EnsureDefaults()
			if initialized {
				response.OK(c, map[string]any{"initialized": true})
				return
			}
			response.OK(c, map[string]any{"initialized": false, "temp_password": pwd})
		})
	}

	// WebSocket 实时事件。
	org.GET("/ws/events", d.Hub.ServeWS(d.Tokens))

	// Swagger（可选）。
	if swaggerEnabled() {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	return r
}

// swaggerEnabled 读取 CINSIGHT_SWAGGER_ENABLED 开关（生产默认关闭）。
func swaggerEnabled() bool {
	return os.Getenv("CINSIGHT_SWAGGER_ENABLED") == "true"
}

// ctx helper
func orgID(c *gin.Context) int64 {
	return c.GetInt64("org_id")
}

func roleOf(c *gin.Context) string {
	return c.GetString("role")
}

func userID(c *gin.Context) int64 {
	return c.GetInt64("user_id")
}

func usernameOf(c *gin.Context) string {
	return c.GetString("username")
}

func clientMeta(c *gin.Context) (string, string) {
	return c.ClientIP(), c.GetHeader("User-Agent")
}

// bindJSON 绑定失败统一 1000。
func bindJSON(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		response.FailMsg(c, errs.CodeValidationFailed, "请求体不合法: "+err.Error())
		return false
	}
	return true
}

// parseIDParam 解析 URL 路径中的 int64 参数。
func parseIDParam(c *gin.Context, name string) (int64, error) {
	return strconv.ParseInt(c.Param(name), 10, 64)
}

var _ = http.MethodGet
var _ = models.StatusActive
var _ = strings.TrimSpace
