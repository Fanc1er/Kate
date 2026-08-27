package service

import (
	"testing"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestAvailabilityAddWhitelistValidation(t *testing.T) {
	gdb := newTestDB(t)
	s := NewAvailabilityService(gdb, nil)

	if _, err := s.AddWhitelist("bogus", "x", ""); err == nil {
		t.Fatal("expected error for invalid kind")
	}
	if _, err := s.AddWhitelist("domain", "", ""); err == nil {
		t.Fatal("expected error for empty value")
	}
}

func TestAvailabilityWhitelistLifecycle(t *testing.T) {
	gdb := newTestDB(t)
	s := NewAvailabilityService(gdb, nil)

	rule, err := s.AddWhitelist("domain", "example.com", "test")
	if err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	if rule.ID == 0 {
		t.Fatal("expected rule id")
	}
	if rule.Enabled != "true" {
		t.Fatalf("expected enabled=true, got %s", rule.Enabled)
	}

	list, err := s.Whitelist()
	if err != nil {
		t.Fatalf("list whitelist: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(list))
	}

	if err := s.RemoveWhitelist(rule.ID); err != nil {
		t.Fatalf("remove whitelist: %v", err)
	}
	list, err = s.Whitelist()
	if err != nil {
		t.Fatalf("list whitelist after remove: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 rules after remove, got %d", len(list))
	}
}

func TestAvailabilityListSort(t *testing.T) {
	gdb := newTestDB(t)
	now := time.Now()
	assets := []models.Asset{
		{Name: "bravo", URL: "https://b.com", Status: "active"},
		{Name: "alpha", URL: "https://a.com", Status: "active"},
		{Name: "charlie", URL: "https://c.com", Status: "active"},
	}
	for i := range assets {
		if err := gdb.Create(&assets[i]).Error; err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}
	points := []models.AvailabilityPoint{
		{AssetID: assets[0].ID, Engine: "availability", StatusCode: 500, ResponseMs: 300, SampledAt: now},
		{AssetID: assets[1].ID, Engine: "availability", StatusCode: 200, ResponseMs: 100, SampledAt: now.Add(-time.Hour)},
		{AssetID: assets[2].ID, Engine: "availability", StatusCode: 200, ResponseMs: 200, SampledAt: now.Add(time.Hour)},
	}
	for i := range points {
		if err := gdb.Create(&points[i]).Error; err != nil {
			t.Fatalf("create point: %v", err)
		}
	}

	s := NewAvailabilityService(gdb, nil)

	names := func(field, order string) []string {
		m, err := s.List("", "", "", 1, 20, field, order)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		list := m["list"].([]AvailabilityItem)
		out := make([]string, len(list))
		for i, it := range list {
			out[i] = it.Name
		}
		return out
	}

	if got := names("name", "asc"); len(got) != 3 || got[0] != "alpha" || got[2] != "charlie" {
		t.Fatalf("name asc order wrong: %v", got)
	}
	if got := names("response_ms", "desc"); len(got) != 3 || got[0] != "bravo" {
		t.Fatalf("response_ms desc order wrong: %v", got)
	}
	if got := names("sampled_at", "desc"); len(got) != 3 || got[0] != "charlie" {
		t.Fatalf("sampled_at desc order wrong: %v", got)
	}
}

func TestIsWhitelisted(t *testing.T) {
	gdb := newTestDB(t)
	for _, r := range []models.ScanWhitelist{
		{Kind: "domain", Value: "example.com", Enabled: "true"},
		{Kind: "ip", Value: "10.0.0.1", Enabled: "true"},
		{Kind: "cidr", Value: "192.168.0.0/16", Enabled: "true"},
	} {
		if err := gdb.Create(&r).Error; err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}
	s := NewWorkerService(gdb, nil, nil, NewHub(), nil, 10)

	cases := []struct {
		url  string
		want bool
	}{
		{"https://example.com/", true},
		{"https://sub.example.com/x", true},
		{"https://badexample.com/", false},
		{"http://10.0.0.1/", true},
		{"http://10.0.0.2/", false},
		{"http://192.168.5.5/", true},
		{"http://10.1.1.1/", false},
		{"not a url", false},
	}
	for _, c := range cases {
		if got := s.isWhitelisted(0, c.url); got != c.want {
			t.Errorf("isWhitelisted(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestAvailabilityListSparkline(t *testing.T) {
	gdb := newTestDB(t)
	now := time.Now()
	asset := models.Asset{Name: "spark", URL: "https://s.com", Status: "active"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	points := []models.AvailabilityPoint{
		{AssetID: asset.ID, Engine: "availability", StatusCode: 200, ResponseMs: 100, SampledAt: now.Add(-2 * time.Hour)},
		{AssetID: asset.ID, Engine: "availability", StatusCode: 0, ResponseMs: 0, SampledAt: now.Add(-time.Hour)},
		{AssetID: asset.ID, Engine: "availability", StatusCode: 200, ResponseMs: 800, SampledAt: now},
	}
	for i := range points {
		if err := gdb.Create(&points[i]).Error; err != nil {
			t.Fatalf("create point: %v", err)
		}
	}

	s := NewAvailabilityService(gdb, nil)
	m, err := s.List("", "", "", 1, 20, "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	list := m["list"].([]AvailabilityItem)
	var item *AvailabilityItem
	for i := range list {
		if list[i].AssetID == asset.ID {
			item = &list[i]
		}
	}
	if item == nil {
		t.Fatal("asset missing from list")
	}
	if len(item.Sparkline) != 3 {
		t.Fatalf("sparkline len = %d, want 3", len(item.Sparkline))
	}
	// 失败探测点必须标记 ok=false，成功点 ok=true。
	if !item.Sparkline[0].OK {
		t.Fatalf("successful point should be ok: %+v", item.Sparkline[0])
	}
	if item.Sparkline[1].OK {
		t.Fatalf("failed point should not be ok: %+v", item.Sparkline[1])
	}
	if item.Sparkline[2].ResponseMs != 800 {
		t.Fatalf("last ms = %d, want 800", item.Sparkline[2].ResponseMs)
	}

	// 状态筛选。
	filtered, err := s.List("", "normal", "", 1, 20, "", "")
	if err != nil {
		t.Fatalf("list normal: %v", err)
	}
	fl := filtered["list"].([]AvailabilityItem)
	for _, it := range fl {
		if it.Status == "abnormal" {
			t.Fatalf("abnormal asset leaked into normal filter")
		}
	}
}

// 超过 24h 未采样的资产应显示 unknown，旧点不再冒充最新状态。
func TestAvailabilityListStalePointUnknown(t *testing.T) {
	gdb := newTestDB(t)
	now := time.Now()
	asset := models.Asset{Name: "stale", URL: "https://old.com", Status: "active"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	stale := models.AvailabilityPoint{AssetID: asset.ID, Engine: "availability", StatusCode: 200, ResponseMs: 50, SampledAt: now.Add(-48 * time.Hour)}
	if err := gdb.Create(&stale).Error; err != nil {
		t.Fatalf("create stale point: %v", err)
	}

	s := NewAvailabilityService(gdb, nil)
	m, err := s.List("", "", "", 1, 20, "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	list := m["list"].([]AvailabilityItem)
	for _, it := range list {
		if it.AssetID == asset.ID && it.Status != "unknown" {
			t.Fatalf("stale asset status = %s, want unknown", it.Status)
		}
		if it.AssetID == asset.ID && it.StatusCode != 0 {
			t.Fatalf("stale asset should not carry old status_code, got %d", it.StatusCode)
		}
	}
}
