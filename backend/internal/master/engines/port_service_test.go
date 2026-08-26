package engines

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestDetectService(t *testing.T) {
	cases := map[int]string{
		22:   "SSH",
		443:  "HTTPS",
		3306: "MySQL",
		6379: "Redis",
		9999: "unknown",
	}
	for port, want := range cases {
		if got := detectService(port); got != want {
			t.Fatalf("detectService(%d) = %s, want %s", port, got, want)
		}
	}
}

func TestPortSeverity(t *testing.T) {
	if portSeverity(22) != SeverityHigh {
		t.Fatalf("port 22 应为 high")
	}
	if portSeverity(80) != SeverityLow {
		t.Fatalf("port 80 应为 low")
	}
}

func TestPortServiceEngineClosedExplicitPort(t *testing.T) {
	// 找一个未监听的本地端口作为显式端口。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	closedPort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	e := NewPortServiceEngine()
	fs, _ := e.Run(context.Background(), Target{URL: "http://" + addr}, Policy{})
	// 显式端口已关闭，不应产出该端口的 port_exposed。
	for _, f := range fs {
		if f.Type == "port_exposed" {
			if p, _ := f.Extra["port"].(int); p == closedPort {
				t.Fatalf("关闭的显式端口不应产出 port_exposed: %v", f)
			}
		}
	}
}

func TestPortServiceEngineOpenPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	e := NewPortServiceEngine()
	fs, _ := e.Run(context.Background(), Target{URL: srv.URL}, Policy{})
	// httptest 监听随机端口，显式端口应被检测到。
	var found bool
	for _, f := range fs {
		if f.Type == "port_exposed" {
			if p, _ := f.Extra["port"].(int); p == port {
				found = true
				if f.Severity != SeverityLow {
					t.Fatalf("非高危端口 severity = %s, want low", f.Severity)
				}
			}
		}
	}
	if !found {
		t.Fatalf("开放端口应产出 port_exposed, got %d", len(fs))
	}
}

func TestParseHostPort(t *testing.T) {
	cases := []struct {
		raw  string
		host string
		port int
	}{
		{"http://127.0.0.1:8080/x", "127.0.0.1", 8080},
		{"https://example.com", "example.com", 0},
		{"127.0.0.1:443", "127.0.0.1", 443},
		{"example.com", "example.com", 0},
	}
	for _, c := range cases {
		host, port, err := parseHostPort(c.raw)
		if err != nil {
			t.Fatalf("parseHostPort(%s) err: %v", c.raw, err)
		}
		if host != c.host || port != c.port {
			t.Fatalf("parseHostPort(%s) = (%s,%d), want (%s,%d)", c.raw, host, port, c.host, c.port)
		}
	}
}
