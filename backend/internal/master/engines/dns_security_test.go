package engines

import (
	"context"
	"net"
	"testing"
)

type fakeResolver struct {
	ips []string
}

func (f fakeResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	var out []net.IP
	for _, s := range f.ips {
		if ip := net.ParseIP(s); ip != nil {
			out = append(out, ip)
		}
	}
	return out, nil
}

func TestDNSSecurityEnginePrivateIP(t *testing.T) {
	e := NewDNSSecurityEngine()
	fs, err := e.Run(context.Background(), Target{URL: "http://127.0.0.1"}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	var hasInternal bool
	for _, f := range fs {
		if f.Type == "dns_internal_ip" {
			hasInternal = true
			if f.Severity != SeverityHigh {
				t.Fatalf("severity = %s, want high", f.Severity)
			}
		}
	}
	if !hasInternal {
		t.Fatalf("内网地址应产出 dns_internal_ip finding, got %d", len(fs))
	}
}

func TestDNSSecurityEngineResolveFailure(t *testing.T) {
	e := NewDNSSecurityEngine()
	fs, err := e.Run(context.Background(), Target{URL: "https://invalid-host-unknown.invalid"}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	var hasFailed bool
	for _, f := range fs {
		if f.Type == "dns_resolution_failed" {
			hasFailed = true
		}
	}
	if !hasFailed {
		t.Fatalf("无法解析的主机应产出 dns_resolution_failed finding, got %d", len(fs))
	}
}

func TestDNSSecurityEngineMultiResolverConsistent(t *testing.T) {
	e := NewDNSSecurityEngine().WithResolvers(
		fakeResolver{ips: []string{"1.1.1.1", "8.8.8.8"}},
		fakeResolver{ips: []string{"8.8.8.8", "1.1.1.1"}},
	)
	fs, _ := e.Run(context.Background(), Target{URL: "http://example.com"}, Policy{})
	for _, f := range fs {
		if f.Type == "dns_resolver_inconsistent" {
			t.Fatalf("一致的解析结果不应产出 inconsistent: %v", f)
		}
	}
}

func TestDNSSecurityEngineMultiResolverInconsistent(t *testing.T) {
	e := NewDNSSecurityEngine().WithResolvers(
		fakeResolver{ips: []string{"1.1.1.1"}},
		fakeResolver{ips: []string{"9.9.9.9"}},
	)
	fs, _ := e.Run(context.Background(), Target{URL: "http://example.com"}, Policy{})
	var hasInconsistent bool
	for _, f := range fs {
		if f.Type == "dns_resolver_inconsistent" {
			hasInconsistent = true
			if f.Severity != SeverityHigh {
				t.Fatalf("severity = %s, want high", f.Severity)
			}
		}
	}
	if !hasInconsistent {
		t.Fatalf("不一致的解析结果应产出 dns_resolver_inconsistent, got %d", len(fs))
	}
}

func TestSameIPSet(t *testing.T) {
	a := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8")}
	b := []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("1.1.1.1")}
	c := []net.IP{net.ParseIP("9.9.9.9")}
	if !sameIPSet(a, b) {
		t.Fatalf("顺序无关的相同集合应相等")
	}
	if sameIPSet(a, c) {
		t.Fatalf("不同集合不应相等")
	}
}

func TestExtractHost(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://example.com/path", "example.com"},
		{"http://example.com:8080/x", "example.com"},
		{"example.com", "example.com"},
	}
	for _, c := range cases {
		if got := extractHost(c.raw); got != c.want {
			t.Fatalf("extractHost(%s) = %s, want %s", c.raw, got, c.want)
		}
	}
}

