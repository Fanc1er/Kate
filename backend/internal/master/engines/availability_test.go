package engines

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	av := NewAvailabilityEngine()
	r.Register(av)
	if got := r.Get(NameAvailability); got == nil {
		t.Fatal("Get 应返回已注册引擎")
	}
	names := r.Names()
	found := false
	for _, n := range names {
		if n == NameAvailability {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names 应包含 availability, got %v", names)
	}
	if r.Get("not_exist") != nil {
		t.Fatal("未注册引擎应返回 nil")
	}
}

func TestRegistryEnabledNames(t *testing.T) {
	r := NewRegistry()
	r.Register(NewAvailabilityEngine())
	// 全部启用。
	on := r.EnabledNames(Policy{Enabled: map[string]bool{NameAvailability: true}})
	if len(on) != 1 || on[0] != NameAvailability {
		t.Fatalf("全部启用应返回 availability, got %v", on)
	}
	// 显式关闭。
	off := r.EnabledNames(Policy{Enabled: map[string]bool{NameAvailability: false}})
	if len(off) != 0 {
		t.Fatalf("关闭后应无启用引擎, got %v", off)
	}
}

func TestAvailabilityEngineOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	e := NewAvailabilityEngine()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, DefaultAvailabilityPolicy())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("正常响应不应有 finding, got %v", fs)
	}
}

func TestAvailabilityEngineHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	e := NewAvailabilityEngine()
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, DefaultAvailabilityPolicy())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(fs) == 0 {
		t.Fatal("5xx 应产生 finding")
	}
	if fs[0].Type != "http_error" || fs[0].Severity != SeverityHigh {
		t.Fatalf("应判定 http_error/high, got %+v", fs[0])
	}
}

func TestAvailabilityEngineSlowResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	e := NewAvailabilityEngine()
	p := Policy{Enabled: map[string]bool{NameAvailability: true}, FailCount: 1, SlowThresholdMS: 10}
	fs, err := e.Run(context.Background(), Target{URL: srv.URL}, p)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	foundSlow := false
	for _, f := range fs {
		if f.Type == "slow_response" {
			foundSlow = true
		}
	}
	if !foundSlow {
		t.Fatalf("慢响应应产生 slow_response finding, got %v", fs)
	}
}

func TestAvailabilityEngineDisabled(t *testing.T) {
	e := NewAvailabilityEngine()
	if e.Enabled(Policy{Enabled: map[string]bool{NameAvailability: false}}) {
		t.Fatal("显式关闭后应禁用")
	}
	if !e.Enabled(Policy{Enabled: nil}) {
		t.Fatal("未配置开关默认启用")
	}
}
