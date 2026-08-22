package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

func registerAvailability(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/availability")

	g.GET("/list", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		m, err := d.Availability.List(c.Query("keyword"), c.Query("status"), c.Query("status_code_group"), page, pageSize, c.Query("sort"), c.Query("sort_order"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	g.GET("/:id/timeseries", func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "无效的资产 ID"))
			return
		}
		hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
		pts, err := d.Availability.Timeseries(id, hours)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, pts)
	})

	g.GET("/worker-topology", func(c *gin.Context) {
		m, err := d.Availability.WorkerTopology()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	g.POST("/reprobe", func(c *gin.Context) {
		var req struct {
			AssetIDs []int64 `json:"asset_ids"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if len(req.AssetIDs) == 0 {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "请选择资产"))
			return
		}
		n, err := d.Availability.Reprobe(req.AssetIDs)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, map[string]any{"queued": n})
	})

	g.GET("/whitelist", func(c *gin.Context) {
		list, err := d.Availability.Whitelist()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	g.POST("/whitelist", func(c *gin.Context) {
		var req struct {
			Kind   string `json:"kind"`
			Value  string `json:"value"`
			Remark string `json:"remark"`
		}
		if !bindJSON(c, &req) {
			return
		}
		rule, err := d.Availability.AddWhitelist(req.Kind, req.Value, req.Remark)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, rule)
	})

	g.DELETE("/whitelist/:id", func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "无效的规则 ID"))
			return
		}
		if err := d.Availability.RemoveWhitelist(id); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}
