package search

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

// Document 搜索文档（assets/findings/events 统一索引）。
type Document struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"` // asset / finding / event
	Title    string `json:"title"`
	URL      string `json:"url"`
	Content  string `json:"content"`
	Severity string `json:"severity,omitempty"`
	Engine   string `json:"engine,omitempty"`
}

// Indexer Bleve 搜索索引管理器。
type Indexer struct {
	idx bleve.Index
	mu  sync.Mutex
}

var instance *Indexer
var once sync.Once

// NewIndexer 构造索引器（单例）。
func NewIndexer(path string) (*Indexer, error) {
	mapping := bleve.NewIndexMapping()
	mapping.DefaultMapping.AddFieldMappingsAt("title", bleve.NewTextFieldMapping())
	mapping.DefaultMapping.AddFieldMappingsAt("url", bleve.NewTextFieldMapping())
	mapping.DefaultMapping.AddFieldMappingsAt("content", bleve.NewTextFieldMapping())
	mapping.DefaultMapping.AddFieldMappingsAt("type", bleve.NewKeywordFieldMapping())
	mapping.DefaultMapping.AddFieldMappingsAt("severity", bleve.NewKeywordFieldMapping())
	mapping.DefaultMapping.AddFieldMappingsAt("engine", bleve.NewKeywordFieldMapping())

	idx, err := bleve.Open(path)
	if err != nil {
		idx, err = bleve.New(path, mapping)
		if err != nil {
			return nil, fmt.Errorf("create bleve index: %w", err)
		}
	}
	return &Indexer{idx: idx}, nil
}

// Close 关闭索引。
func (i *Indexer) Close() error {
	return i.idx.Close()
}

// Instance 获取全局索引器单例。
func Instance() *Indexer {
	once.Do(func() {
		path := "data/bleve"
		var err error
		instance, err = NewIndexer(path)
		if err != nil {
			instance = &Indexer{}
		}
	})
	return instance
}

// IndexAsset 索引资产。
func (i *Indexer) IndexAsset(a *models.Asset) error {
	if i.idx == nil {
		return nil
	}
	doc := Document{
		ID:      a.ID,
		Type:    "asset",
		Title:   a.Name,
		URL:     a.URL,
		Content: a.Remark,
	}
	return i.idx.Index(fmt.Sprintf("asset:%d", a.ID), doc)
}

// IndexFinding 索引发现。
func (i *Indexer) IndexFinding(f *models.Finding) error {
	if i.idx == nil {
		return nil
	}
	doc := Document{
		ID:       f.ID,
		Type:     "finding",
		Title:    f.Title,
		URL:      f.URL,
		Content:  f.Description,
		Severity: f.Severity,
		Engine:   f.EngineName,
	}
	return i.idx.Index(fmt.Sprintf("finding:%d", f.ID), doc)
}

// IndexEvent 索引事件。
func (i *Indexer) IndexEvent(e *models.Event) error {
	if i.idx == nil {
		return nil
	}
	doc := Document{
		ID:      e.ID,
		Type:    "event",
		Title:   e.Title,
		URL:     e.URL,
		Content: e.Content,
	}
	return i.idx.Index(fmt.Sprintf("event:%d", e.ID), doc)
}

// DeleteAsset 删除资产索引。
func (i *Indexer) DeleteAsset(id int64) error {
	if i.idx == nil {
		return nil
	}
	return i.idx.Delete(fmt.Sprintf("asset:%d", id))
}

// DeleteFinding 删除发现索引。
func (i *Indexer) DeleteFinding(id int64) error {
	if i.idx == nil {
		return nil
	}
	return i.idx.Delete(fmt.Sprintf("finding:%d", id))
}

// DeleteEvent 删除事件索引。
func (i *Indexer) DeleteEvent(id int64) error {
	if i.idx == nil {
		return nil
	}
	return i.idx.Delete(fmt.Sprintf("event:%d", id))
}

// Search 跨 assets/findings/events 全文搜索。
func (i *Indexer) Search(ctx context.Context, keyword string, page, pageSize int) ([]Document, int64, error) {
	if i.idx == nil || keyword == "" {
		return nil, 0, nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	mq := query.NewMatchQuery(keyword)
	mq.SetField("title")
	mq.SetField("content")

	terms := []query.Query{mq}
	termQ := query.NewBooleanQuery(nil, nil, terms)
	searchRequest := bleve.NewSearchRequest(termQ)
	searchRequest.Size = pageSize
	searchRequest.From = (page - 1) * pageSize

	result, err := i.idx.Search(searchRequest)
	if err != nil {
		return nil, 0, err
	}

	var docs []Document
	for _, hit := range result.Hits {
		doc := Document{
			ID:      parseIDFromKey(hit.ID),
			Type:    asString(hit.Fields["type"]),
			Title:   asString(hit.Fields["title"]),
			URL:     asString(hit.Fields["url"]),
			Content: asString(hit.Fields["content"]),
		}
		if s, ok := hit.Fields["severity"].(string); ok {
			doc.Severity = s
		}
		if e, ok := hit.Fields["engine"].(string); ok {
			doc.Engine = e
		}
		docs = append(docs, doc)
	}
	return docs, int64(result.Total), nil
}

// Rebuild 全量重建索引。
func RebuildIndex(db *gorm.DB) error {
	idx := Instance()
	if idx == nil || idx.idx == nil {
		return nil
	}
	if err := idx.idx.Close(); err != nil {
		return err
	}
	newIdx, err := bleve.Open("data/bleve")
	if err != nil {
		return err
	}
	idx.idx = newIdx

	var assets []models.Asset
	if err := db.Find(&assets).Error; err != nil {
		return err
	}
	for _, a := range assets {
		_ = idx.IndexAsset(&a)
	}

	var findings []models.Finding
	if err := db.Find(&findings).Error; err != nil {
		return err
	}
	for _, f := range findings {
		_ = idx.IndexFinding(&f)
	}

	var events []models.Event
	if err := db.Find(&events).Error; err != nil {
		return err
	}
	for _, e := range events {
		_ = idx.IndexEvent(&e)
	}
	return nil
}

func parseIDFromKey(key string) int64 {
	parts := splitKey(key)
	if len(parts) >= 2 {
		if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			return id
		}
	}
	return 0
}

func splitKey(key string) []string {
	var parts []string
	for i, c := range key {
		if c == ':' {
			parts = append(parts, key[:i], key[i+1:])
			break
		}
	}
	if len(parts) == 0 {
		return []string{key}
	}
	return parts
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
