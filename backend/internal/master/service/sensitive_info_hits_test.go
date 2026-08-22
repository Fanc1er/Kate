package service

import (
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestPersistSensitiveInfoHits(t *testing.T) {
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

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "si-1", TaskID: task.ID, Status: models.StatusCompleted, Progress: 100,
		Findings: []WorkerFinding{{
			EngineName: "content_security", Type: "sensitive_info", Severity: "high",
			Title: "敏感信息泄漏", Description: "desc", URL: "https://example.com", Confidence: 0.9,
			Extra: map[string]any{
				"sensitive_info_hits": []map[string]any{
					{"group": "身份证", "name": "身份证号码", "matched_text": "110101199003071234", "scope": "response body", "url": "https://example.com"},
					{"group": "手机号", "name": "中国大陆手机号", "matched_text": "13800138000", "scope": "response body", "url": "https://example.com"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	var hits []models.SensitiveInfoHit
	if err := gdb.Find(&hits).Error; err != nil {
		t.Fatalf("find hits: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("应落 2 条命中明细, got %d", len(hits))
	}
	if hits[0].TaskID != task.ID {
		t.Fatalf("task 错误: %+v", hits[0])
	}
}

func TestPersistSensitiveInfoHitsNoExtra(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	_ = gdb.Create(&task)

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "si-2", TaskID: task.ID, Status: models.StatusCompleted, Progress: 100,
		Findings: []WorkerFinding{{
			EngineName: "availability", Type: "http_error", Severity: "medium",
			Title: "HTTP 500", Description: "desc", URL: "https://example.com", Confidence: 0.9,
		}},
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var count int64
	gdb.Model(&models.SensitiveInfoHit{}).Count(&count)
	if count != 0 {
		t.Fatalf("非敏感信息 finding 不应落明细, got %d", count)
	}
}

func TestPersistSensitiveInfoHitsBadFormat(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	_ = gdb.Create(&task)

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "si-3", TaskID: task.ID, Status: models.StatusCompleted, Progress: 100,
		Findings: []WorkerFinding{{
			EngineName: "content_security", Type: "sensitive_info", Severity: "high",
			Title: "t", Description: "d", URL: "https://example.com", Confidence: 0.9,
			Extra: map[string]any{"sensitive_info_hits": "not-an-array"},
		}},
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var count int64
	gdb.Model(&models.SensitiveInfoHit{}).Count(&count)
	if count != 0 {
		t.Fatalf("格式错误不应落明细, got %d", count)
	}
}
