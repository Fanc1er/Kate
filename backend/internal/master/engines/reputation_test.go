package engines

import (
	"context"
	"testing"
)

type fakeReputationFetcher struct{ score int }

func (f fakeReputationFetcher) Lookup(ctx context.Context, host string) (int, error) {
	return f.score, nil
}

func TestReputationEngineThreatDomain(t *testing.T) {
	e := NewReputationEngine()
	fs, err := e.Run(context.Background(), Target{URL: "http://malware.example.com"}, Policy{})
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	var hasThreat bool
	for _, f := range fs {
		if f.Type == "threat_domain" {
			hasThreat = true
			if f.Severity != SeverityHigh {
				t.Fatalf("severity = %s, want high", f.Severity)
			}
		}
	}
	if !hasThreat {
		t.Fatalf("恶意域名应产出 threat_domain finding, got %d", len(fs))
	}
}

func TestReputationEngineCleanDomain(t *testing.T) {
	e := NewReputationEngine()
	fs, _ := e.Run(context.Background(), Target{URL: "https://example.com"}, Policy{})
	for _, f := range fs {
		if f.Type == "threat_domain" {
			t.Fatalf("正常域名不应产出 threat_domain: %v", f)
		}
	}
}

func TestReputationEngineFetcherHigh(t *testing.T) {
	e := NewReputationEngine().WithReputationFetcher(fakeReputationFetcher{score: 85})
	fs, _ := e.Run(context.Background(), Target{URL: "https://scan-target.com"}, Policy{})
	var hasScore bool
	for _, f := range fs {
		if f.Type == "intel_threat_score" {
			hasScore = true
			if f.Severity != SeverityHigh {
				t.Fatalf("severity = %s, want high", f.Severity)
			}
		}
	}
	if !hasScore {
		t.Fatalf("高情报评分应产出 intel_threat_score, got %d", len(fs))
	}
}

func TestReputationEngineFetcherLow(t *testing.T) {
	e := NewReputationEngine().WithReputationFetcher(fakeReputationFetcher{score: 10})
	fs, _ := e.Run(context.Background(), Target{URL: "https://scan-target.com"}, Policy{})
	for _, f := range fs {
		if f.Type == "intel_threat_score" {
			t.Fatalf("低情报评分不应产出 intel_threat_score: %v", f)
		}
	}
}

func TestReputationEngineRandomDomain(t *testing.T) {
	e := NewReputationEngine()
	fs, _ := e.Run(context.Background(), Target{URL: "https://kz9xq2m4v7.top"}, Policy{})
	var hasEntropy bool
	for _, f := range fs {
		if f.Type == "suspicious_domain_entropy" {
			hasEntropy = true
			if f.Severity != SeverityMedium {
				t.Fatalf("severity = %s, want medium", f.Severity)
			}
		}
	}
	if !hasEntropy {
		t.Fatalf("高熵域名应产出 suspicious_domain_entropy, got %d", len(fs))
	}
}

func TestReputationEngineDirectIP(t *testing.T) {
	e := NewReputationEngine()
	fs, _ := e.Run(context.Background(), Target{URL: "http://203.0.113.10"}, Policy{})
	var hasDirect bool
	for _, f := range fs {
		if f.Type == "direct_ip_host" {
			hasDirect = true
			if f.Severity != SeverityInfo {
				t.Fatalf("severity = %s, want info", f.Severity)
			}
		}
	}
	if !hasDirect {
		t.Fatalf("IP 直连应产出 direct_ip_host finding, got %d", len(fs))
	}
}

func TestDomainEntropy(t *testing.T) {
	if domainEntropy("example") >= 0.6 {
		t.Fatalf("普通域名熵应低于阈值: %v", domainEntropy("example"))
	}
	if domainEntropy("kz9xq2m4v7") < 0.6 {
		t.Fatalf("随机域名熵应高于阈值: %v", domainEntropy("kz9xq2m4v7"))
	}
}

