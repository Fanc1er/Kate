package routes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

func registerEvidence(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/evidence")
	parseID := func(c *gin.Context) (int64, bool) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return 0, false
		}
		return id, true
	}

	// 证据元数据 + 子文件链。
	g.GET("/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		ev, err := d.Evidence.Get(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		files, _ := d.Evidence.Files(orgID(c), id)
		response.OK(c, map[string]any{"evidence": ev, "files": files})
	})

	// 证据文件流（读取时强制 Hash 校验）。
	g.GET("/:id/file", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		ev, err := d.Evidence.Get(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		data, _, err := d.Evidence.Read(orgID(c), id)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		c.Header("Content-Type", ev.MimeType)
		c.Header("X-Evidence-SHA256", ev.SHA256)
		c.Data(http.StatusOK, ev.MimeType, data)
	})

	// 证据下载（支持 format=har）。
	g.GET("/:id/download", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		data, filename, err := d.Evidence.Download(orgID(c), id, c.Query("format"))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Data(http.StatusOK, "application/octet-stream", data)
	})

	// 截图上传（Base64 或文件流，MIME png/jpeg/webp，≤10MB）。
	g.POST("/screenshots", d.Security.RequireWrite(), func(c *gin.Context) {
		header, err := c.FormFile("file")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "缺少文件")
			return
		}
		ev, err := d.Evidence.UploadScreenshot(orgID(c), "screenshot", header)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, ev)
	})
}
