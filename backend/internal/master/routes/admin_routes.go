package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// registerAdmin 平台管理（仅管理员，单租户）。
func registerAdmin(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/admin", d.Security.RequireAdmin())

	// 平台统计。
	g.GET("/stats", func(c *gin.Context) {
		var users, assets, tasks, alerts int64
		d.DB.Model(&models.User{}).Count(&users)
		d.DB.Model(&models.Asset{}).Count(&assets)
		d.DB.Model(&models.ScanTask{}).Count(&tasks)
		d.DB.Model(&models.Alert{}).Count(&alerts)
		response.OK(c, map[string]any{
			"users": users, "assets": assets, "tasks": tasks, "alerts": alerts,
		})
	})
}
