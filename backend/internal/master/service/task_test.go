package service

import (
	"testing"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestTaskStopCancelsPending(t *testing.T) {
	gdb := newTestDB(t)
	s := NewTaskService(gdb, nil, nil)
	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusPending}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	updated, err := s.Stop(task.ID, 99, "op", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if updated.Status != models.StatusCancelled {
		t.Fatalf("expected cancelled, got %s", updated.Status)
	}
	if !updated.StoppedByUser {
		t.Fatal("expected stopped_by_user=true")
	}
	if updated.FinishedAt == nil {
		t.Fatal("expected finished_at set")
	}
}

func TestTaskStopSkipsCompleted(t *testing.T) {
	gdb := newTestDB(t)
	s := NewTaskService(gdb, nil, nil)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusCompleted}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	updated, err := s.Stop(task.ID, 99, "op", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if updated.Status != models.StatusCompleted {
		t.Fatalf("completed task must not be cancelled, got %s", updated.Status)
	}
}

func TestReportResultCompleted(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "r-1", TaskID: task.ID, Status: models.StatusCompleted, Progress: 100,
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var got models.ScanTask
	if err := gdb.First(&got, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if got.Status != models.StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
}

func TestReportResultDuplicateIdempotent(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, hub, nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	finding := WorkerFinding{EngineName: "recon", Type: "info", Severity: "low", Title: "t", URL: "http://x"}
	first := &WorkerResult{ResultID: "dup-1", TaskID: task.ID, Status: models.StatusCompleted, Progress: 100, Findings: []WorkerFinding{finding}}
	if _, err := s.ReportResult(first); err != nil {
		t.Fatalf("first report: %v", err)
	}
	res2, err := s.ReportResult(&WorkerResult{
		ResultID: "dup-1", TaskID: task.ID, Status: models.StatusFailed, Progress: 10, Message: "dup",
	})
	if err != nil {
		t.Fatalf("duplicate report: %v", err)
	}
	if dup, _ := res2["duplicated"].(bool); !dup {
		t.Fatal("expected duplicated=true on re-report")
	}
	var got models.ScanTask
	gdb.First(&got, task.ID)
	if got.Status != models.StatusCompleted {
		t.Fatalf("duplicate must not override status, got %s", got.Status)
	}
}

func TestReportResultCancelledByUser(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "c-1", TaskID: task.ID, Status: models.StatusCancelled, StoppedByUser: true,
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var got models.ScanTask
	gdb.First(&got, task.ID)
	if got.Status != models.StatusCancelled {
		t.Fatalf("expected cancelled, got %s", got.Status)
	}
	if !got.StoppedByUser {
		t.Fatal("expected stopped_by_user=true")
	}
}

func TestReportResultRetryLimitReached(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, nil, 10)

	task := models.ScanTask{
		AssetID: 1, PolicyID: 1,
		Status: models.StatusProcessing, RetryCount: 3,
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "r-3", TaskID: task.ID, Status: models.StatusFailed, Message: "boom",
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var got models.ScanTask
	gdb.First(&got, task.ID)
	if got.Status != models.StatusFailed {
		t.Fatalf("retry exhausted expected failed, got %s", got.Status)
	}
	if got.RetryCount != 3 {
		t.Fatalf("expected retry_count stays 3, got %d", got.RetryCount)
	}
	if got.RetryAt != nil {
		t.Fatal("expected no retry_at when exhausted")
	}
}

func TestReportResultRetrySchedulesBackoff(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "r-2", TaskID: task.ID, Status: models.StatusFailed, Message: "transient",
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var got models.ScanTask
	gdb.First(&got, task.ID)
	if got.Status != models.StatusPending {
		t.Fatalf("expected pending for retry, got %s", got.Status)
	}
	if got.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", got.RetryCount)
	}
	if got.RetryAt == nil {
		t.Fatal("expected retry_at set")
	}
	if got.RetryAt.Before(time.Now().Add(25 * time.Second)) {
		t.Fatalf("expected backoff ~30s, got %s", got.RetryAt.String())
	}
}

func TestReportResultTimeoutNoRetry(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, nil, 10)

	task := models.ScanTask{AssetID: 1, PolicyID: 1, Status: models.StatusProcessing}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	_, err := s.ReportResult(&WorkerResult{
		ResultID: "t-1", TaskID: task.ID, Status: models.StatusFailed, TaskTimeout: true,
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	var got models.ScanTask
	gdb.First(&got, task.ID)
	if got.Status != models.StatusFailed {
		t.Fatalf("timeout expected failed (no retry), got %s", got.Status)
	}
	if got.RetryCount != 0 {
		t.Fatalf("expected no retry on timeout, got %d", got.RetryCount)
	}
}
