package routes

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// registerNotify 通知渠道管理：列表/创建/更新/删除/测试。
func registerNotify(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/notify/channels")

	g.GET("", func(c *gin.Context) {
		list, err := d.Notify.List()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	g.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Type   string `json:"type"`
			Config string `json:"config"`
		}
		if !bindJSON(c, &req) {
			return
		}
		ch, err := d.Notify.Create(req.Type, req.Config)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeNotifyAudit(c, d, "notify.channel.create", ch.ID)
		response.OK(c, ch)
	})

	g.PUT("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "无效的渠道 ID"))
			return
		}
		var req struct {
			Config  *string `json:"config"`
			Enabled *string `json:"enabled"`
		}
		if !bindJSON(c, &req) {
			return
		}
		ch, err := d.Notify.Update(id, req.Config, req.Enabled)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeNotifyAudit(c, d, "notify.channel.update", id)
		response.OK(c, ch)
	})

	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "无效的渠道 ID"))
			return
		}
		if err := d.Notify.Delete(id); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeNotifyAudit(c, d, "notify.channel.delete", id)
		response.OK(c, nil)
	})

	g.POST("/:id/test", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "无效的渠道 ID"))
			return
		}
		if err := d.Notify.Test(id); err != nil {
			response.Fail(c, errs.New(errs.CodeValidationFailed, "测试推送失败: "+err.Error()))
			return
		}
		writeNotifyAudit(c, d, "notify.channel.test", id)
		response.OK(c, nil)
	})
}

// writeNotifyAudit 记录渠道管理审计（Audit 未装配时静默跳过）。
func writeNotifyAudit(c *gin.Context, d *Deps, action string, channelID int64) {
	if d.Notify.Audit == nil {
		return
	}
	d.Notify.Audit.Write(userID(c), usernameOf(c), action, "notify_channel", fmt.Sprint(channelID), "", "", c.ClientIP(), c.GetHeader("User-Agent"))
}
