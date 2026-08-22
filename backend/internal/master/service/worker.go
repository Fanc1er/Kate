package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// WorkerService Worker 握手、心跳、拉取、回传。
type WorkerService struct {
	DB         *gorm.DB
	Task       *TaskService
	Evidence   *EvidenceService
	Hub        *Hub
	License    *license.Manager
	StormLimit int
}

// NewWorkerService 构造 WorkerService。
func NewWorkerService(db *gorm.DB, task *TaskService, evidence *EvidenceService, hub *Hub, lic *license.Manager, stormLimit int) *WorkerService {
	return &WorkerService{DB: db, Task: task, Evidence: evidence, Hub: hub, License: lic, StormLimit: stormLimit}
}

// ---------- 注册握手 ----------

// CreateWorkerNode 管理员创建节点并生成一次性 Bootstrap Token。
func (s *WorkerService) CreateWorkerNode(name, ip string) (*models.WorkerNode, string, error) {
	if err := s.checkWorkerQuota(); err != nil {
		return nil, "", err
	}
	token := randomSecret(48)
	node := &models.WorkerNode{
		Name: name, IP: ip, Status: models.StatusOffline,
		BootTokenHash: utils.SHA256HexString(token),
	}
	if err := s.DB.Create(node).Error; err != nil {
		return nil, "", err
	}
	return node, token, nil
}

// checkWorkerQuota 校验 Worker 节点数是否超过授权上限。
func (s *WorkerService) checkWorkerQuota() error {
	if s.License == nil {
		return nil
	}
	max := s.License.MaxWorkers()
	if max <= 0 {
		return nil
	}
	var used int64
	s.DB.Model(&models.WorkerNode{}).Where("status <> ?", "offline_removed").Count(&used)
	if int(used) >= max {
		return errs.New(errs.CodeWorkerQuota, "")
	}
	return nil
}

// Register Bootstrap Token 一次性换取长期凭证。
func (s *WorkerService) Register(token, name, version, ip string) (map[string]any, error) {
	tokenHash := utils.SHA256HexString(token)
	var node models.WorkerNode
	if err := s.DB.Where("boot_token_hash = ?", tokenHash).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeWorkerUnauthorized, "Bootstrap Token 无效")
		}
		return nil, err
	}
	if node.ClientID != "" {
		return nil, errs.New(errs.CodeWorkerUnauthorized, "Bootstrap Token 已使用")
	}
	clientID := "w-" + randomSecret(16)
	clientSecret := randomSecret(24)
	secretHash, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.DB.Model(&node).Updates(map[string]any{
		"client_id":          clientID,
		"client_secret_hash": string(secretHash),
		"name":               name,
		"version":            version,
		"ip":                 ip,
		"status":             models.StatusOnline,
		"heartbeat_at":       now,
		"boot_token_hash":    "",
	}).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"client_id":     clientID,
		"client_secret": clientSecret,
	}, nil
}

// Heartbeat 心跳上报。
func (s *WorkerService) Heartbeat(nodeID int64, load float64, version string) error {
	now := time.Now()
	return s.DB.Model(&models.WorkerNode{}).
		Where("id = ?", nodeID).
		Updates(map[string]any{"load": load, "version": version, "heartbeat_at": now, "status": models.StatusOnline}).Error
}

// ---------- 任务拉取 ----------

// PullTask 拉取 pending 任务并原子置 processing（单事务防双拉）。
func (s *WorkerService) PullTask(workerID string) (map[string]any, error) {
	var task models.ScanTask
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("status = ? AND (retry_at IS NULL OR retry_at <= ?)", models.StatusPending, time.Now()).
			Order("id ASC").First(&task).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&task).Updates(map[string]any{
			"status": models.StatusProcessing, "started_at": now, "worker_id": workerID,
		}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[string]any{"task": nil}, nil
		}
		return nil, err
	}
	var policy models.ScanPolicy
	if err := s.DB.Where("id = ?", task.PolicyID).First(&policy).Error; err != nil {
		policy = models.ScanPolicy{Name: "默认", Concurrency: 4, Timeout: 60, ScanDepth: 2, ConcurrencyLimit: 4, SameOrigin: true, CrawlSubpages: true}
	}
	var asset models.Asset
	s.DB.Where("id = ?", task.AssetID).First(&asset)
	// 从 EngineSwitches 解析可用性引擎配置（fail_count / slow_threshold_ms）随任务下发。
	switches := map[string]any{}
	_ = json.Unmarshal([]byte(policy.EngineSwitches), &switches)
	avail, _ := switches["availability"].(map[string]any)
	failCount := 3
	slowThreshold := 3000
	if fc, ok := avail["fail_count"].(float64); ok && fc > 0 {
		failCount = int(fc)
	}
	if st, ok := avail["slow_threshold_ms"].(float64); ok && st > 0 {
		slowThreshold = int(st)
	}
	// 证据开关：默认开启（策略未显式关闭即启用）。
	evidenceEnabled := true
	if ev, ok := switches["evidence"].(map[string]any); ok {
		if en, ok := ev["enabled"].(bool); ok {
			evidenceEnabled = en
		}
	}
	// 引擎开关列表：解析全部引擎 enabled 状态，启用列表供 Worker 按开关执行。
	enabledEngines := []string{}
	for name, cfg := range switches {
		m, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		if en, ok := m["enabled"].(bool); ok && en {
			enabledEngines = append(enabledEngines, name)
		}
	}
	// 高级开关展开为 Worker 细粒度引擎名（policy 开关 → 实际引擎）。
	enabledEngines = expandEngineSwitches(enabledEngines)
	// 缺省 availability 开关（兼容旧策略）。
	hasAvailability := false
	for _, n := range enabledEngines {
		if n == "availability" {
			hasAvailability = true
		}
	}
	if !hasAvailability {
		enabledEngines = append(enabledEngines, "availability")
	}
	// 缺省 multi_ua 开关：未显式配置则默认启用（多端 UA 综合评估为高价值能力）。
	hasMultiUA := false
	for _, n := range enabledEngines {
		if n == "multi_ua" {
			hasMultiUA = true
		}
	}
	if !hasMultiUA {
		if m, ok := switches["multi_ua"].(map[string]any); ok {
			if en, ok := m["enabled"].(bool); ok && !en {
				// 显式关闭。
			} else {
				enabledEngines = append(enabledEngines, "multi_ua")
			}
		} else {
			enabledEngines = append(enabledEngines, "multi_ua")
		}
	}
	// 关键词规则：rule_definitions.kind=keyword 且 loaded 的启用规则随任务下发。
	keywordRules := []map[string]any{}
	{
		var defs []models.RuleDefinition
		s.DB.Where("kind = ? AND loaded = ?", "keyword", true).
			Order("id ASC").Find(&defs)
		for _, d := range defs {
			keywordRules = append(keywordRules, map[string]any{
				"id": d.ID, "name": d.Name, "group": d.Group,
				"pattern": d.FRegex, "s_regex": d.SRegex,
				"sensitive": d.Sensitive, "scope": d.Scope,
			})
		}
	}
	// 域名规则：白名单 + 恶意域名库随任务下发（外链发现评估用）。
	domainRules := []map[string]any{}
	{
		var defs []models.RuleDefinition
		s.DB.Where("kind IN ? AND loaded = ?", []string{"domain_whitelist", "malicious_domain"}, true).
			Order("id ASC").Find(&defs)
		for _, d := range defs {
			domainRules = append(domainRules, map[string]any{
				"kind": d.Kind, "name": d.Name,
				"pattern": d.FRegex, "sensitive": d.Sensitive,
			})
		}
	}
	return map[string]any{
		"task":   task,
		"policy": policy,
		"asset":  asset,
		"keyword_rules": keywordRules,
		"domain_rules":  domainRules,
		"recursion": map[string]any{
			"scan_depth":        policy.ScanDepth,
			"concurrency_limit": policy.ConcurrencyLimit,
			"allow_static":      policy.AllowStatic,
			"same_origin":       policy.SameOrigin,
			"crawl_subpages":    policy.CrawlSubpages,
		},
		"engine_switches": map[string]any{
			"availability": map[string]any{
				"enabled":           true,
				"fail_count":        failCount,
				"slow_threshold_ms": slowThreshold,
			},
			"evidence": map[string]any{"enabled": evidenceEnabled},
		},
		"engines": enabledEngines,
	}, nil
}

// ---------- 结果回传 ----------

// WorkerFinding 回传 finding 结构。
type WorkerFinding struct {
	EngineName  string         `json:"engine_name"`
	Type        string         `json:"type"`
	Severity    string         `json:"severity"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	URL         string         `json:"url"`
	LineNo      int            `json:"line_no"`
	Confidence  float64        `json:"confidence"`
	EvidenceIDs []int64        `json:"evidence_ids"`
	InlineEvidence []InlineEvidence `json:"inline_evidence,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// InlineEvidence <1MB 内联证据。
type InlineEvidence struct {
	Kind    string `json:"kind"` // html/har/screenshot/req/resp
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

// WorkerResult 结果回传协议。
type WorkerResult struct {
	ResultID      string           `json:"result_id"`
	TaskID        int64            `json:"task_id"`
	Status        string           `json:"status"`
	TaskTimeout   bool             `json:"task_timeout"`
	StoppedByUser bool             `json:"stopped_by_user"`
	Message       string           `json:"message"`
	Progress      int              `json:"progress"`
	Findings      []WorkerFinding  `json:"findings"`
	Evidence      []InlineEvidence `json:"evidence,omitempty"`
	Metrics       map[string]any   `json:"metrics"`
	Discovered    []DiscoveredAsset `json:"discovered_assets,omitempty"`
}

// DiscoveredAsset 递归扫描发现的子资产（落 assets 表）。
type DiscoveredAsset struct {
	URL        string `json:"url"`
	SourceType string `json:"source_type"`
}

// ReportResult 处理结果回传：幂等去重 → 落 finding → 结果评估 → 降噪 → 事件/漏洞/告警 → WS 广播。
func (s *WorkerService) ReportResult(result *WorkerResult) (map[string]any, error) {
	if result.ResultID == "" {
		return nil, errs.New(errs.CodeValidationFailed, "缺少 result_id")
	}
	// 幂等去重。
	var existing int64
	s.DB.Model(&models.Finding{}).Where("result_id = ?", result.ResultID).Count(&existing)
	if existing > 0 {
		return map[string]any{"received": true, "duplicated": true}, nil
	}
	// 更新任务状态。
	var task models.ScanTask
	if err := s.DB.Where("id = ?", result.TaskID).First(&task).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, "任务不存在")
	}
	now := time.Now()
	updates := map[string]any{"progress": result.Progress, "finished_at": now}
	switch result.Status {
	case models.StatusCompleted:
		updates["status"] = models.StatusCompleted
	case models.StatusFailed:
		if result.TaskTimeout {
			// 任务级超时中止，不参与失败自动重试。
			updates["status"] = models.StatusFailed
			updates["message"] = "任务执行超时"
		} else if task.RetryCount < 3 {
			// 失败自动重试：指数退避（2^retry×30s），重新置 pending。
			backoff := time.Duration(1<<task.RetryCount) * 30 * time.Second
			updates["status"] = models.StatusPending
			updates["retry_count"] = task.RetryCount + 1
			updates["started_at"] = nil
			updates["message"] = fmt.Sprintf("执行失败，%s 后自动重试（第 %d 次）", backoff.String(), task.RetryCount+1)
			updates["retry_at"] = now.Add(backoff)
		} else {
			updates["status"] = models.StatusFailed
			updates["message"] = "重试 3 次仍失败: " + result.Message
		}
	case models.StatusCancelled:
		updates["status"] = models.StatusCancelled
		updates["stopped_by_user"] = result.StoppedByUser
	}
	s.DB.Model(&task).Updates(updates)

	// 轻量可用性探测任务：写可用性时序点（status_code / latency_ms 从 Metrics 提取）。
	if task.TaskScope == "availability_probe" {
		point := models.AvailabilityPoint{
			AssetID:    task.AssetID,
			Engine:     "availability",
			StatusCode: int(metricsFloat(result.Metrics, "status_code")),
			ResponseMs: int(metricsFloat(result.Metrics, "latency_ms")),
			SampledAt:  now,
		}
		if err := s.DB.Create(&point).Error; err != nil {
			log.Printf("worker: persist availability point: %v", err)
		}
	}

	// 递归扫描发现的子资产落库：source_type 标注 + 配额校验。
	// 达到 max_assets 停止写入新发现并标记 discovery_stopped: quota_exceeded。
	if len(result.Discovered) > 0 {
		s.persistDiscoveredAssets(task, result.Discovered)
	}

	// 处理 findings。
	created := 0
	for _, wf := range result.Findings {
		// 内容完整性指纹：仅维护基线（importance=high 建立/比对），不产生 info finding。
		if wf.EngineName == "content_security" && wf.Type == "content_integrity" {
			s.processContentIntegrity(task, result.ResultID, wf)
			continue
		}
		// 外链清单：维护外链基线，新增/移除/可疑由 processExternalLink 比对处理。
		if wf.EngineName == "content_security" && wf.Type == "external_link" {
			s.processExternalLink(task, result.ResultID, wf)
			continue
		}
		// 顶层证据未内嵌在 finding 时，回退到 result.Evidence（全量证据链）。
		if len(wf.InlineEvidence) == 0 && len(result.Evidence) > 0 {
			wf.InlineEvidence = result.Evidence
		}
		if err := s.processFinding(result.TaskID, task.AssetID, result.ResultID, wf); err == nil {
			created++
		}
	}
	// 顶层证据独立落库（即使无 finding，也保留证据链）。
	for _, ie := range result.Evidence {
		data, err := decodeBase64(ie.Content)
		if err != nil {
			continue
		}
		if utils.SHA256Hex(data) != ie.SHA256 {
			continue
		}
		if _, err := s.Evidence.CreateFromBytes(ie.Kind, data); err != nil {
			continue
		}
	}
	if created > 0 {
		s.Hub.Broadcast(map[string]any{
			"type": "finding.new", "data": map[string]any{"task_id": result.TaskID, "count": created},
		})
	}
	return map[string]any{"received": true, "findings_created": created}, nil
}

// metricsFloat 从回传 metrics 中提取数值（JSON 反序列化为 float64，兼容 int）。
func metricsFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func (s *WorkerService) processFinding(taskID, assetID int64, resultID string, wf WorkerFinding) error {
	// 处理内联证据。
	evidenceIDs := wf.EvidenceIDs
	for _, ie := range wf.InlineEvidence {
		data, err := decodeBase64(ie.Content)
		if err != nil {
			continue
		}
		if utils.SHA256Hex(data) != ie.SHA256 {
			continue
		}
		id, err := s.Evidence.CreateFromBytes(ie.Kind, data)
		if err != nil {
			continue
		}
		evidenceIDs = append(evidenceIDs, id)
	}
	sev := wf.Severity
	if sev == "" {
		sev = "info"
	}
	// 结果评估。
	score, level, suggestion, assessment, err := s.Task.Assessor.Assess(assetID, wf.EngineName, sev, wf.Type, wf.Confidence)
	if err != nil {
		score, level, suggestion = 0, "info", ""
	}
	if wf.Extra == nil {
		wf.Extra = map[string]any{}
	}
	if assessment != nil {
		wf.Extra["assessment"] = assessment
	}
	extraJSON, _ := json.Marshal(wf.Extra)
	evIDs, _ := json.Marshal(evidenceIDs)
	finding := &models.Finding{
		TaskID: taskID, AssetID: assetID, ResultID: resultID,
		EngineName: wf.EngineName, Type: wf.Type, Severity: sev,
		RiskLevel: level, RiskScore: score, Suggestion: suggestion,
		Title: wf.Title, Description: wf.Description, URL: wf.URL, LineNo: wf.LineNo,
		Confidence: wf.Confidence, EvidenceIDs: string(evIDs), Status: "open", Extra: string(extraJSON),
	}
	if err := s.DB.Create(finding).Error; err != nil {
		return err
	}
	// 敏感信息命中明细落库：engine=content_security 且 type=sensitive_info 时解析 extra.sensitive_info_hits。
	if wf.EngineName == "content_security" && wf.Type == "sensitive_info" {
		s.persistSensitiveInfoHits(taskID, wf.Extra)
	}
	// 降噪过滤（简化：无规则命中即放行）。
	if s.isNoisy(wf.URL, wf.EngineName) {
		return nil
	}
	// 多端 UA 正常评估报告（info）不入事件，避免扫描噪音；仅异常（high/medium）生成事件。
	if wf.Type == "multi_ua_evaluation" && sev == "info" {
		return nil
	}
	// 生成事件。
	eventType, title := mapEvent(wf.EngineName, wf.Type, wf.Severity)
	event := &models.Event{
		AssetID: assetID, FindingIDs: fmt.Sprint(finding.ID),
		EngineName: wf.EngineName, EventType: eventType, Title: title,
		Severity: sev, URL: wf.URL, Content: wf.Description,
		EvidenceIDs: string(evIDs), Status: "pending",
	}
	if err := s.DB.Create(event).Error; err != nil {
		return err
	}
	s.Hub.Broadcast(map[string]any{
		"type": "event.new",
		"data": map[string]any{"event_id": event.ID, "title": title, "severity": sev},
	})
	// 漏洞聚合（vuln_scan / dns 类）。
	if wf.EngineName == "vuln_scan" || wf.EngineName == "dns_security" {
		s.aggregateVuln(finding, wf, evidenceIDs)
	}
	// 告警生成：high/critical 或告警类类型。
	if isAlertWorthy(wf.Severity, wf.Type) {
		alert := &models.Alert{
			AssetID: assetID, FindingID: finding.ID,
			AlertType: alertTypeOf(wf.EngineName), Severity: sev, Title: wf.Title,
			Content: wf.Description, Status: "open",
		}
		if err := s.DB.Create(alert).Error; err == nil {
			s.Hub.Broadcast(map[string]any{
				"type": "alert.new",
				"data": map[string]any{"alert_id": alert.ID, "title": wf.Title, "severity": sev},
			})
		}
	}
	return nil
}

// processContentIntegrity 内容完整性基线维护：importance=high 资产建立/比对内容指纹，
// 变更时生成篡改 finding（type=content_integrity）与事件。
func (s *WorkerService) processContentIntegrity(task models.ScanTask, resultID string, wf WorkerFinding) {
	// 仅重点资产纳入基线监测。
	var asset models.Asset
	if err := s.DB.Where("id = ?", task.AssetID).First(&asset).Error; err != nil {
		return
	}
	// 仅种子资产 URL 纳入基线（递归发现的子页面不单独建基线）。
	if asset.Importance != "high" || (wf.URL != "" && wf.URL != asset.URL) {
		return
	}
	fpRaw, ok := wf.Extra["content_fingerprint"].(map[string]any)
	if !ok {
		return
	}
	fp := map[string]string{}
	for _, k := range []string{"title_hash", "text_hash", "body_hash"} {
		if v, ok := fpRaw[k].(string); ok {
			fp[k] = v
		}
	}
	if len(fp) == 0 {
		return
	}
	ver, _ := wf.Extra["fingerprint_version"].(string)
	if ver == "" {
		ver = "v1"
	}
	now := time.Now()
	var bl models.ContentBaseline
	err := s.DB.Where("asset_id = ? AND url = ?", asset.ID, wf.URL).First(&bl).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 首次采集：建立基线。
		s.DB.Create(&models.ContentBaseline{
			AssetID: asset.ID, URL: wf.URL,
			Fingerprint: mustJSON(fp), FingerprintVer: ver,
			FirstSeenAt: now, LastSeenAt: now,
		})
		return
	}
	if err != nil {
		return
	}
	// 比对基线：计算变更维度。
	oldFP := map[string]string{}
	_ = json.Unmarshal([]byte(bl.Fingerprint), &oldFP)
	if fp["title_hash"] == oldFP["title_hash"] && fp["text_hash"] == oldFP["text_hash"] && fp["body_hash"] == oldFP["body_hash"] {
		// 无变更：仅刷新 last_seen。
		s.DB.Model(&bl).Updates(map[string]any{"last_seen_at": now})
		return
	}
	// 有变更：计算变更维度。
	changedDims := []string{}
	if fp["title_hash"] != oldFP["title_hash"] {
		changedDims = append(changedDims, "标题")
	}
	if fp["text_hash"] != oldFP["text_hash"] {
		changedDims = append(changedDims, "正文")
	}
	if fp["body_hash"] != oldFP["body_hash"] {
		changedDims = append(changedDims, "页面结构")
	}
	s.DB.Model(&bl).Updates(map[string]any{
		"fingerprint": mustJSON(fp), "last_seen_at": now, "changed_at": now,
		"changed_count": bl.ChangedCount + 1,
	})
	// 生成篡改 finding。
	changedStr := strings.Join(changedDims, "、")
	finding := &models.Finding{
		TaskID: task.ID, AssetID: asset.ID, ResultID: resultID,
		EngineName: "content_security", Type: "content_integrity", Severity: "high",
		Title:       "内容完整性基线偏差: " + asset.Name,
		Description: "重点资产「" + asset.Name + "」内容发生变更，变更维度: " + changedStr + "，共变更 " + fmt.Sprint(len(changedDims)) + " 项。",
		URL:         wf.URL, Confidence: 0.9,
		Status: "open",
		Extra: mustJSON(map[string]any{
			"before": oldFP, "after": fp, "changed_dims": changedDims,
			"changed_at": now.Format(time.RFC3339), "changed_count": bl.ChangedCount + 1,
		}),
	}
	if err := s.DB.Create(finding).Error; err != nil {
		return
	}
	// 事件。
	event := &models.Event{
		AssetID: asset.ID, FindingIDs: fmt.Sprint(finding.ID),
		EngineName: "content_security", EventType: "篡改", Title: finding.Title,
		Severity: "high", URL: wf.URL, Content: finding.Description,
		Status: "pending",
	}
	if err := s.DB.Create(event).Error; err == nil {
		s.Hub.Broadcast(map[string]any{
			"type": "event.new", "data": map[string]any{"event_id": event.ID, "title": finding.Title, "severity": "high"},
		})
	}
	// 告警。
	alert := &models.Alert{
		AssetID: asset.ID, FindingID: finding.ID,
		AlertType: "tamper", Severity: "high", Title: finding.Title,
		Content: finding.Description, Status: "open",
	}
	if err := s.DB.Create(alert).Error; err == nil {
		s.Hub.Broadcast(map[string]any{
			"type": "alert.new", "data": map[string]any{"alert_id": alert.ID, "title": finding.Title, "severity": "high"},
		})
	}
}

// extLinkItem 外链清单项（与 worker external_link 结构对应）。
type extLinkItem struct {
	URL              string `json:"url"`
	Type             string `json:"type"`
	Domain           string `json:"domain"`
	Suspicious       bool   `json:"suspicious"`
	SuspiciousReason string `json:"suspicious_reason,omitempty"`
}

// processExternalLink 外链发现基线维护：资产页面外链清单建立/比对，
// 检测新增/移除/目标域名变更/可疑外链，生成 external_link finding 与暗链挂马事件。
func (s *WorkerService) processExternalLink(task models.ScanTask, resultID string, wf WorkerFinding) {
	rawLinks, ok := wf.Extra["external_links"].([]any)
	if !ok {
		return
	}
	var links []extLinkItem
	cur := map[string]extLinkItem{}
	for _, item := range rawLinks {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		e := extLinkItem{}
		e.URL, _ = m["url"].(string)
		e.Type, _ = m["type"].(string)
		e.Domain, _ = m["domain"].(string)
		e.Suspicious, _ = m["suspicious"].(bool)
		e.SuspiciousReason, _ = m["suspicious_reason"].(string)
		if e.URL == "" {
			continue
		}
		links = append(links, e)
		cur[e.URL] = e
	}
	now := time.Now()
	// 资产信息（来源页展示）。
	var asset models.Asset
	if err := s.DB.Where("id = ?", task.AssetID).First(&asset).Error; err != nil {
		asset = models.Asset{ID: task.AssetID, Name: fmt.Sprint(task.AssetID)}
	}
	var bl models.ExternalLinkBaseline
	err := s.DB.Where("asset_id = ? AND url = ?", asset.ID, wf.URL).First(&bl).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 首次采集：建立基线。
		s.DB.Create(&models.ExternalLinkBaseline{
			AssetID: asset.ID, URL: wf.URL,
			Links: mustJSON(links), FirstSeenAt: now, LastSeenAt: now,
		})
		// 首次即发现可疑外链也生成 finding（无需等待变更）。
		if s.emitSuspiciousExternalLink(task, resultID, asset, links) {
			return
		}
		return
	}
	if err != nil {
		return
	}
	// 比对基线。
	var oldLinks []extLinkItem
	_ = json.Unmarshal([]byte(bl.Links), &oldLinks)
	oldSet := map[string]extLinkItem{}
	for _, l := range oldLinks {
		oldSet[l.URL] = l
	}
	added, removed, changed := []extLinkItem{}, []extLinkItem{}, []extLinkItem{}
	for _, l := range links {
		o, existed := oldSet[l.URL]
		if !existed {
			added = append(added, l)
		} else if o.Domain != l.Domain {
			// 同 URL 目标域名变更（重定向/篡改）。
			changed = append(changed, l)
		}
	}
	for _, l := range oldLinks {
		if _, existed := cur[l.URL]; !existed {
			removed = append(removed, l)
		}
	}
	// 更新基线。
	s.DB.Model(&bl).Updates(map[string]any{
		"links": mustJSON(links), "last_seen_at": now,
	})
	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		// 无结构变更，仅检查可疑外链。
		if s.emitSuspiciousExternalLink(task, resultID, asset, links) {
			return
		}
		return
	}
	// 有变更：生成 finding + 事件。
	s.DB.Model(&bl).Updates(map[string]any{"changed_at": now, "changed_count": bl.ChangedCount + 1})
	dims := []string{}
	if len(added) > 0 {
		dims = append(dims, fmt.Sprintf("新增 %d 条", len(added)))
	}
	if len(removed) > 0 {
		dims = append(dims, fmt.Sprintf("移除 %d 条", len(removed)))
	}
	if len(changed) > 0 {
		dims = append(dims, fmt.Sprintf("目标域名变更 %d 条", len(changed)))
	}
	finding := &models.Finding{
		TaskID: task.ID, AssetID: asset.ID, ResultID: resultID,
		EngineName: "content_security", Type: "external_link", Severity: "medium",
		Title:       "外链变更: " + asset.Name,
		Description: "资产「" + asset.Name + "」外链清单发生变化: " + strings.Join(dims, "、"),
		URL:         wf.URL, Confidence: 0.9, Status: "open",
		Extra: mustJSON(map[string]any{
			"added": added, "removed": removed, "changed": changed,
			"link_tampered": len(changed) > 0,
			"changed_at":    now.Format(time.RFC3339),
		}),
	}
	if err := s.DB.Create(finding).Error; err != nil {
		return
	}
	s.createIntegrityEvent(asset.ID, finding, "暗链挂马", "外链变更", now)
}

// emitSuspiciousExternalLink 检测可疑外链（恶意域名/相似度），命中生成 high finding + 暗链挂马事件。
func (s *WorkerService) emitSuspiciousExternalLink(task models.ScanTask, resultID string, asset models.Asset, links []extLinkItem) bool {
	var suspicious []extLinkItem
	for _, l := range links {
		if l.Suspicious {
			suspicious = append(suspicious, l)
		}
	}
	if len(suspicious) == 0 {
		return false
	}
	now := time.Now()
	names := []string{}
	for _, l := range suspicious {
		names = append(names, l.URL)
	}
	finding := &models.Finding{
		TaskID: task.ID, AssetID: asset.ID, ResultID: resultID,
		EngineName: "content_security", Type: "external_link", Severity: "high",
		Title:       "可疑外链: " + asset.Name,
		Description: "资产「" + asset.Name + "」页面存在 " + fmt.Sprint(len(suspicious)) + " 条可疑外链（恶意域名库命中/域名相似度）: " + truncateText(strings.Join(names, ", "), 200),
		URL:         suspicious[0].URL, Confidence: 0.95, Status: "open",
		Extra: mustJSON(map[string]any{
			"suspicious_links": suspicious, "link_tampered": true,
			"detected_at": now.Format(time.RFC3339),
		}),
	}
	if err := s.DB.Create(finding).Error; err != nil {
		return false
	}
	s.createIntegrityEvent(asset.ID, finding, "暗链挂马", "可疑外链", now)
	return true
}

// createIntegrityEvent 生成事件 + WS 广播（外链/篡改通用）。
func (s *WorkerService) createIntegrityEvent(assetID int64, finding *models.Finding, eventType, title string, now time.Time) {
	event := &models.Event{
		AssetID: assetID, FindingIDs: fmt.Sprint(finding.ID),
		EngineName: "content_security", EventType: eventType, Title: title,
		Severity: finding.Severity, URL: finding.URL, Content: finding.Description,
		Status: "pending",
	}
	if err := s.DB.Create(event).Error; err == nil {
		s.Hub.Broadcast(map[string]any{
			"type": "event.new", "data": map[string]any{"event_id": event.ID, "title": title, "severity": finding.Severity},
		})
	}
	alert := &models.Alert{
		AssetID: assetID, FindingID: finding.ID,
		AlertType: "tamper", Severity: finding.Severity, Title: finding.Title,
		Content: finding.Description, Status: "open",
	}
	if err := s.DB.Create(alert).Error; err == nil {
		s.Hub.Broadcast(map[string]any{
			"type": "alert.new", "data": map[string]any{"alert_id": alert.ID, "title": finding.Title, "severity": finding.Severity},
		})
	}
}

// mustJSON 序列化任意值为 JSON 字符串，失败返回空对象。
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// truncateText 按 rune 截断字符串（超长加省略号）。
func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// aggregateVuln 按 asset+engine+签名 聚合并入 vulnerabilities。
func (s *WorkerService) aggregateVuln(f *models.Finding, wf WorkerFinding, evidenceIDs []int64) {
	sign := utils.MD5Hex(wf.EngineName + "|" + wf.Type + "|" + wf.URL)
	var vuln models.Vulnerability
	err := s.DB.Where("asset_id = ? AND engine_name = ? AND cve_id = ?",
		f.AssetID, wf.EngineName, sign).First(&vuln).Error
	now := time.Now()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		evIDs, _ := json.Marshal(evidenceIDs)
		vuln = models.Vulnerability{
			FindingID: f.ID, AssetID: f.AssetID, CVEID: sign,
			EngineName: wf.EngineName, Severity: wf.Severity, Status: "open",
			Title: wf.Title, Description: wf.Description, EvidenceIDs: string(evIDs),
			FirstSeenAt: now, LastSeenAt: now,
		}
		s.DB.Create(&vuln)
	} else {
		s.DB.Model(&vuln).Updates(map[string]any{"last_seen_at": now, "severity": wf.Severity})
	}
}

// persistSensitiveInfoHits 解析 extra.sensitive_info_hits 并写入命中明细表。
func (s *WorkerService) persistSensitiveInfoHits(taskID int64, extra map[string]any) {
	raw, ok := extra["sensitive_info_hits"]
	if !ok {
		return
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return
	}
	var hits []struct {
		Group       string `json:"group"`
		Name        string `json:"name"`
		MatchedText string `json:"matched_text"`
		Scope       string `json:"scope"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal(bytes, &hits); err != nil {
		return
	}
	for _, h := range hits {
		hit := &models.SensitiveInfoHit{
			TaskID: taskID, Group: h.Group, Name: h.Name,
			MatchedText: h.MatchedText, Scope: h.Scope, URL: h.URL,
		}
		_ = s.DB.Create(hit).Error
	}
}

// isNoisy 降噪过滤（MVP 支持白名单 IP 与忽略类型）。
func (s *WorkerService) isNoisy(url, engineName string) bool {
	var rules []models.NoiseRule
	s.DB.Where("enabled = ?", "true").Find(&rules)
	for _, r := range rules {
		var cfg map[string]any
		_ = json.Unmarshal([]byte(r.Config), &cfg)
		switch r.Type {
		case "whitelist_ip":
			if ip, ok := cfg["ip"].(string); ok && ip != "" && strings.Contains(url, ip) {
				return true
			}
		case "ignore_type":
			if et, ok := cfg["event_type"].(string); ok && et != "" && et == engineName {
				return true
			}
		}
	}
	return false
}

// expandEngineSwitches 把策略高级开关展开为 Worker 侧细粒度引擎名。
// 保留可直接识别的引擎名，映射 content/sensitive 等高级开关为对应子引擎。
func expandEngineSwitches(enabled []string) []string {
	expansion := map[string][]string{
		"sensitive": {"sensitive_word", "sensitive_info"},
		"content":   {"ai_classify", "dead_link", "keyword", "image_ocr", "external_link", "content_integrity"},
		"multi_ua":  {"multi_ua"},
		"webshell":  {"webshell"},
	}
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, name := range enabled {
		if subs, ok := expansion[name]; ok {
			for _, s := range subs {
				add(s)
			}
		} else {
			add(name)
		}
	}
	return out
}

func mapEvent(engine, findingType, severity string) (eventType, title string) {
	switch engine {
	case "vuln_scan":
		return "漏洞", "漏洞发现"
	case "content_security":
		switch findingType {
		case "sensitive_info":
			return "敏感信息泄漏", "敏感信息泄漏"
		case "content_violation":
			return "内容违规", "AI 分类内容违规"
		case "content_integrity":
			return "篡改", "页面篡改"
		case "external_link":
			return "暗链挂马", "外链发现异常"
		case "dead_link":
			return "可用性异常", "死链命中"
		case "keyword_hit":
			return "内容违规", "关键词命中"
		case "image_ocr":
			return "内容违规", "图片 OCR 识别命中"
		default:
			return "内容违规", "内容违规"
		}
	case "hidden_link":
		return "暗链挂马", "暗链挂马"
	case "webshell":
		return "Webshell", "Webshell 检测"
	case "phishing":
		return "钓鱼", "钓鱼仿冒"
	case "availability":
		return "可用性异常", "可用性异常"
	case "multi_ua":
		return "可用性异常", "端差异化宕机"
	case "port_service":
		return "端口暴露", "端口服务暴露"
	case "dns_security":
		if findingType == "certificate" {
			return "证书告警", "证书异常"
		}
		return "篡改", "DNS 异常"
	case "reputation":
		return "信誉异常", "信誉异常"
	case "intelligence":
		return "情报预警", "情报预警"
	default:
		return "漏洞", "安全发现"
	}
}

func isAlertWorthy(severity, findingType string) bool {
	if severity == "high" || severity == "critical" {
		return true
	}
	switch findingType {
	case "content_integrity", "hidden_link", "tamper", "port", "intel", "availability", "certificate":
		return true
	}
	return false
}

func alertTypeOf(engine string) string {
	switch engine {
	case "vuln_scan":
		return "vuln"
	case "content_security":
		return "content"
	case "hidden_link":
		return "hidden_link"
	case "webshell":
		return "webshell"
	case "phishing":
		return "phishing"
	case "availability":
		return "availability"
	case "port_service":
		return "port"
	case "dns_security":
		return "tamper"
	case "reputation":
		return "content"
	case "intelligence":
		return "intel"
	default:
		return "vuln"
	}
}

func randomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// persistDiscoveredAssets 递归扫描发现的子资产落 assets 表。
// 达到授权 max_assets 配额后停止写入新发现并标记 discovery_stopped: quota_exceeded。
func (s *WorkerService) persistDiscoveredAssets(task models.ScanTask, discovered []DiscoveredAsset) {
	maxAssets := 0
	if s.License != nil {
		maxAssets = s.License.MaxAssets()
	}
	var used int64
	s.DB.Model(&models.Asset{}).Where("status <> ?", models.StatusDeleted).Count(&used)
	quotaExceeded := maxAssets > 0 && int(used) >= maxAssets
	created := 0
	for _, d := range discovered {
		if d.URL == "" || d.SourceType == "" {
			continue
		}
		if quotaExceeded {
			break
		}
		// 同 URL 去重（含 source_type 区分）。
		var exist int64
		s.DB.Model(&models.Asset{}).Where("url = ?", d.URL).Count(&exist)
		if exist > 0 {
			continue
		}
		asset := &models.Asset{
			URL: d.URL, Name: d.URL, SourceType: d.SourceType,
			Importance: "medium", Status: models.StatusActive,
		}
		if err := s.DB.Create(asset).Error; err != nil {
			continue
		}
		created++
		used++
		if maxAssets > 0 && int(used) >= maxAssets {
			quotaExceeded = true
		}
	}
	// 任务进度：已发现资产数 + discovery_stopped 标记。
	progress := map[string]any{"discovered": created}
	if quotaExceeded {
		progress["discovery_stopped"] = "quota_exceeded"
	}
	_ = s.DB.Model(&models.ScanTask{}).Where("id = ?", task.ID).
		Update("progress_meta", toJSONString(progress)).Error
}

func toJSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
