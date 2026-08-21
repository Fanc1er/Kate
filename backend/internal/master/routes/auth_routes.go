package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

func registerAuth(rg *gin.RouterGroup, d *Deps) {
	auth := rg.Group("/auth")

	// 登录。
	auth.POST("/login", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res, err := d.Auth.Login(req.Username, req.Password, c.ClientIP())
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, res)
	})

	// 刷新 token。
	auth.POST("/refresh", func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if !bindJSON(c, &req) {
			return
		}
		pair, err := d.Auth.Refresh(req.RefreshToken)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, pair)
	})

	// 登出（refresh token 入黑名单）。
	auth.POST("/logout", func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.ShouldBindJSON(&req)
		d.Auth.Logout(req.RefreshToken)
		response.OK(c, nil)
	})

	// 忘记密码（发验证码）。
	auth.POST("/forgot-password", func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if err := d.Auth.ForgotPassword(req.Email); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 重置密码。
	auth.POST("/reset-password", func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"`
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if err := d.Auth.ResetPassword(req.Email, req.Code, req.Password); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 登录态改密。
	authed := auth.Group("", d.Security.AuthRequired())
	authed.POST("/change-password", func(c *gin.Context) {
		var req struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}
		if !bindJSON(c, &req) {
			return
		}
		if err := d.Auth.ChangePassword(c.GetInt64("user_id"), req.OldPassword, req.NewPassword); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}
