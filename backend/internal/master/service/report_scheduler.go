package service

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// ReportScheduler 定时报告调度：按模板 cron 表达式生成报告存档。
// 复用 CronScheduler 的 matchCronExpr（分 时 日 月 周，支持 */N），每 30 秒检查一次。
type ReportScheduler struct {
	DB     *gorm.DB
	Report *ReportService
	// Now 可注入时间（测试用），默认 time.Now。
	Now func() time.Time
}

// NewReportScheduler 构造 ReportScheduler。
func NewReportScheduler(db *gorm.DB, report *ReportService) *ReportScheduler {
	return &ReportScheduler{DB: db, Report: report, Now: time.Now}
}

// Run 启动定时循环。
func (s *ReportScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick 扫描启用中的模板，到点触发生成。
func (s *ReportScheduler) tick() {
	now := s.Now()
	var templates []models.ReportTemplate
	if err := s.DB.Where("enabled = ?", true).Find(&templates).Error; err != nil {
		log.Printf("report-scheduler: load templates failed: %v", err)
		return
	}
	for i := range templates {
		tpl := &templates[i]
		if !matchCronExpr(tpl.CronExpr, tInZone(now, tpl.Timezone)) {
			continue
		}
		// 同一分钟内避免重复触发。
		if tpl.LastRunAt != nil && tpl.LastRunAt.Format("200601021504") == now.Format("200601021504") {
			continue
		}
		rep, err := s.Report.Generate(tpl, now)
		if err != nil {
			log.Printf("report-scheduler: template %d generate failed: %v", tpl.ID, err)
		} else {
			log.Printf("report-scheduler: template %d generated report %d", tpl.ID, rep.ID)
		}
		if err := s.DB.Model(tpl).Updates(map[string]any{"last_run_at": now}).Error; err != nil {
			log.Printf("report-scheduler: template %d update last_run_at failed: %v", tpl.ID, err)
		}
	}
}

// tInZone 按模板时区换行判定时间，时区非法时回退本地时间。
func tInZone(now time.Time, timezone string) time.Time {
	if timezone == "" {
		return now
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return now
	}
	return now.In(loc)
}
