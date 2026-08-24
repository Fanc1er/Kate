package engines

import (
	"context"
	"fmt"
	"strings"
)

const NameIntelligence = "intelligence"

// IntelligenceEngine 安全情报引擎骨架：CVE/CNVD/CNNVD 订阅与资产匹配。
// 完整实现需支持定时拉取情报源、资产匹配、受影响资产数计算。
type IntelligenceEngine struct{}

// NewIntelligenceEngine 构造安全情报引擎。
func NewIntelligenceEngine() *IntelligenceEngine { return &IntelligenceEngine{} }

// Name 返回引擎名。
func (e *IntelligenceEngine) Name() string { return NameIntelligence }

// Enabled 策略开关：intelligence 未显式关闭即启用。
func (e *IntelligenceEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NameIntelligence]
	if !ok {
		return true
	}
	return en
}

// Run 执行安全情报检测（骨架：基于已知 CVE 匹配）。
func (e *IntelligenceEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	// 骨架实现：检查目标 URL 是否命中已知 CVE 情报
	// 完整实现需查询 intel_items 表并按资产匹配
	_ = target
	_ = p
	// TODO: 接入 CVE/CNVD/CNNVD 订阅数据匹配逻辑
	return findings, nil
}

// MatchCVE 检查目标是否命中指定 CVE。
func MatchCVE(targetURL, cveID, description string) Finding {
	return Finding{
		Type:        "cve_match",
		Severity:    SeverityHigh,
		Title:       fmt.Sprintf("命中 CVE 情报：%s", cveID),
		Description: fmt.Sprintf("资产 %s 命中安全情报 %s：%s", targetURL, cveID, description),
		URL:         targetURL,
		Confidence:  0.9,
		Extra: map[string]any{
			"cve_id":      cveID,
			"description": description,
		},
	}
}

// ExtractVersion 从目标 URL 或 User-Agent 中提取版本号（骨架）。
func ExtractVersion(raw string) string {
	parts := strings.Split(raw, "/")
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if len(last) > 0 && last[0] >= '0' && last[0] <= '9' {
			return last
		}
	}
	return ""
}
