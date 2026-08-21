package routes

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// registerReports 报告导出接口（所有角色开放，基于生成时刻数据快照）。
func registerReports(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/reports")
	g.GET("/export", func(c *gin.Context) {
		format := c.Query("format")
		if format == "" {
			format = "pdf"
		}
		p := service.ExportParams{Format: format}
		if v := c.Query("asset_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil || id <= 0 {
				response.FailMsg(c, errs.CodeValidationFailed, "asset_id 非法")
				return
			}
			p.AssetID = id
		}
		p.Severity = c.Query("severity")
		p.Status = c.Query("status")
		parseTime := func(q string) *time.Time {
			if q == "" {
				return nil
			}
			t, err := time.Parse("2006-01-02", q)
			if err != nil {
				return nil
			}
			return &t
		}
		p.From = parseTime(c.Query("from"))
		p.To = parseTime(c.Query("to"))
		data, filename, err := d.Report.Export(orgID(c), p)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		ct := "application/pdf"
		if format == "excel" {
			ct = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
		c.Data(200, ct, data)
	})
}
