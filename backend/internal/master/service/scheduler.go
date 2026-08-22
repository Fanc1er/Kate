package service

import (
	"context"
	"log"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/badger"
)

// MasterScheduler 后台定时任务：任务超时对账 + 过期证据清理 + Badger 缓存一致性对账。
type MasterScheduler struct {
	DB       *gorm.DB
	Cache    *badger.Store
	Evidence *EvidenceService
}

// NewMasterScheduler 构造 MasterScheduler。
func NewMasterScheduler(db *gorm.DB, cache *badger.Store, evidence *EvidenceService) *MasterScheduler {
	return &MasterScheduler{DB: db, Cache: cache, Evidence: evidence}
}

// Run 启动后台循环（每 1 分钟对账一次，Badger 一致性每小时一次）。
func (s *MasterScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	s.reconcile()
	lastConsistency := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcile()
			if time.Since(lastConsistency) >= time.Hour {
				s.reconcileBadger()
				lastConsistency = time.Now()
			}
		}
	}
}

// reconcileBadger BadgerDB-SQLite 一致性对账：以 SQLite 为准清理脏 URL 防重键。
func (s *MasterScheduler) reconcileBadger() {
	if s.Cache == nil {
		return
	}
	kvs, err := s.Cache.ScanPrefix("urlmd5:")
	if err != nil {
		log.Printf("master-scheduler: badger scan failed: %v", err)
		return
	}
	removed := 0
	for _, kv := range kvs {
		// key 形如 urlmd5:{md5}，value 为资产 ID。
		assetID, err := strconv.ParseInt(kv.Value, 10, 64)
		if err != nil {
			continue
		}
		var count int64
		s.DB.Model(&models.Asset{}).Where("id = ?", assetID).Count(&count)
		if count == 0 {
			// 脏 key：SQLite 中无对应资产，清理。
			_ = s.Cache.Delete(kv.Key)
			removed++
		}
	}
	if removed > 0 {
		log.Printf("master-scheduler: badger consistency removed %d stale keys", removed)
	}
}

// reconcile 对账：
// 1) processing 超过 30min 的任务重置为 pending（可重新拉取）。
// 2) pending 超过任务级超时上限的任务置 failed（timeout）。
// 3) 过期证据清理。
func (s *MasterScheduler) reconcile() {
	now := time.Now()
	// 超时任务：processing 超 30min 重置 pending。
	res := s.DB.Model(&models.ScanTask{}).
		Where("status = ? AND started_at IS NOT NULL AND started_at < ?", models.StatusProcessing, now.Add(-30*time.Minute)).
		Updates(map[string]any{"status": models.StatusPending, "started_at": nil, "message": "worker 超时，任务重新排队"})
	if res.Error == nil && res.RowsAffected > 0 {
		log.Printf("master-scheduler: requeued %d stale tasks", res.RowsAffected)
	}
	// 任务级超时（策略 timeout 分钟）：pending 太久未开始的任务置 failed（含重试等待中的任务按创建时间计）。
	var stale []models.ScanTask
	s.DB.Where("status = ? AND retry_at IS NOT NULL AND retry_at <= ?", models.StatusPending, now).Find(&stale)
	for _, t := range stale {
		s.DB.Model(&t).Updates(map[string]any{"retry_at": nil})
	}
	s.DB.Where("status = ?", models.StatusPending).Find(&stale)
	for _, t := range stale {
		var policy models.ScanPolicy
		if err := s.DB.Where("id = ?", t.PolicyID).First(&policy).Error; err != nil {
			continue
		}
		timeout := time.Duration(policy.Timeout) * time.Minute
		if timeout <= 0 {
			timeout = 60 * time.Minute
		}
		if now.Sub(t.CreatedAt) > timeout {
			s.DB.Model(&t).Updates(map[string]any{"status": models.StatusFailed, "message": "任务超时未开始", "finished_at": now})
		}
	}
	// 过期证据清理（expires_at < now 的证据文件删除并回收空间）。
	if s.Evidence != nil {
		s.Evidence.Cleanup()
	}
}
