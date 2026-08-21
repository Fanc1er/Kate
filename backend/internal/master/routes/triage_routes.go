package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func registerTriage(rg *gin.RouterGroup, d *Deps) {
	page := func(c *gin.Context) (int, int) {
		return utils.ParsePage(c.Query("page")), utils.ParsePageSize(c.Query("page_size"))
	}
	parseID := func(c *gin.Context) (int64, bool) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return 0, false
		}
		return id, true
	}
	meta := func(c *gin.Context) (int64, string, string, string) {
		return userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")
	}

	// ---------- findings ----------
	f := rg.Group("/findings")
	f.GET("", func(c *gin.Context) {
		p := service.FindingListParams{
			Status: c.Query("status"), Severity: c.Query("severity"), EngineName: c.Query("engine_name"),
			Type: c.Query("type"), Keyword: c.Query("keyword"),
		}
		p.Page, p.PageSize = page(c)
		if v, err := strconv.ParseInt(c.Query("asset_id"), 10, 64); err == nil {
			p.AssetID = v
		}
		if v, err := strconv.ParseInt(c.Query("task_id"), 10, 64); err == nil {
			p.TaskID = v
		}
		list, total, err := d.Triage.ListFindings(orgID(c), p)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	f.GET("/overview", func(c *gin.Context) {
		m, err := d.Triage.TriageOverview(orgID(c))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})
	f.GET("/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		detail, err := d.Triage.GetFindingDetail(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, detail)
	})
	f.PUT("/:id/status", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			Status string `json:"status"`
		}
		if !bindJSON(c, &req) {
			return
		}
		uid, uname, ip, ua := meta(c)
		fnd, err := d.Triage.UpdateFindingStatus(orgID(c), id, req.Status, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, fnd)
	})

	// ---------- events ----------
	e := rg.Group("/events")
	e.GET("", func(c *gin.Context) {
		p, ps := page(c)
		list, total, err := d.Triage.ListEvents(orgID(c), c.Query("status"), c.Query("event_type"), p, ps)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	e.PUT("/:id/status", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			Status string `json:"status"`
		}
		if !bindJSON(c, &req) {
			return
		}
		uid, uname, ip, ua := meta(c)
		if err := d.Triage.UpdateEventStatus(orgID(c), id, req.Status, uid, uname, ip, ua); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// ---------- alerts ----------
	a := rg.Group("/alerts")
	a.GET("", func(c *gin.Context) {
		p, ps := page(c)
		list, total, err := d.Triage.ListAlerts(orgID(c), c.Query("status"), c.Query("alert_type"), p, ps)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	a.POST("/:id/resolve", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		uid, uname, ip, ua := meta(c)
		if err := d.Triage.ResolveAlert(orgID(c), id, uid, uname, ip, ua); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// ---------- vulnerabilities ----------
	v := rg.Group("/vulnerabilities")
	v.GET("", func(c *gin.Context) {
		p, ps := page(c)
		assetID, _ := strconv.ParseInt(c.Query("asset_id"), 10, 64)
		list, total, err := d.Triage.ListVulns(orgID(c), c.Query("status"), c.Query("severity"), assetID, p, ps)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	v.GET("/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		detail, err := d.Triage.GetVulnDetail(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, detail)
	})
	v.GET("/:id/evidence", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		evs, err := d.Triage.GetVulnEvidence(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, evs)
	})
}
