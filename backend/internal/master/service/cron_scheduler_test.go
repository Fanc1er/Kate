package service

import (
	"testing"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestMatchCronExprEveryMinute(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	if !matchCronExpr("* * * * *", now) {
		t.Fatal("每分钟表达式应匹配任意分钟")
	}
}

func TestMatchCronExprFixedMinute(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	if !matchCronExpr("30 * * * *", now) {
		t.Fatal("固定分钟应匹配")
	}
	if matchCronExpr("15 * * * *", now) {
		t.Fatal("非匹配分钟不应命中")
	}
}

func TestMatchCronExprStep(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	if !matchCronExpr("*/5 * * * *", now) {
		t.Fatal("30 应匹配 */5")
	}
	if matchCronExpr("*/7 * * * *", now) {
		t.Fatal("30 不应匹配 */7")
	}
}

func TestMatchCronExprHourAndWeekday(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC) // 周四
	if !matchCronExpr("30 10 * * 4", now) {
		t.Fatal("小时+星期匹配应命中")
	}
	if matchCronExpr("30 10 * * 1", now) {
		t.Fatal("星期不匹配不应命中")
	}
	if !matchCronExpr("*/15 8-23 * * *", now) {
		t.Fatal("小时范围 8-23 应包含 10 点")
	}
}

func TestCronSchedulerTriggerCreatesTasks(t *testing.T) {
	gdb := newTestDB(t)
	taskSvc := NewTaskService(gdb, nil, nil)
	cs := NewCronScheduler(gdb, taskSvc, nil)
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	cs.Now = func() time.Time { return now }

	// 构造资产、策略、计划。
	asset := models.Asset{URL: "https://example.com", Name: "test", GroupName: "web"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	policy := models.ScanPolicy{Name: "p1", Timeout: 60}
	if err := gdb.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	plan := models.ScanPlan{
		Name: "daily", PolicyID: policy.ID, AssetGroupName: "web",
		CronExpr: "30 * * * *", Status: "enabled",
	}
	if err := gdb.Create(&plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	cs.tick()

	var tasks []models.ScanTask
	if err := gdb.Find(&tasks).Error; err != nil {
		t.Fatalf("find tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("到点应创建 1 个任务, got %d", len(tasks))
	}
	if tasks[0].AssetID != asset.ID || tasks[0].PolicyID != policy.ID {
		t.Fatalf("任务关联错误: %+v", tasks[0])
	}
	var updated models.ScanPlan
	if err := gdb.First(&updated, plan.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.LastRunAt == nil {
		t.Fatal("LastRunAt 应更新")
	}
}

func TestCronSchedulerSameMinuteIdempotent(t *testing.T) {
	gdb := newTestDB(t)
	taskSvc := NewTaskService(gdb, nil, nil)
	cs := NewCronScheduler(gdb, taskSvc, nil)
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	cs.Now = func() time.Time { return now }

	asset := models.Asset{URL: "https://example.com", GroupName: "g"}
	_ = gdb.Create(&asset)
	policy := models.ScanPolicy{Name: "p"}
	_ = gdb.Create(&policy)
	plan := models.ScanPlan{
		PolicyID: policy.ID, AssetGroupName: "g",
		CronExpr: "* * * * *", Status: "enabled",
	}
	_ = gdb.Create(&plan)

	cs.tick()
	cs.tick() // 同一分钟内第二次触发应幂等。

	var count int64
	gdb.Model(&models.ScanTask{}).Count(&count)
	if count != 1 {
		t.Fatalf("同分钟内应只创建 1 个任务, got %d", count)
	}
}

func TestCronSchedulerDisabledPlanSkipped(t *testing.T) {
	gdb := newTestDB(t)
	taskSvc := NewTaskService(gdb, nil, nil)
	cs := NewCronScheduler(gdb, taskSvc, nil)
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	cs.Now = func() time.Time { return now }

	policy := models.ScanPolicy{Name: "p"}
	_ = gdb.Create(&policy)
	// disabled 计划不应触发。
	plan := models.ScanPlan{
		PolicyID: policy.ID, CronExpr: "* * * * *", Status: "disabled",
	}
	_ = gdb.Create(&plan)

	cs.tick()

	var count int64
	gdb.Model(&models.ScanTask{}).Count(&count)
	if count != 0 {
		t.Fatalf("禁用计划不应创建任务, got %d", count)
	}
}

func TestCronSchedulerNoMatchingAssets(t *testing.T) {
	gdb := newTestDB(t)
	taskSvc := NewTaskService(gdb, nil, nil)
	cs := NewCronScheduler(gdb, taskSvc, nil)
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	cs.Now = func() time.Time { return now }

	policy := models.ScanPolicy{Name: "p"}
	_ = gdb.Create(&policy)
	// 计划指向不存在的分组。
	plan := models.ScanPlan{
		PolicyID: policy.ID, AssetGroupName: "nonexist",
		CronExpr: "* * * * *", Status: "enabled",
	}
	_ = gdb.Create(&plan)

	cs.tick()

	var count int64
	gdb.Model(&models.ScanTask{}).Count(&count)
	if count != 0 {
		t.Fatalf("无资产分组不应创建任务, got %d", count)
	}
}

func TestCronSchedulerTimezone(t *testing.T) {
	gdb := newTestDB(t)
	taskSvc := NewTaskService(gdb, nil, nil)
	cs := NewCronScheduler(gdb, taskSvc, nil)
	// UTC 10:30，计划时区 Asia/Shanghai（+8）→ 本地 18:30。
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	cs.Now = func() time.Time { return now }

	asset := models.Asset{URL: "https://example.com", GroupName: "g"}
	_ = gdb.Create(&asset)
	policy := models.ScanPolicy{Name: "p"}
	_ = gdb.Create(&policy)
	plan := models.ScanPlan{
		PolicyID: policy.ID, AssetGroupName: "g",
		CronExpr: "30 18 * * *", Status: "enabled", Timezone: "Asia/Shanghai",
	}
	_ = gdb.Create(&plan)

	cs.tick()

	var count int64
	gdb.Model(&models.ScanTask{}).Count(&count)
	if count != 1 {
		t.Fatalf("时区换算后 18:30 应触发, got %d", count)
	}
}
