package service

import (
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// AvailabilityService 可用性监测：站点状态聚合、时序查询、重新探测、白名单、工作节点拓扑。
type AvailabilityService struct {
	DB   *gorm.DB
	Task *TaskService
}

// NewAvailabilityService 构造 AvailabilityService。
func NewAvailabilityService(db *gorm.DB, task *TaskService) *AvailabilityService {
	return &AvailabilityService{DB: db, Task: task}
}

// Reprobe 对指定资产立即创建轻量可用性探测任务，返回实际入队数量。
func (s *AvailabilityService) Reprobe(assetIDs []int64) (int, error) {
	created, err := s.Task.CreateAvailabilityProbe(assetIDs)
	if err != nil {
		return 0, err
	}
	return len(created), nil
}

// Whitelist 查询白名单规则列表。
func (s *AvailabilityService) Whitelist() ([]models.ScanWhitelist, error) {
	var list []models.ScanWhitelist
	if err := s.DB.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// AddWhitelist 新增白名单规则（kind：domain/ip/cidr）。
func (s *AvailabilityService) AddWhitelist(kind, value, remark string) (*models.ScanWhitelist, error) {
	if kind == "" || value == "" {
		return nil, errs.New(errs.CodeValidationFailed, "规则类型与值必填")
	}
	if kind != "domain" && kind != "ip" && kind != "cidr" {
		return nil, errs.New(errs.CodeValidationFailed, "规则类型仅支持 domain/ip/cidr")
	}
	rule := &models.ScanWhitelist{Kind: kind, Value: value, Remark: remark, Enabled: "true"}
	if err := s.DB.Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

// RemoveWhitelist 删除白名单规则。
func (s *AvailabilityService) RemoveWhitelist(id int64) error {
	if err := s.DB.Delete(&models.ScanWhitelist{}, "id = ?", id).Error; err != nil {
		return err
	}
	return nil
}

// SparkPoint 迷你趋势图数据点。
type SparkPoint struct {
	ResponseMs int  `json:"response_ms"`
	OK         bool `json:"ok"` // 探测成功（2xx/3xx）
}

// AvailabilityItem 站点可用性聚合项。
type AvailabilityItem struct {
	AssetID    int64        `json:"asset_id"`
	Name       string       `json:"name"`
	URL        string       `json:"url"`
	GroupName  string       `json:"group_name"`
	Importance string       `json:"importance"`
	StatusCode int          `json:"status_code"`
	ResponseMs int          `json:"response_ms"`
	SampledAt  *time.Time   `json:"sampled_at"`
	Status     string       `json:"availability_status"` // normal/abnormal/unknown
	Sparkline  []SparkPoint `json:"sparkline"`           // 最近 24h 响应耗时序列（升序，含探测结果标记）
}

// List 站点可用性列表：聚合最新时序点，支持关键词/状态/状态码分组筛选、排序与分页。
// 状态码分组：2xx/3xx/4xx/5xx（按首位匹配）。status：normal/abnormal/unknown。
// sortField：name/url/status_code/response_ms/sampled_at/availability_status；sortOrder：asc/desc。
func (s *AvailabilityService) List(keyword, status, statusCodeGroup string, page, pageSize int, sortField, sortOrder string) (map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	var assets []models.Asset
	if err := s.DB.Where("status <> ?", "deleted").Order("id ASC").Find(&assets).Error; err != nil {
		return nil, err
	}

	// 关键词预筛。
	filtered := assets[:0]
	kw := strings.TrimSpace(strings.ToLower(keyword))
	for _, a := range assets {
		if kw != "" && !strings.Contains(strings.ToLower(a.Name), kw) && !strings.Contains(strings.ToLower(a.URL), kw) {
			continue
		}
		filtered = append(filtered, a)
	}
	assets = filtered
	if len(assets) == 0 {
		return map[string]any{"list": []AvailabilityItem{}, "total": 0}, nil
	}

	ids := make([]int64, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}

	// 最新时序点（每资产一条）：仅看最近 24h，超期未探测的资产按无数据显示
	// unknown（拿几周前的旧点冒充"最新状态"会误导，且随点数增长全量查询会拖慢列表）。
	since := time.Now().Add(-24 * time.Hour)
	var points []models.AvailabilityPoint
	s.DB.Where("asset_id IN ? AND sampled_at >= ?", ids, since).Order("sampled_at DESC, id DESC").Find(&points)
	latest := make(map[int64]*models.AvailabilityPoint, len(points))
	for i := range points {
		p := &points[i]
		if _, ok := latest[p.AssetID]; !ok {
			latest[p.AssetID] = p
		}
	}

	// 最近 24h 响应耗时序列（sparkline）。
	type sparkRow struct {
		AssetID    int64
		ResponseMs int
		StatusCode int
	}
	var spark []sparkRow
	s.DB.Model(&models.AvailabilityPoint{}).
		Select("asset_id, response_ms, status_code").
		Where("asset_id IN ? AND sampled_at >= ?", ids, time.Now().Add(-24*time.Hour)).
		Order("sampled_at ASC").
		Scan(&spark)
	sparkline := make(map[int64][]SparkPoint, len(assets))
	for _, r := range spark {
		ok := r.StatusCode >= 200 && r.StatusCode < 400
		sparkline[r.AssetID] = append(sparkline[r.AssetID], SparkPoint{ResponseMs: r.ResponseMs, OK: ok})
	}

	// 状态码分组首位数（2/3/4/5）。
	codeGroup := 0
	if len(statusCodeGroup) >= 2 && strings.HasSuffix(statusCodeGroup, "xx") {
		switch statusCodeGroup[0] {
		case '2', '3', '4', '5':
			codeGroup = int(statusCodeGroup[0] - '0')
		}
	}

	items := make([]AvailabilityItem, 0, len(assets))
	for _, a := range assets {
		p := latest[a.ID]
		item := AvailabilityItem{
			AssetID:    a.ID,
			Name:       a.Name,
			URL:        a.URL,
			GroupName:  a.GroupName,
			Importance: a.Importance,
			Sparkline:  sparkline[a.ID],
		}
		if p != nil {
			item.StatusCode = p.StatusCode
			item.ResponseMs = p.ResponseMs
			t := p.SampledAt
			item.SampledAt = &t
			if p.StatusCode >= 200 && p.StatusCode < 400 {
				item.Status = "normal"
			} else {
				item.Status = "abnormal"
			}
		} else {
			item.Status = "unknown"
			item.Sparkline = []SparkPoint{}
		}
		// 状态筛选。
		if status != "" && item.Status != status {
			continue
		}
		// 状态码分组筛选。
		if codeGroup != 0 && item.StatusCode/100 != codeGroup {
			continue
		}
		items = append(items, item)
	}

	sortItems(items, sortField, sortOrder)

	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return map[string]any{"list": items[start:end], "total": total}, nil
}

// sortItems 按字段对可用性列表排序，仅白名单字段生效，其余保持不变（稳定排序）。
func sortItems(items []AvailabilityItem, field, order string) {
	if field == "" {
		return
	}
	desc := order == "desc"
	var less func(i, j int) bool
	switch field {
	case "name":
		less = func(i, j int) bool { return items[i].Name < items[j].Name }
	case "url":
		less = func(i, j int) bool { return items[i].URL < items[j].URL }
	case "status_code":
		less = func(i, j int) bool { return items[i].StatusCode < items[j].StatusCode }
	case "response_ms":
		less = func(i, j int) bool { return items[i].ResponseMs < items[j].ResponseMs }
	case "availability_status":
		less = func(i, j int) bool { return items[i].Status < items[j].Status }
	case "sampled_at":
		less = func(i, j int) bool {
			return sampledAtOf(items[i]).Before(sampledAtOf(items[j]))
		}
	default:
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}

func sampledAtOf(it AvailabilityItem) time.Time {
	if it.SampledAt == nil {
		return time.Time{}
	}
	return *it.SampledAt
}

// Timeseries 单资产可用性时序（默认最近 24h，按采样时间升序）。
func (s *AvailabilityService) Timeseries(assetID int64, hours int) ([]models.AvailabilityPoint, error) {
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	from := time.Now().Add(-time.Duration(hours) * time.Hour)
	var pts []models.AvailabilityPoint
	if err := s.DB.Where("asset_id = ? AND sampled_at >= ?", assetID, from).
		Order("sampled_at ASC").Find(&pts).Error; err != nil {
		return nil, err
	}
	return pts, nil
}

// WorkerTopology 工作节点拓扑：master 元信息 + worker 节点列表。
func (s *AvailabilityService) WorkerTopology() (map[string]any, error) {
	var workers []models.WorkerNode
	if err := s.DB.Order("id ASC").Find(&workers).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"master":  map[string]any{"name": "master", "role": "master", "status": "online"},
		"workers": workers,
	}, nil
}
