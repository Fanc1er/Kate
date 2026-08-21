package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func registerAssets(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/assets")

	// 资产列表。
	g.GET("", func(c *gin.Context) {
		page := utils.ParsePage(c.Query("page"))
		pageSize := utils.ParsePageSize(c.Query("page_size"))
		list, total, err := d.Asset.List(
			orgID(c), c.Query("keyword"), c.Query("group_name"), c.Query("importance"), c.Query("status"),
			c.Query("source_type"), page, pageSize,
		)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})

	// 分组统计。
	g.GET("/groups", func(c *gin.Context) {
		list, err := d.Asset.Groups(orgID(c))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	// 创建资产。
	g.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Name       string `json:"name"`
			URL        string `json:"url"`
			GroupName  string `json:"group_name"`
			Importance string `json:"importance"`
			Remark     string `json:"remark"`
		}
		if !bindJSON(c, &req) {
			return
		}
		a, err := d.Asset.Create(orgID(c), req.Name, req.URL, req.GroupName, req.Importance, req.Remark,
			userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, a)
	})

	// 批量加入扫描。
	g.POST("/batch-scan", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if !bindJSON(c, &req) {
			return
		}
		started, err := d.Task.BatchScan(orgID(c), req.IDs, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, map[string]any{"started": started})
	})

	// 批量导入。
	g.POST("/batch-import", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res, err := d.Asset.BatchImport(orgID(c), req.Content, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, res)
	})

	// 批量删除。
	g.POST("/batch-delete", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res := d.Asset.BatchDelete(orgID(c), req.IDs, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		response.OK(c, res)
	})

	// 批量改分组。
	g.POST("/batch-group", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			IDs    []int64 `json:"ids"`
			Group  string  `json:"group_name"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res := d.Asset.BatchGroup(orgID(c), req.IDs, req.Group)
		response.OK(c, res)
	})

	// 导入模板下载。
	g.GET("/import-template", func(c *gin.Context) {
		tpl := "url,name,group,importance,remark\nhttps://example.com,示例站点,核心业务,high,\n"
		c.Header("Content-Disposition", "attachment; filename=asset_import_template.csv")
		c.Data(200, "text/csv; charset=utf-8", []byte(tpl))
	})

	// 当前筛选结果导出。
	g.GET("/export", func(c *gin.Context) {
		data, err := d.Asset.ExportCSV(orgID(c), c.Query("keyword"), c.Query("group_name"), c.Query("importance"), c.Query("status"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		c.Header("Content-Disposition", "attachment; filename=assets.csv")
		c.Data(200, "text/csv; charset=utf-8", data)
	})

	// 资产详情。
	g.GET("/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		a, err := d.Asset.Get(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, a)
	})

	// 资产画像。
	g.GET("/:id/profile", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		p, err := d.Asset.Profile(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, p)
	})

	// 资产变更追踪。
	g.GET("/:id/history", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		logs, err := d.Asset.History(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, logs)
	})

	// 更新资产。
	g.PUT("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		var patch map[string]any
		if !bindJSON(c, &patch) {
			return
		}
		a, err := d.Asset.Update(orgID(c), id, patch, 0, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, a)
	})

	// 删除资产。
	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		if err := d.Asset.Delete(orgID(c), id, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 微信公众号资产。
	wechat := rg.Group("/wechat-assets")
	wechat.GET("", func(c *gin.Context) {
		list, total, err := d.Asset.ListWechat(orgID(c), utils.ParsePage(c.Query("page")), utils.ParsePageSize(c.Query("page_size")))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	wechat.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var m map[string]any
		if !bindJSON(c, &m) {
			return
		}
		a, err := d.Asset.CreateWechat(orgID(c), m)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, a)
	})
	wechat.PUT("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		var m map[string]any
		if !bindJSON(c, &m) {
			return
		}
		a, err := d.Asset.UpdateWechat(orgID(c), id, m)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, a)
	})
	wechat.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		if err := d.Asset.DeleteWechat(orgID(c), id); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}

var _ = models.StatusActive
