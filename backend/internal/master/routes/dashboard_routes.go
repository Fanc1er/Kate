package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

func registerDashboard(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/dashboard")

	g.GET("/stats", func(c *gin.Context) {
		m, err := d.Dashboard.Stats(orgID(c))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	g.GET("/trends", func(c *gin.Context) {
		days, _ := strconv.Atoi(c.Query("days"))
		m, err := d.Dashboard.Trends(orgID(c), days)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	g.GET("/top-risks", func(c *gin.Context) {
		list, err := d.Dashboard.TopRisks(orgID(c), 10)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	g.GET("/engine-coverage", func(c *gin.Context) {
		list, err := d.Dashboard.EngineCoverage(orgID(c))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})
}
