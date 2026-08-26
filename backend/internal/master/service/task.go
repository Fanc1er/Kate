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

// guard 返回单租户查询守卫（无组织隔离）。
func (s *TaskService) guard() *repository.Guard {
	return repository.NewGuard(s.DB)
}

// TaskCreateReq 发起任务请求。
type TaskCreateReq struct {
	AssetIDs []int64 `json:"asset_ids"`
	PolicyID int64   `json:"policy_id"`
}

// Create 为资产集合批量创建任务。
// 去重：同 asset+policy 存在 pending/processing 时返回 3001 TASK_STATE_CONFLICT。
func (s *TaskService) Create(req TaskCreateReq, userID int64, username, ip, ua string) ([]models.ScanTask, error) {
	if len(req.AssetIDs) == 0 {
		return nil, errs.New(errs.CodeValidationFailed, "请选择资产")
	}
	if req.PolicyID <= 0 {
		return nil, errs.New(errs.CodeValidationFailed, "请选择策略模板")
	}
	// 校验策略存在。
	var policy models.ScanPolicy
	if err := s.DB.Where("id = ?", req.PolicyID).First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "策略模板不存在")
		}
		return nil, err
	}
	var created []models.ScanTask
	for _, assetID := range req.AssetIDs {
		var count int64
		s.DB.Model(&models.ScanTask{}).
			Where("asset_id = ? AND policy_id = ? AND status IN (?, ?)",
				assetID, req.PolicyID, models.StatusPending, models.StatusProcessing).
			Count(&count)
		if count > 0 {
			return nil, errs.New(errs.CodeTaskStateConflict, fmt.Sprintf("资产 %d 已有进行中的任务", assetID))
		}
		task := &models.ScanTask{
			PolicyID: req.PolicyID, AssetID: assetID,
			TaskScope: "root", Status: models.StatusPending,
		}
		if err := s.DB.Create(task).Error; err != nil {
			return nil, err
		}
		created = append(created, *task)
		if s.Audit != nil {
			s.Audit.Write(userID, username, "task.start", "task", fmt.Sprint(task.ID), "", fmt.Sprintf("policy:%d asset:%d", req.PolicyID, assetID), ip, ua)
		}
	}
	return created, nil
}

// BatchScan 批量加入扫描：为资产集合创建任务，策略取首个可用策略。
func (s *TaskService) BatchScan(assetIDs []int64, userID int64, username, ip, ua string) (int, error) {
	if len(assetIDs) == 0 {
		return 0, errs.New(errs.CodeValidationFailed, "请选择资产")
	}
	var policy models.ScanPolicy
	if err := s.DB.Order("id ASC").First(&policy).Error; err != nil {
		return 0, errs.New(errs.CodeNotFound, "无可用的扫描策略，请先在策略页创建")
	}
	created, err := s.Create(TaskCreateReq{AssetIDs: assetIDs, PolicyID: policy.ID}, userID, username, ip, ua)
	if err != nil {
		return 0, err
	}
	return len(created), nil
}

// CreateAvailabilityProbe 为资产集合创建轻量可用性探测任务（task_scope=availability_probe）。
// 去重：同资产已存在 availability_probe pending/processing 任务时跳过（幂等，不报错）。
// 策略取首个可用策略（仅用于下发的引擎开关与超时配置），无策略时 policy_id=0（Worker 侧兜底默认）。
func (s *TaskService) CreateAvailabilityProbe(assetIDs []int64) ([]models.ScanTask, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}
	var policy models.ScanPolicy
	if err := s.DB.Order("id ASC").First(&policy).Error; err != nil {
		policy = models.ScanPolicy{}
	}
	var created []models.ScanTask
	for _, assetID := range assetIDs {
		var count int64
		s.DB.Model(&models.ScanTask{}).
			Where("asset_id = ? AND task_scope = ? AND status IN (?, ?)",
				assetID, "availability_probe", models.StatusPending, models.StatusProcessing).
			Count(&count)
		if count > 0 {
			continue
		}
		task := &models.ScanTask{
			PolicyID:  policy.ID,
			AssetID:   assetID,
			TaskScope: "availability_probe",
			Status:    models.StatusPending,
		}
		if err := s.DB.Create(task).Error; err != nil {
			return nil, err
		}
		created = append(created, *task)
	}
	return created, nil
}

// List 任务列表。
func (s *TaskService) List(status string, page, pageSize int) ([]models.ScanTask, int64, error) {
	q := s.guard().Scoped(&models.ScanTask{})
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
func (s *TaskService) Get(id int64) (*models.ScanTask, error) {
	var t models.ScanTask
	if err := s.DB.Where("id = ?", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	return &t, nil
}

// Stop 停止任务（置 cancelled + stopped_by_user）。
func (s *TaskService) Stop(id int64, userID int64, username, ip, ua string) (*models.ScanTask, error) {
	t, err := s.Get(id)
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
			s.Audit.Write(userID, username, "task.stop", "task", fmt.Sprint(id), t.Status, "cancelled", ip, ua)
		}
	}
	return s.Get(id)
}

// BatchStop 批量停止。
func (s *TaskService) BatchStop(ids []int64, userID int64, username, ip, ua string) BatchResult {
	res := BatchResult{Success: 0}
	for _, id := range ids {
		if _, err := s.Stop(id, userID, username, ip, ua); err != nil {
			res.Failed = append(res.Failed, FailedItem{ID: id, Reason: errs.FromError(err).Message})
		} else {
			res.Success++
		}
	}
	return res
}

// Rerun 失败重跑（复用原参数，重置状态为 pending）。
func (s *TaskService) Rerun(id int64, userID int64, username, ip, ua string) (*models.ScanTask, error) {
	t, err := s.Get(id)
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
		s.Audit.Write(userID, username, "task.rerun", "task", fmt.Sprint(id), "", "rerun", ip, ua)
	}
	return s.Get(id)
}

// BatchRerun 批量重跑。
func (s *TaskService) BatchRerun(ids []int64, userID int64, username, ip, ua string) BatchResult {
	res := BatchResult{Success: 0}
	for _, id := range ids {
		if _, err := s.Rerun(id, userID, username, ip, ua); err != nil {
			res.Failed = append(res.Failed, FailedItem{ID: id, Reason: errs.FromError(err).Message})
		} else {
			res.Success++
		}
	}
	return res
}

// Delete 删除历史任务（仅管理员）。
func (s *TaskService) Delete(id int64, userID int64, username, ip, ua string) error {
	t, err := s.Get(id)
	if err != nil {
		return err
	}
	if t.Status == models.StatusProcessing {
		return errs.New(errs.CodeTaskStateConflict, "执行中的任务不能删除")
	}
	if err := s.DB.Delete(&models.ScanTask{}, "id = ?", id).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(userID, username, "task.delete", "task", fmt.Sprint(id), "", "deleted", ip, ua)
	}
	return nil
}

// Queue 队列监控。
func (s *TaskService) Queue() (map[string]any, error) {
	type cnt struct {
		Status string
		Cnt    int64
	}
	var rows []cnt
	if err := s.DB.Model(&models.ScanTask{}).
		Select("status, COUNT(*) AS cnt").
		Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := map[string]int64{"pending": 0, "processing": 0, "completed": 0, "failed": 0, "cancelled": 0}
	for _, r := range rows {
		m[r.Status] = r.Cnt
	}
	// Worker 分配情况。
	var workers []models.WorkerNode
	s.DB.Where("status = ?", models.StatusOnline).Find(&workers)
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
func (s *TaskService) Progress(id int64) (map[string]any, error) {
	t, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	var findings int64
	s.DB.Model(&models.Finding{}).Where("task_id = ?", id).Count(&findings)
	var crawled int64
	s.DB.Model(&models.SensitiveInfoHit{}).Where("task_id = ?", id).Count(&crawled)
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
func (s *TaskService) StopCheck(id int64) bool {
	var t models.ScanTask
	if err := s.DB.Where("id = ?", id).First(&t).Error; err != nil {
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
func (a *ResultAssessor) Assess(assetID int64, engineName, severity, vulnType string, confidence float64) (int, string, string, *Assessment, error) {
	base := map[string]int{"critical": 80, "high": 60, "medium": 40, "low": 20, "info": 0}[severity]
	detail := &Assessment{SeverityBase: base, Confidence: confidence}
	score := int(float64(base) * confidence)

	// 多引擎重合加成：同一资产已命中的引擎数。
	var hitCount int64
	a.DB.Model(&models.Finding{}).Where("asset_id = ?", assetID).Distinct("engine_name").Count(&hitCount)
	bonus := int(hitCount) * 10
	if bonus > 30 {
		bonus = 30
	}
	detail.EngineHitBonus = bonus
	score += bonus

	// 重点资产加成。
	if assetID > 0 {
		var asset models.Asset
		if err := a.DB.Where("id = ?", assetID).First(&asset).Error; err == nil && asset.Importance == "high" {
			detail.ImportanceBonus = 10
			score += 10
		}
	}
	// 高危类型加成。
	switch vulnType {
	case "content_integrity", "hidden_element", "external_iframe", "javascript_protocol",
		"typosquatting", "dns_internal_ip", "dns_resolver_inconsistent",
		"port_exposed", "cert_expired", "cert_hostname_mismatch", "cert_not_yet_valid",
		"webshell_pattern", "webshell_php", "webshell_obfuscated",
		"cve_match", "intel_threat_score":
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
	case "port_service":
		return "收敛暴露面+补丁加固"
	case "dns_security":
		return "核查 DNS 配置并处理证书"
	case "reputation":
		return "确认域名归属并处置"
	case "intelligence":
		return "按情报修复漏洞并复核"
	case "multi_ua":
		return "排查多线路/边缘节点"
	default:
		return "核查并确认处置"
	}
}

var _ = json.Marshal
var _ = utils.MD5Hex
