package routes

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// registerScenarios 扫描场景管理：列表/创建/更新/删除/切换激活。
func registerScenarios(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/scenarios")

	g.GET("", func(c *gin.Context) {
		list, err := d.Scenario.List()
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})

	g.POST("", d.Security.RequireWrite(), func(c *gin.Context) {
		var req struct {
			Name             string `json:"name"`
			ScenarioType     string `json:"scenario_type"`
			Description      string `json:"description"`
			PolicyID         int64  `json:"policy_id"`
			AssetGroupName   string `json:"asset_group_name"`
			Activated        bool   `json:"activated"`
		}
		if !bindJSON(c, &req) {
			return
		}
		sc, err := d.Scenario.Create(req.Name, req.ScenarioType, req.Description, req.PolicyID, req.AssetGroupName, req.Activated)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeScenarioAudit(c, d, "scenario.create", sc.ID)
		response.OK(c, sc)
	})

	g.PUT("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的场景 ID")
			return
		}
		var req struct {
			Name             *string `json:"name"`
			ScenarioType     *string `json:"scenario_type"`
			Description      *string `json:"description"`
			PolicyID         *int64  `json:"policy_id"`
			AssetGroupName   *string `json:"asset_group_name"`
		}
		if !bindJSON(c, &req) {
			return
		}
		sc, err := d.Scenario.Update(id, req.Name, req.ScenarioType, req.Description, req.PolicyID, req.AssetGroupName)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeScenarioAudit(c, d, "scenario.update", id)
		response.OK(c, sc)
	})

	g.DELETE("/:id", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的场景 ID")
			return
		}
		if err := d.Scenario.Delete(id); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		writeScenarioAudit(c, d, "scenario.delete", id)
		response.OK(c, nil)
	})

	// 切换激活状态（激活时触发任务创建）。
	g.POST("/:id/toggle", d.Security.RequireWrite(), func(c *gin.Context) {
		id, err := parseIDParam(c, "id")
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "无效的场景 ID")
			return
		}
		var req struct {
			Activated bool `json:"activated"`
		}
		if !bindJSON(c, &req) {
			return
		}
		sc, err := d.Scenario.ToggleActivate(id, req.Activated)
		if err != nil {
		if errs.CodeOf(err) == errs.CodeTaskStateConflict {
			log.Printf("scenario %d toggle: task conflict: %v", id, err)
		}
			response.Fail(c, errs.FromError(err))
			return
		}
		action := "scenario.activate"
		if !req.Activated {
			action = "scenario.deactivate"
		}
		writeScenarioAudit(c, d, action, id)
		response.OK(c, sc)
	})
}

// writeScenarioAudit 占位，暂未接入审计（场景管理属低风险操作）。
func writeScenarioAudit(_ *gin.Context, _ *Deps, _ string, _ int64) {}
