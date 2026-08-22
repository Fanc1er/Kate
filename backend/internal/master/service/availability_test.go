package service

import (
	"testing"

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
