package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func registerMembers(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/members")

	// 成员列表。
	g.GET("", d.Security.RequireRoles(models.RoleOrgAdmin), func(c *gin.Context) {
		list, total, err := d.Member.List(orgID(c), utils.ParsePage(c.Query("page")), utils.ParsePageSize(c.Query("page_size")))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})

	// 邀请成员。
	g.POST("", d.Security.RequireRoles(models.RoleOrgAdmin), func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if !bindJSON(c, &req) {
			return
		}
		uo, err := d.Member.Invite(orgID(c), req.Email, req.Role, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, uo)
	})

	// 修改角色。
	g.PUT("/:user_id/role", d.Security.RequireRoles(models.RoleOrgAdmin), func(c *gin.Context) {
		uid, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "user_id 非法")
			return
		}
		var req struct {
			Role string `json:"role"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if err := d.Member.UpdateRole(orgID(c), uid, req.Role, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 启停成员。
	g.PUT("/:user_id/status", d.Security.RequireRoles(models.RoleOrgAdmin), func(c *gin.Context) {
		uid, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "user_id 非法")
			return
		}
		var req struct {
			Status string `json:"status"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if err := d.Member.ToggleStatus(orgID(c), uid, req.Status, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 移除成员。
	g.DELETE("/:user_id", d.Security.RequireRoles(models.RoleOrgAdmin), func(c *gin.Context) {
		uid, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "user_id 非法")
			return
		}
		if err := d.Member.Remove(orgID(c), uid, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}
