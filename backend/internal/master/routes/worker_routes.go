package routes

import (
	"encoding/base64"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// workerAuth 校验 Worker 长期凭证（client_id + client_secret），注入 worker_id/org_id。
func workerAuth(d *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := c.GetHeader("X-Client-Id")
		secret := c.GetHeader("X-Client-Secret")
		if clientID == "" || secret == "" {
			response.FailCode(c, errs.CodeWorkerUnauthorized)
			c.Abort()
			return
		}
		var node models.WorkerNode
		if err := d.DB.Where("client_id = ?", clientID).First(&node).Error; err != nil {
			response.FailCode(c, errs.CodeWorkerUnauthorized)
			c.Abort()
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(node.ClientSecretHash), []byte(secret)) != nil {
			response.FailCode(c, errs.CodeWorkerUnauthorized)
			c.Abort()
			return
		}
		c.Set("worker_id", node.ID)
		c.Set("org_id", node.OrgID)
		c.Next()
	}
}

func registerWorker(rg *gin.RouterGroup, d *Deps) {
	w := rg.Group("/worker")

	// 注册握手：Bootstrap Token 一次性换长期凭证。
	w.POST("/register", func(c *gin.Context) {		var req struct {
			Token   string `json:"token"`
			Name    string `json:"name"`
			Version string `json:"version"`
			IP      string `json:"ip"`
		}
		if !bindJSON(c, &req) {
			return
		}
		res, err := d.Worker.Register(req.Token, req.Name, req.Version, req.IP)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, res)
	})

	// 心跳。
	w.POST("/heartbeat", workerAuth(d), func(c *gin.Context) {
		var req struct {
			Load    float64 `json:"load"`
			Version string  `json:"version"`
		}
		_ = c.ShouldBindJSON(&req)
		if err := d.Worker.Heartbeat(c.GetInt64("worker_id"), c.GetInt64("org_id"), req.Load, req.Version); err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})

	// 拉取任务。
	w.POST("/pull", workerAuth(d), func(c *gin.Context) {
		m, err := d.Worker.PullTask(c.GetInt64("org_id"), strconv.FormatInt(c.GetInt64("worker_id"), 10))
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	// 停止检查。
	w.POST("/stop-check", workerAuth(d), func(c *gin.Context) {
		var req struct {
			TaskID int64 `json:"task_id"`
		}
		if !bindJSON(c, &req) {
			return
		}
		stopped := d.Task.StopCheck(c.GetInt64("org_id"), req.TaskID)
		response.OK(c, map[string]any{"stopped": stopped})
	})

	// 结果回传。
	w.POST("/result", workerAuth(d), func(c *gin.Context) {
		var result service.WorkerResult
		if !bindJSON(c, &result) {
			return
		}
		m, err := d.Worker.ReportResult(c.GetInt64("org_id"), &result)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, m)
	})

	// 证据分片上传：<1MB 走内联，≥1MB 分片（≤8MB/片，upload_id 断点续传，收齐 SHA-256 校验）。
	w.POST("/evidence", workerAuth(d), func(c *gin.Context) {
		var req struct {
			UploadID    string `json:"upload_id"`
			Kind        string `json:"kind"`
			TotalChunks int    `json:"total_chunks"`
			ChunkIndex  int    `json:"chunk_index"`
			Data        string `json:"data"`
			SHA256      string `json:"sha256"`
		}
		if !bindJSON(c, &req) {
			return
		}
		data, err := decodeBase64(req.Data)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "data 必须为 base64")
			return
		}
		id, complete, err := d.Evidence.ChunkUpload(c.GetInt64("org_id"), req.UploadID, req.Kind,
			req.TotalChunks, req.ChunkIndex, data, req.SHA256)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, map[string]any{"evidence_id": id, "complete": complete})
	})
}

// registerWorkerNodes 组织侧 Worker 节点管理（org_admin）。
func registerWorkerNodes(rg *gin.RouterGroup, d *Deps) {
	g := rg.Group("/worker/nodes", d.Security.AdminOnly())
	g.GET("", func(c *gin.Context) {
		var list []models.WorkerNode
		if err := d.DB.Where("org_id = ?", orgID(c)).Order("id DESC").Find(&list).Error; err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, list)
	})
	g.POST("", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
			IP   string `json:"ip"`
		}
		if !bindJSON(c, &req) {
			return
		}
		node, token, err := d.Worker.CreateWorkerNode(orgID(c), req.Name, req.IP)
		if err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, map[string]any{"node": node, "bootstrap_token": token})
	})
	g.POST("/:id/revoke", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			response.FailMsg(c, errs.CodeValidationFailed, "id 非法")
			return
		}
		if err := d.DB.Model(&models.WorkerNode{}).Where("id = ? AND org_id = ?", id, orgID(c)).
			Updates(map[string]any{"status": "offline_removed", "boot_token_hash": ""}).Error; err != nil {
			response.Fail(c, errs.FromError(err))
			return
		}
		response.OK(c, nil)
	})
}

// decodeBase64 解码 worker 回传的 base64 内容。
func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
