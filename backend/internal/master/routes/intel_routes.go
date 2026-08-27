package routes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func registerIntel(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/intel")

	// 情报条目分页列表。
	g.GET("", func(c *gin.Context) {
		list, total, err := d.Intel.List(utils.ParsePage(c.Query("page")), utils.ParsePageSize(c.Query("page_size")))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, response.Page{List: list, Total: total})
	})

	// 批量导入情报（按 intel_id 幂等覆盖）。
	g.POST("/import", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Items []service.IntelInput `json:"items"`
		}
		if !bindJSON(c, &req) {
			return
		}
		imported, err := d.Intel.Import(req.Items)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, err.Error())
			return
		}
		userID, username := userID(c), usernameOf(c)
		ip, ua := clientMeta(c)
		if d.Intel.Audit != nil {
			d.Intel.Audit.Write(userID, username, "intel.import", "intel_item", "", "", reqTitle(req.Items), ip, ua)
		}
		response.OK(c, map[string]any{"imported": imported})
	})

	// 删除情报条目。
	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		if err := d.Intel.Delete(id, userID(c), usernameOf(c), c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}

func reqTitle(items []service.IntelInput) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0].IntelID
	}
	return "batch:" + items[0].IntelID + "...(" + strconv.Itoa(len(items)) + ")"
}
