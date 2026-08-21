// Package db 提供 SQLite/WAL 单写连接初始化与迁移。
package db

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// Init 打开 SQLite 数据库并配置 WAL/单写连接池，随后执行 AutoMigrate。
func Init(dbPath string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)",
		dbPath,
	)
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	// Master 单写约束：写并发收敛到单连接，配合 busy_timeout 与事务重试兜底。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := Migrate(gdb); err != nil {
		return nil, err
	}
	return gdb, nil
}

// Migrate 执行 AutoMigrate 初始建表 + 版本化迁移记录表。
func Migrate(gdb *gorm.DB) error {
	// 版本化迁移记录表先行创建。
	if err := gdb.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	if err := gdb.AutoMigrate(
		&models.Organization{},
		&models.User{},
		&models.UserOrg{},
		&models.Asset{},
		&models.WechatAsset{},
		&models.ScanPolicy{},
		&models.ScanTask{},
		&models.Finding{},
		&models.SensitiveInfoHit{},
		&models.RuleDefinition{},
		&models.Vulnerability{},
		&models.Alert{},
		&models.Event{},
		&models.Ticket{},
		&models.Evidence{},
		&models.EvidenceFile{},
		&models.ScanPlan{},
		&models.AuditLog{},
		&models.APIToken{},
		&models.NotifyChannel{},
		&models.NotifyRoute{},
		&models.NoiseRule{},
		&models.ScanWhitelist{},
		&models.WorkerNode{},
		&models.IntelItem{},
		&models.IntelSubscription{},
		&models.ReportTemplate{},
		&models.Report{},
		&models.Webhook{},
		&models.AvailabilityPoint{},
		&models.TrendPoint{},
		&models.EscalationRule{},
		&models.WatchShift{},
		&models.DailyWarReport{},
		&models.Scenario{},
		&models.SOPTemplate{},
		&models.ContentBaseline{},
	); err != nil {
		return fmt.Errorf("automigrate: %w", err)
	}
	return nil
}
