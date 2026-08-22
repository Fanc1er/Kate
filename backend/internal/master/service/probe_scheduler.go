package service

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// ProbeScheduler 轻量可用性探针调度：按固定间隔为启用资产创建 availability_probe 任务。
type ProbeScheduler struct {
	DB       *gorm.DB
	Task     *TaskService
	License  *license.Manager
	Interval time.Duration
	Now      func() time.Time
}

// NewProbeScheduler 构造 ProbeScheduler。探测间隔默认 5 分钟，可经 CINSIGHT_AVAIL_INTERVAL_SECONDS 覆盖。
func NewProbeScheduler(db *gorm.DB, task *TaskService, lic *license.Manager) *ProbeScheduler {
	interval := 5 * time.Minute
	if v := os.Getenv("CINSIGHT_AVAIL_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}
	return &ProbeScheduler{DB: db, Task: task, License: lic, Interval: interval, Now: time.Now}
}

// Run 启动调度循环（每 30 秒检查一次到期资产）。
func (s *ProbeScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick 扫描到期资产并批量创建轻量探测任务。
// 到期判定：资产在 Interval 内无 AvailabilityPoint（从未探测或已过期）。
func (s *ProbeScheduler) tick() {
	if s.License != nil && s.License.Check() != license.StatusValid {
		return
	}
	if s.Task == nil {
		return
	}
	cutoff := s.Now().Add(-s.Interval)

	var allIDs []int64
	if err := s.DB.Model(&models.Asset{}).Where("status <> ?", "deleted").Pluck("id", &allIDs).Error; err != nil {
		log.Printf("probe-scheduler: load assets failed: %v", err)
		return
	}
	if len(allIDs) == 0 {
		return
	}
	var recentIDs []int64
	s.DB.Model(&models.AvailabilityPoint{}).
		Where("sampled_at >= ?", cutoff).
		Where("asset_id IN ?", allIDs).
		Distinct("asset_id").Pluck("asset_id", &recentIDs)
	recent := make(map[int64]bool, len(recentIDs))
	for _, id := range recentIDs {
		recent[id] = true
	}
	var due []int64
	for _, id := range allIDs {
		if !recent[id] {
			due = append(due, id)
		}
	}
	if len(due) == 0 {
		return
	}
	created, err := s.Task.CreateAvailabilityProbe(due)
	if err != nil {
		log.Printf("probe-scheduler: create probe tasks: %v", err)
		return
	}
	if len(created) > 0 {
		log.Printf("probe-scheduler: queued %d availability probes", len(created))
	}
}
