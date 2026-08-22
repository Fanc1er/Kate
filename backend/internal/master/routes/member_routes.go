package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func registerMembers(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/members")

	// 用户列表。
	g.GET("", d.Security.RequireAdmin(), func(c *gin.Context) {
		list, total, err := d.Member.List(utils.ParsePage(c.Query("page")), utils.ParsePageSize(c.Query("page_size")))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})

	// 邀请用户。
	g.POST("", d.Security.RequireAdmin(), func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if !bindJSON(c, &req) {
			return
		}
		u, err := d.Member.Invite(req.Email, req.Role, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, u)
	})

	// 修改角色。
	g.PUT("/:user_id/role", d.Security.RequireAdmin(), func(c *gin.Context) {
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
		if err := d.Member.UpdateRole(uid, req.Role, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 启停用户。
	g.PUT("/:user_id/status", d.Security.RequireAdmin(), func(c *gin.Context) {
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
		if err := d.Member.ToggleStatus(uid, req.Status, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 删除用户。
	g.DELETE("/:user_id", d.Security.RequireAdmin(), func(c *gin.Context) {
		uid, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "user_id 非法")
			return
		}
		if err := d.Member.Remove(uid, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}
