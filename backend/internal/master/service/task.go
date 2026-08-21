package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/repository"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// TaskService 任务调度服务（用户侧）。
type TaskService struct {
	DB      *gorm.DB
	Audit   *AuditWriter
	Assessor *ResultAssessor
}

// NewTaskService 构造 TaskService。
func NewTaskService(db *gorm.DB, audit *AuditWriter, assessor *ResultAssessor) *TaskService {
	return &TaskService{DB: db, Audit: audit, Assessor: assessor}
}

// guard 返回组织隔离守卫（org_id 强制过滤，缺省 org_id 拒绝查询）。
func (s *TaskService) guard(orgID int64) *repository.Guard {
	g, err := repository.NewGuard(s.DB, orgID)
	if err != nil {
		panic(err)
	}
	return g
}

// TaskCreateReq 发起任务请求。
type TaskCreateReq struct {
	AssetIDs []int64 `json:"asset_ids"`
	PolicyID int64   `json:"policy_id"`
}

// Create 为资产集合批量创建任务。
// 去重：同 org+asset+policy 存在 pending/processing 时返回 3001 TASK_STATE_CONFLICT。
func (s *TaskService) Create(orgID int64, req TaskCreateReq, userID int64, username, ip, ua string) ([]models.ScanTask, error) {
	if len(req.AssetIDs) == 0 {
		return nil, errs.New(errs.CodeValidationFailed, "请选择资产")
	}
	if req.PolicyID <= 0 {
		return nil, errs.New(errs.CodeValidationFailed, "请选择策略模板")
	}
	// 校验策略属于本组织（含平台级 org_id=0 模板）。
	var policy models.ScanPolicy
	if err := s.DB.Where("id = ? AND org_id IN (0, ?)", req.PolicyID, orgID).First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "策略模板不存在")
		}
		return nil, err
	}
	var created []models.ScanTask
	for _, assetID := range req.AssetIDs {
		var count int64
		s.DB.Model(&models.ScanTask{}).
			Where("org_id = ? AND asset_id = ? AND policy_id = ? AND status IN (?, ?)",
				orgID, assetID, req.PolicyID, models.StatusPending, models.StatusProcessing).
			Count(&count)
		if count > 0 {
			return nil, errs.New(errs.CodeTaskStateConflict, fmt.Sprintf("资产 %d 已有进行中的任务", assetID))
		}
		task := &models.ScanTask{
			OrgID: orgID, PolicyID: req.PolicyID, AssetID: assetID,
			TaskScope: "root", Status: models.StatusPending,
		}
		if err := s.DB.Create(task).Error; err != nil {
			return nil, err
		}
		created = append(created, *task)
		if s.Audit != nil {
			s.Audit.Write(orgID, userID, username, "task.start", "task", fmt.Sprint(task.ID), "", fmt.Sprintf("policy:%d asset:%d", req.PolicyID, assetID), ip, ua)
		}
	}
	return created, nil
}

// BatchScan 批量加入扫描：为资产集合创建任务，策略取组织首个可用策略（优先组织内，其次平台级模板）。
func (s *TaskService) BatchScan(orgID int64, assetIDs []int64, userID int64, username, ip, ua string) (int, error) {
	if len(assetIDs) == 0 {
		return 0, errs.New(errs.CodeValidationFailed, "请选择资产")
	}
	var policy models.ScanPolicy
	if err := s.DB.Where("org_id = ?", orgID).Order("id ASC").First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err2 := s.DB.Where("org_id = 0").Order("id ASC").First(&policy).Error; err2 != nil {
				return 0, errs.New(errs.CodeNotFound, "无可用的扫描策略，请先在策略页创建")
			}
		} else {
			return 0, err
		}
	}
	created, err := s.Create(orgID, TaskCreateReq{AssetIDs: assetIDs, PolicyID: policy.ID}, userID, username, ip, ua)
	if err != nil {
		return 0, err
	}
	return len(created), nil
}

// List 任务列表。
func (s *TaskService) List(orgID int64, status string, page, pageSize int) ([]models.ScanTask, int64, error) {
	q := s.guard(orgID).Scoped(&models.ScanTask{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.ScanTask
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Get 任务详情。
func (s *TaskService) Get(orgID, id int64) (*models.ScanTask, error) {
	var t models.ScanTask
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	return &t, nil
}

// Stop 停止任务（置 cancelled + stopped_by_user）。
func (s *TaskService) Stop(orgID, id int64, userID int64, username, ip, ua string) (*models.ScanTask, error) {
	t, err := s.Get(orgID, id)
	if err != nil {
		return nil, err
	}
	if t.Status == models.StatusPending || t.Status == models.StatusProcessing {
		now := time.Now()
		if err := s.DB.Model(t).Updates(map[string]any{
			"status": models.StatusCancelled, "stopped_by_user": true, "finished_at": now,
		}).Error; err != nil {
			return nil, err
		}
		if s.Audit != nil {
			s.Audit.Write(orgID, userID, username, "task.stop", "task", fmt.Sprint(id), t.Status, "cancelled", ip, ua)
		}
	}
	return s.Get(orgID, id)
}

// BatchStop 批量停止。
func (s *TaskService) BatchStop(orgID int64, ids []int64, userID int64, username, ip, ua string) BatchResult {
	res := BatchResult{Success: 0}
	for _, id := range ids {
		if _, err := s.Stop(orgID, id, userID, username, ip, ua); err != nil {
			res.Failed = append(res.Failed, FailedItem{ID: id, Reason: errs.FromError(err).Message})
		} else {
			res.Success++
		}
	}
	return res
}

// Rerun 失败重跑（复用原参数，重置状态为 pending）。
func (s *TaskService) Rerun(orgID, id int64, userID int64, username, ip, ua string) (*models.ScanTask, error) {
	t, err := s.Get(orgID, id)
	if err != nil {
		return nil, err
	}
	if t.Status != models.StatusFailed && t.Status != models.StatusCancelled {
		return nil, errs.New(errs.CodeTaskStateConflict, "仅失败或已取消的任务可重跑")
	}
	if err := s.DB.Model(t).Updates(map[string]any{
		"status": models.StatusPending, "retry_count": 0, "message": "", "started_at": nil, "finished_at": nil, "stopped_by_user": false,
	}).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "task.rerun", "task", fmt.Sprint(id), "", "rerun", ip, ua)
	}
	return s.Get(orgID, id)
}

// BatchRerun 批量重跑。
func (s *TaskService) BatchRerun(orgID int64, ids []int64, userID int64, username, ip, ua string) BatchResult {
	res := BatchResult{Success: 0}
	for _, id := range ids {
		if _, err := s.Rerun(orgID, id, userID, username, ip, ua); err != nil {
			res.Failed = append(res.Failed, FailedItem{ID: id, Reason: errs.FromError(err).Message})
		} else {
			res.Success++
		}
	}
	return res
}

// Delete 删除历史任务（仅 org_admin）。
func (s *TaskService) Delete(orgID, id int64, userID int64, username, ip, ua string) error {
	t, err := s.Get(orgID, id)
	if err != nil {
		return err
	}
	if t.Status == models.StatusProcessing {
		return errs.New(errs.CodeTaskStateConflict, "执行中的任务不能删除")
	}
	if err := s.DB.Delete(&models.ScanTask{}, "id = ? AND org_id = ?", id, orgID).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "task.delete", "task", fmt.Sprint(id), "", "deleted", ip, ua)
	}
	return nil
}

// Queue 队列监控。
func (s *TaskService) Queue(orgID int64) (map[string]any, error) {
	type cnt struct {
		Status string
		Cnt    int64
	}
	var rows []cnt
	if err := s.DB.Model(&models.ScanTask{}).
		Select("status, COUNT(*) AS cnt").
		Where("org_id = ?", orgID).Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := map[string]int64{"pending": 0, "processing": 0, "completed": 0, "failed": 0, "cancelled": 0}
	for _, r := range rows {
		m[r.Status] = r.Cnt
	}
	// Worker 分配情况。
	var workers []models.WorkerNode
	s.DB.Where("org_id = ? AND status = ?", orgID, models.StatusOnline).Find(&workers)
	return map[string]any{
		"queued":     m["pending"],
		"processing": m["processing"],
		"completed":  m["completed"],
		"failed":     m["failed"],
		"cancelled":  m["cancelled"],
		"workers":    workers,
	}, nil
}

// Progress 断点续扫状态（任务进度 + 已爬 URL 数）。
func (s *TaskService) Progress(orgID, id int64) (map[string]any, error) {
	t, err := s.Get(orgID, id)
	if err != nil {
		return nil, err
	}
	var findings int64
	s.DB.Model(&models.Finding{}).Where("org_id = ? AND task_id = ?", orgID, id).Count(&findings)
	var crawled int64
	s.DB.Model(&models.SensitiveInfoHit{}).Where("org_id = ? AND task_id = ?", orgID, id).Count(&crawled)
	out := map[string]any{
		"task_id":  id,
		"status":   t.Status,
		"progress": t.Progress,
		"crawled_urls": crawled,
		"findings": findings,
	}
	// 递归扫描进度：已发现资产数 / discovery_stopped 标记。
	if t.ProgressMeta != "" {
		var meta map[string]any
		if err := json.Unmarshal([]byte(t.ProgressMeta), &meta); err == nil {
			out["discovered"] = meta["discovered"]
			out["discovery_stopped"] = meta["discovery_stopped"]
		}
	}
	return out, nil
}

// StopCheck 供 Worker 轮询判断任务是否被用户停止。
func (s *TaskService) StopCheck(orgID, id int64) bool {
	var t models.ScanTask
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&t).Error; err != nil {
		return false
	}
	return t.Status == models.StatusCancelled
}

// Assess 结果评估器（R2.13 简化实现）。
type ResultAssessor struct {
	DB *gorm.DB
}

// NewResultAssessor 构造 ResultAssessor。
func NewResultAssessor(db *gorm.DB) *ResultAssessor { return &ResultAssessor{DB: db} }

// Assessment 评估明细。
type Assessment struct {
	SeverityBase    int     `json:"severity_base"`
	Confidence      float64 `json:"confidence"`
	EngineHitBonus  int     `json:"engine_hit_bonus"`
	ImportanceBonus int     `json:"importance_bonus"`
	TypeBonus       int     `json:"type_bonus"`
	Total           int     `json:"total"`
}

// Assess 计算 risk_score / risk_level / suggestion。
func (a *ResultAssessor) Assess(orgID, assetID int64, engineName, severity, vulnType string, confidence float64) (int, string, string, *Assessment, error) {
	base := map[string]int{"critical": 80, "high": 60, "medium": 40, "low": 20, "info": 0}[severity]
	detail := &Assessment{SeverityBase: base, Confidence: confidence}
	score := int(float64(base) * confidence)

	// 多引擎重合加成：同一资产已命中的引擎数。
	var hitCount int64
	a.DB.Model(&models.Finding{}).Where("org_id = ? AND asset_id = ?", orgID, assetID).Distinct("engine_name").Count(&hitCount)
	bonus := int(hitCount) * 10
	if bonus > 30 {
		bonus = 30
	}
	detail.EngineHitBonus = bonus
	score += bonus

	// 重点资产加成。
	if assetID > 0 {
		var asset models.Asset
		if err := a.DB.Where("id = ? AND org_id = ?", assetID, orgID).First(&asset).Error; err == nil && asset.Importance == "high" {
			detail.ImportanceBonus = 10
			score += 10
		}
	}
	// 高危类型加成。
	if vulnType == "content_integrity" || vulnType == "hidden_link" || vulnType == "tamper" {
		detail.TypeBonus = 5
		score += 5
	}
	if score > 100 {
		score = 100
	}
	detail.Total = score
	level := "info"
	switch {
	case score >= 85:
		level = "critical"
	case score >= 60:
		level = "high"
	case score >= 40:
		level = "medium"
	case score >= 20:
		level = "low"
	}
	suggestion := suggestionFor(engineName)
	return score, level, suggestion, detail, nil
}

func suggestionFor(engine string) string {
	switch engine {
	case "vuln_scan":
		return "按 CVE 修复并复测"
	case "hidden_link":
		return "隔离+溯源+加固"
	case "webshell":
		return "立即下线+清除+溯源"
	case "phishing":
		return "下线仿冒+备案"
	case "availability":
		return "检查服务/DNS/CDN 状态"
	case "content_security":
		return "核查内容来源并整改"
	default:
		return "核查并确认处置"
	}
}

var _ = json.Marshal
var _ = utils.MD5Hex
