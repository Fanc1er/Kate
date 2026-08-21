package service

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/db"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	gdb, err := db.Init(path)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	return gdb
}

func TestSchedulerRequeueStaleProcessing(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMasterScheduler(gdb, nil, nil)

	// 一个 processing 超过 30min 的任务。
	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 1,
		Status:    models.StatusProcessing,
		StartedAt: ptrTime(time.Now().Add(-40 * time.Minute)),
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	// 一个刚 processing 的任务不应被重置。
	fresh := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 1,
		Status:    models.StatusProcessing,
		StartedAt: ptrTime(time.Now()),
	}
	if err := gdb.Create(&fresh).Error; err != nil {
		t.Fatalf("create fresh task: %v", err)
	}

	s.reconcile()

	var stale models.ScanTask
	if err := gdb.First(&stale, task.ID).Error; err != nil {
		t.Fatalf("load stale: %v", err)
	}
	if stale.Status != models.StatusPending {
		t.Fatalf("超时任务状态 = %q, want pending", stale.Status)
	}

	var keep models.ScanTask
	if err := gdb.First(&keep, fresh.ID).Error; err != nil {
		t.Fatalf("load fresh: %v", err)
	}
	if keep.Status != models.StatusProcessing {
		t.Fatalf("新鲜任务状态 = %q, want processing", keep.Status)
	}
}

func TestSchedulerTimeoutToFailed(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMasterScheduler(gdb, nil, nil)

	// 平台默认策略，timeout=60min；任务 pending 超过 70min → failed。
	policy := models.ScanPolicy{OrgID: 0, Name: "default", Timeout: 60}
	if err := gdb.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: policy.ID,
		Status:    models.StatusPending,
		CreatedAt: time.Now().Add(-70 * time.Minute),
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// 未超时的任务保留。
	task2 := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: policy.ID,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
	}
	if err := gdb.Create(&task2).Error; err != nil {
		t.Fatalf("create task2: %v", err)
	}

	s.reconcile()

	var t1 models.ScanTask
	if err := gdb.First(&t1, task.ID).Error; err != nil {
		t.Fatalf("load t1: %v", err)
	}
	if t1.Status != models.StatusFailed {
		t.Fatalf("超时任务状态 = %q, want failed", t1.Status)
	}

	var t2 models.ScanTask
	if err := gdb.First(&t2, task2.ID).Error; err != nil {
		t.Fatalf("load t2: %v", err)
	}
	if t2.Status != models.StatusPending {
		t.Fatalf("未超时任务状态 = %q, want pending", t2.Status)
	}
}

func TestSchedulerNoPolicySkipped(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMasterScheduler(gdb, nil, nil)

	// 找不到策略时跳过，任务保持原状态不 panic。
	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 99999,
		Status:    models.StatusPending,
		CreatedAt: time.Now().Add(-5 * time.Hour),
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	s.reconcile()
	var got models.ScanTask
	if err := gdb.First(&got, task.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Status != models.StatusPending {
		t.Fatalf("找不到策略应跳过，状态 = %q", got.Status)
	}
}

func TestSchedulerRetryAtClearedWhenDue(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMasterScheduler(gdb, nil, nil)

	// 重试等待中的任务（retry_at 已到）应被清除 retry_at 使其可被拉取。
	due := time.Now().Add(-time.Minute)
	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 1,
		Status:    models.StatusPending,
		RetryAt:   &due,
		CreatedAt: time.Now(),
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	s.reconcile()
	var got models.ScanTask
	if err := gdb.First(&got, task.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RetryAt != nil {
		t.Fatalf("到期重试任务 retry_at 应被清空，仍为 %v", got.RetryAt)
	}
}

func TestSchedulerRetryAtNotDueKept(t *testing.T) {
	gdb := newTestDB(t)
	s := NewMasterScheduler(gdb, nil, nil)

	future := time.Now().Add(5 * time.Minute)
	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 1,
		Status:    models.StatusPending,
		RetryAt:   &future,
		CreatedAt: time.Now(),
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	s.reconcile()
	var got models.ScanTask
	if err := gdb.First(&got, task.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.RetryAt == nil {
		t.Fatalf("未到期重试任务 retry_at 不应被清空")
	}
}

func TestWorkerReportResultRetryBackoff(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, 100)

	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 1,
		Status: models.StatusProcessing,
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	// 第一次失败回传 → 应置 pending 并递增 retry_count。
	_, err := s.ReportResult(1, &WorkerResult{
		ResultID: "r-1", TaskID: task.ID, Status: "failed", Message: "boom", Progress: 50,
	})
	if err != nil {
		t.Fatalf("report result: %v", err)
	}
	var got models.ScanTask
	if err := gdb.First(&got, task.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Status != models.StatusPending {
		t.Fatalf("失败任务应自动重试置 pending，状态 = %q", got.Status)
	}
	if got.RetryCount != 1 {
		t.Fatalf("retry_count = %d, want 1", got.RetryCount)
	}
	if got.RetryAt == nil {
		t.Fatalf("retry_at 应被设置")
	}
}

func TestWorkerReportResultTimeoutNoRetry(t *testing.T) {
	gdb := newTestDB(t)
	hub := NewHub()
	s := NewWorkerService(gdb, nil, nil, hub, 100)

	task := models.ScanTask{
		OrgID: 1, AssetID: 1, PolicyID: 1,
		Status: models.StatusProcessing,
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	// 任务超时中止（task_timeout=true）不参与自动重试，直接 failed。
	_, err := s.ReportResult(1, &WorkerResult{
		ResultID: "r-2", TaskID: task.ID, Status: "failed",
		TaskTimeout: true, Message: "timeout", Progress: 50,
	})
	if err != nil {
		t.Fatalf("report result: %v", err)
	}
	var got models.ScanTask
	if err := gdb.First(&got, task.ID).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Status != models.StatusFailed {
		t.Fatalf("超时任务应直接 failed，状态 = %q", got.Status)
	}
	if got.RetryCount != 0 {
		t.Fatalf("超时任务不应重试，retry_count = %d", got.RetryCount)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
