package service

import (
	"encoding/json"
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestContentIntegrityBaselineCreate(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	org := models.Organization{Name: "org", Plan: "free", MaxAssets: 100}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}

	s.processContentIntegrity(org.ID, task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{
			"content_fingerprint": map[string]any{
				"title_hash": "aaa", "text_hash": "bbb", "body_hash": "ccc",
			},
			"fingerprint_version": "v1",
		},
	})

	var bl models.ContentBaseline
	if err := gdb.Where("org_id = ? AND asset_id = ?", org.ID, asset.ID).First(&bl).Error; err != nil {
		t.Fatalf("应建立基线: %v", err)
	}
	if bl.Fingerprint == "" {
		t.Fatal("基线应记录指纹")
	}
	// 首次采集不应产生 finding。
	var findCnt int64
	gdb.Model(&models.Finding{}).Where("org_id = ?", org.ID).Count(&findCnt)
	if findCnt != 0 {
		t.Fatalf("首次采集不应产生 finding, got %d", findCnt)
	}
}

func TestContentIntegrityBaselineChange(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "官网", Importance: "high"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	org := models.Organization{Name: "org", Plan: "free", MaxAssets: 100}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}

	fp := map[string]any{
		"title_hash": "aaa", "text_hash": "bbb", "body_hash": "ccc",
	}
	// 首次建立基线。
	s.processContentIntegrity(org.ID, task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{"content_fingerprint": fp, "fingerprint_version": "v1"},
	})
	// 第二次：标题/正文变更。
	s.processContentIntegrity(org.ID, task, "res-2", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{
			"content_fingerprint": map[string]any{
				"title_hash": "xxx", "text_hash": "yyy", "body_hash": "ccc",
			},
			"fingerprint_version": "v1",
		},
	})

	var find models.Finding
	if err := gdb.Where("org_id = ? AND type = ?", org.ID, "content_integrity").First(&find).Error; err != nil {
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
	if err := gdb.Where("org_id = ? AND event_type = ?", org.ID, "篡改").First(&evt).Error; err != nil {
		t.Fatalf("应生成篡改事件: %v", err)
	}
	// 告警应生成。
	var alert models.Alert
	if err := gdb.Where("org_id = ? AND finding_id = ?", org.ID, find.ID).First(&alert).Error; err != nil {
		t.Fatalf("应生成告警: %v", err)
	}
	// 基线应更新。
	var bl models.ContentBaseline
	gdb.Where("org_id = ? AND asset_id = ?", org.ID, asset.ID).First(&bl)
	if bl.ChangedCount != 1 {
		t.Fatalf("changed_count 应为 1, got %d", bl.ChangedCount)
	}
}

func TestContentIntegritySkipsNonHigh(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	// importance=medium 资产不建基线。
	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "普通", Importance: "medium"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	org := models.Organization{Name: "org", Plan: "free", MaxAssets: 100}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}

	s.processContentIntegrity(org.ID, task, "res-1", WorkerFinding{
		EngineName: "content_security", Type: "content_integrity", URL: asset.URL,
		Extra: map[string]any{
			"content_fingerprint": map[string]any{
				"title_hash": "aaa", "text_hash": "bbb", "body_hash": "ccc",
			},
		},
	})

	var cnt int64
	gdb.Model(&models.ContentBaseline{}).Where("org_id = ?", org.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("非 high 资产不应建基线, got %d", cnt)
	}
}

func TestPersistDiscoveredAssets(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "example"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	org := models.Organization{Name: "org", Plan: "free", MaxAssets: 3}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}

	s.persistDiscoveredAssets(org.ID, task, []DiscoveredAsset{
		{URL: "https://example.com/app.js", SourceType: "js"},
		{URL: "https://example.com/style.css", SourceType: "css"},
		{URL: "https://example.com/logo.png", SourceType: "image"},
	})

	var count int64
	gdb.Model(&models.Asset{}).Where("org_id = ?", org.ID).Count(&count)
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
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "example"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	org := models.Organization{Name: "org", Plan: "free", MaxAssets: 100}
	if err := gdb.Create(&org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}

	// 重复发现同一 URL，应去重。
	s.persistDiscoveredAssets(org.ID, task, []DiscoveredAsset{
		{URL: "https://example.com/app.js", SourceType: "js"},
		{URL: "https://example.com/app.js", SourceType: "js"},
	})
	var count int64
	gdb.Model(&models.Asset{}).Where("org_id = ?", org.ID).Count(&count)
	if count != 2 {
		t.Fatalf("应写入 1 个新资产（去重）+ 1 个已有, got %d", count)
	}
}
