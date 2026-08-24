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

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/middleware"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/search"
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
	Worker       *service.WorkerService
	Dashboard    *service.DashboardService
	Availability *service.AvailabilityService
	Member       *service.MemberService
	Report    *service.ReportService
	Tokens    *service.TokenManager
	Security  *middleware.Security
	License   *license.Manager
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
	// 授权接口：无门禁（机器码/状态/导入）。
	registerLicense(api, d)

	// 授权门禁：以下所有接口要求有效授权。
	gated := api.Group("", d.Security.LicenseRequired())
	registerAuth(gated, d)
	registerWorker(gated, d)

	// WebSocket 实时事件：握手认证由 ServeWS 自行处理（浏览器 WS 只能传 query token）。
	gated.GET("/ws/events", d.Hub.ServeWS(d.Tokens))

	// 认证后（仅 JWT）：
	authed := gated.Group("", d.Security.AuthRequired())
	authed.GET("/auth/me", func(c *gin.Context) {
		u, err := d.Auth.Me(c.GetInt64("user_id"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, u)
	})

	// 平台管理（仅管理员）。
	registerAdmin(authed, d)

	// 业务路由（认证 + RBAC）。
	registerAssets(authed, d)
	registerTasks(authed, d)
	registerPolicies(authed, d)
	registerTriage(authed, d)
	registerReports(authed, d)
	registerEvidence(authed, d)
	registerDashboard(authed, d)
	registerAvailability(authed, d)
	registerMembers(authed, d)
	registerWorkerNodes(authed, d)
	search.RegisterRoutes(authed)

	if d.Seed != nil && !d.Seed.IsInitialized() {
		// 未初始化时平台管理路由单独暴露。
		initGroup := gated.Group("/init")
		initGroup.POST("", d.Security.AuthRequired(), d.Security.RequireAdmin(), func(c *gin.Context) {
			initialized, pwd, err := d.Seed.EnsureAdmin()
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
