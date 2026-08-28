package routes

import (
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
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
		data, filename, err := d.Report.Export(p)
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

	// 报告存档列表。
	g.GET("", func(c *gin.Context) {
		list, err := d.Report.ListReports()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	// 报告存档下载。
	g.GET("/:id/download", func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的报告 ID")
			return
		}
		data, filename, err := d.Report.Download(id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Data(200, "application/pdf", data)
	})

	// 报告存档删除。
	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的报告 ID")
			return
		}
		if err := d.Report.DeleteReport(id); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeReportAudit(c, d, "report.delete", id)
		response.OK(c, nil)
	})

	// 定时报告模板管理。
	tg := rg.Group("/report-templates")

	tg.GET("", func(c *gin.Context) {
		list, err := d.Report.ListTemplates()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	tg.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Name     string `json:"name"`
			Period   string `json:"period"`
			CronExpr string `json:"cron_expr"`
			Timezone string `json:"timezone"`
			Enabled  bool   `json:"enabled"`
		}
		if !bindJSON(c, &req) {
			return
		}
		tpl, err := d.Report.CreateTemplate(req.Name, req.Period, req.CronExpr, req.Timezone, req.Enabled)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeReportAudit(c, d, "report.template.create", tpl.ID)
		response.OK(c, tpl)
	})

	tg.PUT("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的模板 ID")
			return
		}
		var req struct {
			Name     *string `json:"name"`
			CronExpr *string `json:"cron_expr"`
			Timezone *string `json:"timezone"`
			Enabled  *bool   `json:"enabled"`
		}
		if !bindJSON(c, &req) {
			return
		}
		tpl, err := d.Report.UpdateTemplate(id, req.Name, req.CronExpr, req.Timezone, req.Enabled)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeReportAudit(c, d, "report.template.update", id)
		response.OK(c, tpl)
	})

	tg.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的模板 ID")
			return
		}
		if err := d.Report.DeleteTemplate(id); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeReportAudit(c, d, "report.template.delete", id)
		response.OK(c, nil)
	})

	// 立即按模板生成一份报告。
	tg.POST("/:id/run", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的模板 ID")
			return
		}
		var tpl models.ReportTemplate
		if err := d.Report.DB.First(&tpl, id).Error; err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		rep, err := d.Report.Generate(&tpl, time.Now())
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		if err := d.Report.DB.Model(&tpl).Updates(map[string]any{"last_run_at": time.Now()}).Error; err != nil {
			log.Printf("report-run: update last_run_at: %v", err)
		}
		writeReportAudit(c, d, "report.generate", rep.ID)
		response.OK(c, rep)
	})
}

// writeReportAudit 记录报告管理审计（Audit 未装配时静默跳过）。
func writeReportAudit(c *gin.Context, d *Deps, action string, resourceID int64) {
	if d.Report.Audit == nil {
		return
	}
	d.Report.Audit.Write(userID(c), usernameOf(c), action, "report", strconv.FormatInt(resourceID, 10), "", "", c.ClientIP(), c.GetHeader("User-Agent"))
}
