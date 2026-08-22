package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"path/filepath"
	"testing"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// newTestLicense 生成临时 RSA 密钥对并签发一份对应当前机器码的授权，
// 返回已导入 valid 状态的 Manager，用于资源配额相关测试。
func newTestLicense(t *testing.T, maxAssets, maxWorkers int) *license.Manager {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	t.Setenv("CINSIGHT_LICENSE_PUBLIC_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})))

	dir := t.TempDir()
	m, err := license.NewManager(filepath.Join(dir, "license.lic"), filepath.Join(dir, "salt"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	code, _, err := m.MachineCode()
	if err != nil {
		t.Fatalf("machine code: %v", err)
	}
	data, err := license.Issue(code, license.IssueOptions{
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		MaxAssets: maxAssets, MaxWorkers: maxWorkers,
	}, priv)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if st := m.Import(data); st != license.StatusValid {
		t.Fatalf("import status = %s, want valid", st)
	}
	return m
}

func TestContentIntegrityBaselineCreate(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	asset := models.Asset{URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	s.processContentIntegrity(task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{
			"content_fingerprint": map[string]any{
				"title_hash": "aaa", "text_hash": "bbb", "body_hash": "ccc",
			},
			"fingerprint_version": "v1",
		},
	})

	var bl models.ContentBaseline
	if err := gdb.Where("asset_id = ?", asset.ID).First(&bl).Error; err != nil {
		t.Fatalf("应建立基线: %v", err)
	}
	if bl.Fingerprint == "" {
		t.Fatal("基线应记录指纹")
	}
	// 首次采集不应产生 finding。
	var findCnt int64
	gdb.Model(&models.Finding{}).Count(&findCnt)
	if findCnt != 0 {
		t.Fatalf("首次采集不应产生 finding, got %d", findCnt)
	}
}

func TestContentIntegrityBaselineChange(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	asset := models.Asset{URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	fp := map[string]any{
		"title_hash": "aaa", "text_hash": "bbb", "body_hash": "ccc",
	}
	// 首次建立基线。
	s.processContentIntegrity(task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{"content_fingerprint": fp, "fingerprint_version": "v1"},
	})
	// 第二次：标题/正文变更。
	s.processContentIntegrity(task, "res-2", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{
			"content_fingerprint": map[string]any{
				"title_hash": "xxx", "text_hash": "yyy", "body_hash": "ccc",
			},
			"fingerprint_version": "v1",
		},
	})

	var find models.Finding
	if err := gdb.Where("type = ?", "content_integrity").First(&find).Error; err != nil {
		t.Fatalf("变更应产生 finding: %v", err)
	}
	if find.Severity != "high" {
		t.Fatalf("变更 severity 应为 high, got %s", find.Severity)
	}
	var extra map[string]any
	_ = json.Unmarshal([]byte(find.Extra), &extra)
	if extra["changed_dims"] == nil {
		t.Fatal("extra 应含 changed_dims")
	}
	dims, _ := extra["changed_dims"].([]any)
	if len(dims) != 2 {
		t.Fatalf("应检出标题+正文 2 个变更维度, got %d", len(dims))
	}
	// 事件应生成（篡改）。
	var evt models.Event
	if err := gdb.Where("event_type = ?", "篡改").First(&evt).Error; err != nil {
		t.Fatalf("应生成篡改事件: %v", err)
	}
	// 告警应生成。
	var alert models.Alert
	if err := gdb.Where("finding_id = ?", find.ID).First(&alert).Error; err != nil {
		t.Fatalf("应生成告警: %v", err)
	}
	// 基线应更新。
	var bl models.ContentBaseline
	gdb.Where("asset_id = ?", asset.ID).First(&bl)
	if bl.ChangedCount != 1 {
		t.Fatalf("changed_count 应为 1, got %d", bl.ChangedCount)
	}
}

func TestContentIntegritySkipsNonHigh(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	// importance=medium 资产不建基线。
	asset := models.Asset{URL: "https://example.com", Name: "普通", Importance: "medium"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	s.processContentIntegrity(task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{
			"content_fingerprint": map[string]any{
				"title_hash": "aaa", "text_hash": "bbb", "body_hash": "ccc",
			},
		},
	})

	var cnt int64
	gdb.Model(&models.ContentBaseline{}).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("非 high 资产不应建基线, got %d", cnt)
	}
}

func TestPersistDiscoveredAssets(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	lic := newTestLicense(t, 3, 0)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), lic, 10)

	asset := models.Asset{URL: "https://example.com", Name: "example"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	s.persistDiscoveredAssets(task, []DiscoveredAsset{
		{URL: "https://example.com/app.js", SourceType: "js"},
		{URL: "https://example.com/style.css", SourceType: "css"},
		{URL: "https://example.com/logo.png", SourceType: "image"},
	})

	var count int64
	gdb.Model(&models.Asset{}).Count(&count)
	// 已有 1 个手动资产 + 配额 3 → 最多新增 2 个，第 3 个触发 quota_exceeded。
	if count != 3 {
		t.Fatalf("配额 3 应只写入 2 个新资产, got %d", count)
	}

	var taskReload models.ScanTask
	gdb.First(&taskReload, task.ID)
	if taskReload.ProgressMeta == "" {
		t.Fatal("应记录 progress_meta")
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(taskReload.ProgressMeta), &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta["discovery_stopped"] != "quota_exceeded" {
		t.Fatalf("应标记 discovery_stopped=quota_exceeded, got %v", meta["discovery_stopped"])
	}
}

func TestPersistDiscoveredAssetsDedup(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	asset := models.Asset{URL: "https://example.com", Name: "example"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// 重复发现同一 URL，应去重。
	s.persistDiscoveredAssets(task, []DiscoveredAsset{
		{URL: "https://example.com/app.js", SourceType: "js"},
		{URL: "https://example.com/app.js", SourceType: "js"},
	})
	var count int64
	gdb.Model(&models.Asset{}).Count(&count)
	if count != 2 {
		t.Fatalf("应写入 1 个新资产（去重）+ 1 个已有, got %d", count)
	}
}

func TestExternalLinkBaselineFirst(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	asset := models.Asset{URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	links := []any{
		map[string]any{"url": "https://partner.com/x", "type": "third_party_domain", "domain": "partner.com", "suspicious": false},
		map[string]any{"url": "https://cdn.example.net/lib.js", "type": "external_resource", "domain": "example.net", "suspicious": false},
	}
	s.processExternalLink(task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "external_link", URL: asset.URL,
		Extra: map[string]any{"external_links": links},
	})

	var bl models.ExternalLinkBaseline
	if err := gdb.Where("asset_id = ?", asset.ID).First(&bl).Error; err != nil {
		t.Fatalf("应建立外链基线: %v", err)
	}
	if bl.Links == "" {
		t.Fatal("基线应记录外链清单")
	}
	// 首次无变更不产生 finding。
	var findCnt int64
	gdb.Model(&models.Finding{}).Count(&findCnt)
	if findCnt != 0 {
		t.Fatalf("首次采集不应产生 finding, got %d", findCnt)
	}
}

func TestExternalLinkBaselineChange(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	asset := models.Asset{URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// 首次：1 条外链。
	s.processExternalLink(task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "external_link", URL: asset.URL,
		Extra: map[string]any{"external_links": []any{
			map[string]any{"url": "https://partner.com/x", "type": "third_party_domain", "domain": "partner.com", "suspicious": false},
		}},
	})
	// 第二次：新增 1 条 + 移除原 1 条。
	s.processExternalLink(task, "res-2", WorkerFinding{
		EngineName: "content_security", Type: "external_link", URL: asset.URL,
		Extra: map[string]any{"external_links": []any{
			map[string]any{"url": "https://evil.net/y", "type": "third_party_domain", "domain": "evil.net", "suspicious": true, "suspicious_reason": "malicious_domain:evil.net"},
		}},
	})

	var find models.Finding
	if err := gdb.Where("type = ?", "external_link").Order("id ASC").First(&find).Error; err != nil {
		t.Fatalf("变更应产生 finding: %v", err)
	}
	var extra map[string]any
	_ = json.Unmarshal([]byte(find.Extra), &extra)
	added, _ := extra["added"].([]any)
	removed, _ := extra["removed"].([]any)
	if len(added) != 1 || len(removed) != 1 {
		t.Fatalf("应检出新增 1 + 移除 1, got added=%d removed=%d", len(added), len(removed))
	}
	// 事件（暗链挂马）。
	var evt models.Event
	if err := gdb.Where("event_type = ?", "暗链挂马").First(&evt).Error; err != nil {
		t.Fatalf("应生成暗链挂马事件: %v", err)
	}
}

func TestExternalLinkBaselineSuspicious(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	asset := models.Asset{URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// 首次采集即含可疑外链 → 生成 high finding。
	s.processExternalLink(task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "external_link", URL: asset.URL,
		Extra: map[string]any{"external_links": []any{
			map[string]any{"url": "https://evil.com/x", "type": "third_party_domain", "domain": "evil.com", "suspicious": true, "suspicious_reason": "malicious_domain:evil.com"},
		}},
	})

	var find models.Finding
	if err := gdb.Where("type = ?", "external_link").First(&find).Error; err != nil {
		t.Fatalf("可疑外链应产生 finding: %v", err)
	}
	if find.Severity != "high" {
		t.Fatalf("可疑外链 severity 应为 high, got %s", find.Severity)
	}
	var evt models.Event
	if err := gdb.Where("event_type = ?", "暗链挂马").First(&evt).Error; err != nil {
		t.Fatalf("应生成暗链挂马事件: %v", err)
	}
}
