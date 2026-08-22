package routes

import (
	"strconv"
	"time"

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
		list, total, err := d.Triage.ListFindings(p)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	f.GET("/overview", func(c *gin.Context) {
		m, err := d.Triage.TriageOverview()
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
		detail, err := d.Triage.GetFindingDetail(id)
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
		fnd, err := d.Triage.UpdateFindingStatus(id, req.Status, uid, uname, ip, ua)
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
		list, total, err := d.Triage.ListEvents(c.Query("status"), c.Query("event_type"), p, ps)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	e.GET("/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		detail, err := d.Triage.GetEventDetail(id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, detail)
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
		if err := d.Triage.UpdateEventStatus(id, req.Status, uid, uname, ip, ua); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
	e.POST("/batch/status", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Ids    []int64 `json:"ids"`
			Status string  `json:"status"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if len(req.Ids) == 0 || len(req.Ids) > 500 {
			response.FailMsg(c, errs.CodeValidationFailed, "ids 需 1-500 条")
			return
		}
		uid, uname, ip, ua := meta(c)
		m, err := d.Triage.BatchUpdateEventStatus(req.Ids, req.Status, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	// ---------- alerts ----------
	a := rg.Group("/alerts")
	a.GET("", func(c *gin.Context) {
		p, ps := page(c)
		list, total, err := d.Triage.ListAlerts(c.Query("status"), c.Query("alert_type"), p, ps)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	a.GET("/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		detail, err := d.Triage.GetAlertDetail(id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, detail)
	})
	a.POST("/:id/resolve", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		uid, uname, ip, ua := meta(c)
		if err := d.Triage.ResolveAlert(id, uid, uname, ip, ua); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
	a.POST("/batch/resolve", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Ids []int64 `json:"ids"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if len(req.Ids) == 0 || len(req.Ids) > 500 {
			response.FailMsg(c, errs.CodeValidationFailed, "ids 需 1-500 条")
			return
		}
		uid, uname, ip, ua := meta(c)
		m, err := d.Triage.BatchResolveAlert(req.Ids, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	// ---------- vulnerabilities ----------
	v := rg.Group("/vulnerabilities")
	v.GET("", func(c *gin.Context) {
		p, ps := page(c)
		assetID, _ := strconv.ParseInt(c.Query("asset_id"), 10, 64)
		list, total, err := d.Triage.ListVulns(c.Query("status"), c.Query("severity"), assetID, p, ps)
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
		detail, err := d.Triage.GetVulnDetail(id)
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
		evs, err := d.Triage.GetVulnEvidence(id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, evs)
	})

	// ---------- tickets ----------
	t := rg.Group("/tickets")
	t.GET("", func(c *gin.Context) {
		p, ps := page(c)
		list, total, err := d.Triage.ListTickets(c.Query("status"), c.Query("source"), p, ps)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})
	t.GET("/sources", func(c *gin.Context) {
		m, err := d.Triage.ListTicketSources()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})
	t.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			EventID  int64      `json:"event_id"`
			VulnID   int64      `json:"vuln_id"`
			Assignee string     `json:"assignee"`
			Notes    string     `json:"notes"`
			DueAt    *time.Time `json:"due_at"`
		}
		if !bindJSON(c, &req) {
			return
		}
		uid, uname, ip, ua := meta(c)
		tk, err := d.Triage.CreateTicket(req.EventID, req.VulnID, req.Assignee, req.Notes, req.DueAt, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, tk)
	})
	t.GET("/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		detail, err := d.Triage.GetTicketDetail(id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, detail)
	})
	t.PUT("/:id/status", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			Status  string `json:"status"`
			Version int    `json:"version"`
		}
		if !bindJSON(c, &req) {
			return
		}
		uid, uname, ip, ua := meta(c)
		tk, err := d.Triage.UpdateTicketStatus(id, req.Status, req.Version, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, tk)
	})
	t.PUT("/:id/assign", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			Assignee string `json:"assignee"`
			Version  int    `json:"version"`
		}
		if !bindJSON(c, &req) {
			return
		}
		uid, uname, ip, ua := meta(c)
		tk, err := d.Triage.AssignTicket(id, req.Assignee, req.Version, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, tk)
	})
	t.POST("/batch/status", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Ids    []int64 `json:"ids"`
			Status string  `json:"status"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if len(req.Ids) == 0 || len(req.Ids) > 500 {
			response.FailMsg(c, errs.CodeValidationFailed, "ids 需 1-500 条")
			return
		}
		uid, uname, ip, ua := meta(c)
		m, err := d.Triage.BatchUpdateTicketStatus(req.Ids, req.Status, uid, uname, ip, ua)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})
	t.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		uid, uname, ip, ua := meta(c)
		if err := d.Triage.DeleteTicket(id, uid, uname, ip, ua); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}
