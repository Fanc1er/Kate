package service

import (
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/repository"
)

// DashboardService 仪表盘聚合。
type DashboardService struct {
	DB *gorm.DB
}

// NewDashboardService 构造 DashboardService。
func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{DB: db}
}

// guard 返回单租户查询守卫（无组织隔离）。
func (s *DashboardService) guard() *repository.Guard {
	return repository.NewGuard(s.DB)
}

// Stats 统计卡片：资产数/任务数/高危数/今日告警/可用性/覆盖率。
func (s *DashboardService) Stats() (map[string]any, error) {
	var assets, tasks, tasksToday, findings, critical, high, alerts, evToday int64
	now := time.Now()
	todayStart := now.Truncate(24 * time.Hour)
	s.guard().Scoped(&models.Asset{}).Where("status <> ?", "deleted").Count(&assets)
	s.guard().Scoped(&models.ScanTask{}).Count(&tasks)
	s.guard().Scoped(&models.ScanTask{}).Where("created_at >= ?", todayStart).Count(&tasksToday)
	s.guard().Scoped(&models.Finding{}).Count(&findings)
	s.guard().Scoped(&models.Finding{}).Where("severity = ? AND status <> ?", "critical", "closed").Count(&critical)
	s.guard().Scoped(&models.Finding{}).Where("severity = ? AND status <> ?", "high", "closed").Count(&high)
	s.guard().Scoped(&models.Alert{}).Where("status = ?", "open").Count(&alerts)
	s.guard().Scoped(&models.Alert{}).Where("created_at >= ?", todayStart).Count(&evToday)

	// 可用性最近 24h 均值。
	var okPts, totalPts int64
	s.guard().Scoped(&models.AvailabilityPoint{}).Where("sampled_at >= ?", now.Add(-24*time.Hour)).Count(&totalPts)
	s.guard().Scoped(&models.AvailabilityPoint{}).Where("sampled_at >= ? AND status_code >= 200 AND status_code < 500", now.Add(-24*time.Hour)).Count(&okPts)
	avail := 0.0
	if totalPts > 0 {
		avail = float64(okPts) / float64(totalPts) * 100
	}
	// 引擎覆盖率：已上线引擎数/总引擎数。
	engineTotal := []string{"vuln_scan", "content_security", "hidden_link", "webshell", "phishing", "availability", "port_service", "dns_security", "reputation", "intelligence"}
	var engineUsed int64
	s.guard().Scoped(&models.Finding{}).Distinct("engine_name").Count(&engineUsed)
	coverage := 0.0
	if len(engineTotal) > 0 {
		coverage = float64(engineUsed) / float64(len(engineTotal)) * 100
	}
	return map[string]any{
		"assets":        assets,
		"tasks":         tasks,
		"tasks_today":   tasksToday,
		"findings":      findings,
		"critical":      critical,
		"high":          high,
		"alerts_open":   alerts,
		"events_today":  evToday,
		"availability":  round1(avail),
		"coverage":      round1(coverage),
	}, nil
}

// Trends 7 天趋势：findings/alerts/availability。
func (s *DashboardService) Trends(days int) (map[string]any, error) {
	if days <= 0 {
		days = 7
	}
	start := time.Now().Truncate(24 * time.Hour).AddDate(0, 0, -(days - 1))
	dates := make([]string, 0, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		dates = append(dates, d.Format("01-02"))
	}
	type dayCnt struct {
		Day string
		Cnt int64
	}
	query := func(table string) map[string]int64 {
		out := make(map[string]int64, days)
		for _, d := range dates {
			out[d] = 0
		}
		var rows []dayCnt
		s.DB.Table(table).
			Select("strftime('%m-%d', created_at) AS day, COUNT(*) AS cnt").
			Where("created_at >= ?", start).
			Group("day").Scan(&rows)
		for _, r := range rows {
			out[r.Day] = r.Cnt
		}
		return out
	}
	findings := query("findings")
	alerts := query("alerts")
	// 可用性每日均值。
	availOut := make(map[string]float64, days)
	for _, d := range dates {
		availOut[d] = 0
	}
	type dayAvail struct {
		Day  string
		Ok   int64
		Tot  int64
	}
	var availRows []dayAvail
	s.DB.Table("availability_points").
		Select("strftime('%m-%d', sampled_at) AS day, SUM(CASE WHEN status_code >= 200 AND status_code < 500 THEN 1 ELSE 0 END) AS ok, COUNT(*) AS tot").
		Where("sampled_at >= ?", start).Group("day").Scan(&availRows)
	for _, r := range availRows {
		if r.Tot > 0 {
			availOut[r.Day] = round1(float64(r.Ok) / float64(r.Tot) * 100)
		}
	}
	fArr := make([]int64, 0, days)
	aArr := make([]int64, 0, days)
	avArr := make([]float64, 0, days)
	for _, d := range dates {
		fArr = append(fArr, findings[d])
		aArr = append(aArr, alerts[d])
		avArr = append(avArr, availOut[d])
	}
	return map[string]any{"dates": dates, "findings": fArr, "alerts": aArr, "availability": avArr}, nil
}

// TopRisks 风险 Top10（按 risk_score）。
func (s *DashboardService) TopRisks(limit int) ([]models.Finding, error) {
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	var list []models.Finding
	err := s.DB.Where("status <> ?", "closed").
		Order("risk_score DESC, id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// EngineCoverage 引擎覆盖率明细。
func (s *DashboardService) EngineCoverage() ([]map[string]any, error) {
	engines := []struct {
		Key  string
		Name string
	}{
		{"vuln_scan", "漏洞扫描"}, {"content_security", "内容安全"}, {"hidden_link", "暗链检测"},
		{"webshell", "Webshell"}, {"phishing", "钓鱼仿冒"}, {"availability", "可用性监测"},
		{"port_service", "端口服务"}, {"dns_security", "DNS安全"}, {"reputation", "信誉风险"}, {"intelligence", "威胁情报"},
	}
	out := make([]map[string]any, 0, len(engines))
	for _, e := range engines {
		var cnt int64
		s.DB.Model(&models.Finding{}).Where("engine_name = ?", e.Key).Count(&cnt)
		enabled := cnt > 0
		out = append(out, map[string]any{"engine": e.Key, "name": e.Name, "enabled": enabled, "findings": cnt})
	}
	return out, nil
}

func round1(f float64) float64 {
	return float64(int(f*10)) / 10
}
