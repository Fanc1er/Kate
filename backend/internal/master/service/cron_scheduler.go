package service

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// CronScheduler Cron 定时计划触发：扫描 scan_plans，到点按资产分组批量创建任务。
// 支持表达式：`* * * * *`（分 时 日 月 周），字段支持 `*`、具体值、`*/N`（每 N 分钟/小时）。
type CronScheduler struct {
	DB   *gorm.DB
	Task *TaskService
	// License 授权管理器，为 nil 时不校验授权（测试用）。
	License *license.Manager
	// Now 可注入时间（测试用），默认 time.Now。
	Now func() time.Time
}

// NewCronScheduler 构造 CronScheduler。
func NewCronScheduler(db *gorm.DB, task *TaskService, lic *license.Manager) *CronScheduler {
	return &CronScheduler{DB: db, Task: task, License: lic, Now: time.Now}
}

// Run 启动定时循环（每 30 秒检查一次，分钟粒度触发）。
func (s *CronScheduler) Run(ctx context.Context) {
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

// tick 扫描启用中的计划，匹配当前时间则触发。
func (s *CronScheduler) tick() {
	// 授权无效时停止调度新的扫描任务。
	if s.License != nil && s.License.Check() != license.StatusValid {
		return
	}
	now := s.Now()
	var plans []models.ScanPlan
	if err := s.DB.Where("status = ?", "enabled").Find(&plans).Error; err != nil {
		log.Printf("cron-scheduler: load plans failed: %v", err)
		return
	}
	for i := range plans {
		p := &plans[i]
		expr := strings.TrimSpace(p.CronExpr)
		if expr == "" {
			continue
		}
		// 时区：计划可指定 timezone（如 Asia/Shanghai），默认本地时区。
		t := now
		if p.Timezone != "" {
			loc, err := time.LoadLocation(p.Timezone)
			if err == nil {
				t = now.In(loc)
			}
		}
		// 暂停/禁用跳过：status != enabled 已被查询过滤，此处幂等保护。
		if p.Status != "enabled" {
			continue
		}
		if !matchCronExpr(expr, t) {
			continue
		}
		// 同一分钟内避免重复触发：上次触发时间同分钟内跳过。
		if p.LastRunAt != nil && p.LastRunAt.In(t.Location()).Format("200601021504") == t.Format("200601021504") {
			continue
		}
		s.triggerPlan(p, t)
	}
}

// triggerPlan 按资产分组名查资产并批量创建任务，更新 LastRunAt。
func (s *CronScheduler) triggerPlan(p *models.ScanPlan, now time.Time) {
	if s.Task == nil {
		return
	}
	var assetIDs []int64
	q := s.DB.Model(&models.Asset{})
	if p.AssetGroupName != "" {
		q = q.Where("group_name = ?", p.AssetGroupName)
	}
	if err := q.Pluck("id", &assetIDs).Error; err != nil {
		log.Printf("cron-scheduler: load assets failed: %v", err)
		return
	}
	if len(assetIDs) == 0 {
		log.Printf("cron-scheduler: plan %d has no assets, skip", p.ID)
		return
	}
	_, err := s.Task.Create(TaskCreateReq{AssetIDs: assetIDs, PolicyID: p.PolicyID}, 0, "cron", "", "")
	if err != nil {
		// 去重冲突（已有进行中任务）时更新 LastRunAt 但跳过报错，视为本次已尝试。
		log.Printf("cron-scheduler: plan %d create tasks: %v", p.ID, err)
	}
	s.DB.Model(p).Updates(map[string]any{"last_run_at": now})
}

// matchCronExpr 判断时间 t 是否匹配 cron 表达式（分 时 日 月 周，支持 */N）。
// 仅分钟/小时/星期参与判定（日/月恒匹配），满足任务调度最小闭环。
func matchCronExpr(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) < 5 {
		return false
	}
	minute := fields[0]
	hour := fields[1]
	weekday := fields[4]
	if !matchField(minute, t.Minute(), 0, 59) {
		return false
	}
	if !matchField(hour, t.Hour(), 0, 23) {
		return false
	}
	if weekday != "*" && !matchField(weekday, int(t.Weekday()), 0, 6) {
		return false
	}
	return true
}

// matchField 判断值 val 是否匹配字段表达式：`*` / 固定值 / `*/N`（每 N）。
func matchField(field string, val, min, max int) bool {
	field = strings.TrimSpace(field)
	if field == "*" {
		return true
	}
	if strings.HasPrefix(field, "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		if err != nil || n <= 0 {
			return false
		}
		return val%n == 0
	}
	if strings.Contains(field, ",") {
		for _, part := range strings.Split(field, ",") {
			if matchField(part, val, min, max) {
				return true
			}
		}
		return false
	}
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) == 2 {
			a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil && val >= a && val <= b {
				return true
			}
		}
		return false
	}
	n, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	return val == n
}
