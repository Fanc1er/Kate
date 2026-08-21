package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func registerTasks(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/tasks")

	// 任务列表。
	g.GET("", func(c *gin.Context) {
		list, total, err := d.Task.List(orgID(c), c.Query("status"), utils.ParsePage(c.Query("page")), utils.ParsePageSize(c.Query("page_size")))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})

	// 队列监控。
	g.GET("/queue", func(c *gin.Context) {
		m, err := d.Task.Queue(orgID(c))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	// 创建任务。
	g.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var req service.TaskCreateReq
		if !bindJSON(c, &req) {
			return
		}
		created, err := d.Task.Create(orgID(c), req, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, created)
	})

	// 批量停止。
	g.POST("/batch-stop", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res := d.Task.BatchStop(orgID(c), req.IDs, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		response.OK(c, res)
	})

	// 批量重跑。
	g.POST("/batch-rerun", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res := d.Task.BatchRerun(orgID(c), req.IDs, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		response.OK(c, res)
	})

	// 任务详情。
	g.GET("/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		t, err := d.Task.Get(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, t)
	})

	// 断点续扫状态。
	g.GET("/:id/progress", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		m, err := d.Task.Progress(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	// 停止任务。
	g.POST("/:id/stop", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		t, err := d.Task.Stop(orgID(c), id, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, t)
	})

	// 失败重跑。
	g.POST("/:id/rerun", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		t, err := d.Task.Rerun(orgID(c), id, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, t)
	})

	// 删除任务。
	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		if err := d.Task.Delete(orgID(c), id, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}

func registerPolicies(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/policies")

	g.GET("", func(c *gin.Context) {
		list, err := d.Policy.List(orgID(c))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	g.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var p service.PolicyInput
		if !bindJSON(c, &p) {
			return
		}
		pol, err := d.Policy.Create(orgID(c), p.ToModel(), userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, pol)
	})

	g.PUT("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		var p service.PolicyInput
		if !bindJSON(c, &p) {
			return
		}
		pol, err := d.Policy.Update(orgID(c), id, p.ToModel(), userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, pol)
	})

	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		if err := d.Policy.Delete(orgID(c), id, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}
