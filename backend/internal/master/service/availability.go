package service

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// AvailabilityService 可用性监测：站点状态聚合、时序查询、工作节点拓扑。
type AvailabilityService struct {
	DB *gorm.DB
}

// NewAvailabilityService 构造 AvailabilityService。
func NewAvailabilityService(db *gorm.DB) *AvailabilityService {
	return &AvailabilityService{DB: db}
}

// AvailabilityItem 站点可用性聚合项。
type AvailabilityItem struct {
	AssetID     int64      `json:"asset_id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	GroupName   string     `json:"group_name"`
	Importance  string     `json:"importance"`
	StatusCode  int        `json:"status_code"`
	ResponseMs  int        `json:"response_ms"`
	SampledAt   *time.Time `json:"sampled_at"`
	Status      string     `json:"availability_status"` // normal/abnormal/unknown
	Sparkline   []int      `json:"sparkline"`           // 最近 24h 响应耗时序列（升序）
}

// List 站点可用性列表：聚合最新时序点，支持关键词/状态/状态码分组筛选与分页。
// 状态码分组：2xx/3xx/4xx/5xx（按首位匹配）。status：normal/abnormal/unknown。
func (s *AvailabilityService) List(keyword, status, statusCodeGroup string, page, pageSize int) (map[string]any, error) {
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

	// 最新时序点（每资产一条）。
	var points []models.AvailabilityPoint
	s.DB.Where("asset_id IN ?", ids).Order("sampled_at DESC, id DESC").Find(&points)
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
	}
	var spark []sparkRow
	s.DB.Model(&models.AvailabilityPoint{}).
		Select("asset_id, response_ms").
		Where("asset_id IN ? AND sampled_at >= ?", ids, time.Now().Add(-24*time.Hour)).
		Order("sampled_at ASC").
		Scan(&spark)
	sparkline := make(map[int64][]int, len(assets))
	for _, r := range spark {
		sparkline[r.AssetID] = append(sparkline[r.AssetID], r.ResponseMs)
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
			item.Sparkline = []int{}
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
