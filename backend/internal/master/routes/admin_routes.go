package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// registerAdmin 平台管理（仅 super_admin，org_id=0 全局通道）。
func registerAdmin(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/admin", d.Security.RequireSuperAdmin())

	// 组织列表。
	g.GET("/organizations", func(c *gin.Context) {
		page := utils.ParsePage(c.Query("page"))
		pageSize := utils.ParsePageSize(c.Query("page_size"))
		var total int64
		d.DB.Model(&models.Organization{}).Count(&total)
		var list []models.Organization
		if err := d.DB.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})

	// 创建组织。
	g.POST("/organizations", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
			Plan string `json:"plan"`
		}
		if !bindJSON(c, &req) {
			return
		}
		org := &models.Organization{Name: req.Name, Plan: req.Plan, Status: models.StatusActive}
		if err := d.DB.Create(org).Error; err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, org)
	})

	// 组织详情。
	g.GET("/organizations/:id", func(c *gin.Context) {
		id, ok := parseOrgID(c)
		if !ok {
			return
		}
		var org models.Organization
		if err := d.DB.First(&org, id).Error; err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		var members, assets int64
		d.DB.Model(&models.UserOrg{}).Where("org_id = ?", id).Count(&members)
		d.DB.Model(&models.Asset{}).Where("org_id = ?", id).Count(&assets)
		response.OK(c, map[string]any{"organization": org, "members": members, "assets": assets})
	})

	// 平台统计。
	g.GET("/stats", func(c *gin.Context) {
		var orgs, users, assets, tasks, alerts int64
		d.DB.Model(&models.Organization{}).Count(&orgs)
		d.DB.Model(&models.User{}).Count(&users)
		d.DB.Model(&models.Asset{}).Count(&assets)
		d.DB.Model(&models.ScanTask{}).Count(&tasks)
		d.DB.Model(&models.Alert{}).Count(&alerts)
		response.OK(c, map[string]any{
			"organizations": orgs, "users": users, "assets": assets, "tasks": tasks, "alerts": alerts,
		})
	})
}

func parseOrgID(c *gin.Context) (int64, bool) {
	id, err := parseIDParam(c, "id")
	if err != nil {
		response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
		return 0, false
	}
	return id, true
}
