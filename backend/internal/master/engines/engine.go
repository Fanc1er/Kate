// Package engines 定义 10 大扫描引擎的统一契约与注册表。
// 引擎以可插拔方式注册，策略引擎开关控制启用，worker 执行时按契约调用。
package engines

import (
	"context"
)

// Severity 级别常量。
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// Evidence 引擎输出证据（内联快照或文件引用）。
type Evidence struct {
	Kind    string // html/har/screenshot/req/resp
	Content string // base64 内容（内联）
	SHA256  string // 内容 SHA-256
	FileID  int64  // 已入库证据 ID（≥1MB 分片上传后回填）
}

// Finding 引擎判定结果。
type Finding struct {
	Type        string    // 引擎内部类型，如 http_error / slow_response
	Severity    string    // critical/high/medium/low/info
	Title       string
	Description string
	URL         string
	LineNo      int       // 触发点行号，无代码定位时为 0
	Confidence  float64   // 0~1 引擎判定可信度
	Evidence    []Evidence
	Extra       map[string]any
}

// Target 引擎扫描目标。
type Target struct {
	ID  int64
	URL string
}

// Policy 引擎执行策略（由 scan_policies 解析下发）。
type Policy struct {
	Enabled       map[string]bool // engine_name -> enabled
	Concurrency   int
	Timeout       int // 分钟
	RateLimit     int
	ScanDepth     int
	FailCount     int
	SlowThresholdMS int
}

// Engine 引擎统一契约。
type Engine interface {
	Name() string
	Enabled(policy Policy) bool
	Run(ctx context.Context, target Target, policy Policy) ([]Finding, error)
}

// Registry 引擎注册表。
type Registry struct {
	engines map[string]Engine
}

// NewRegistry 构造空注册表。
func NewRegistry() *Registry {
	return &Registry{engines: map[string]Engine{}}
}

// Register 注册引擎（同名覆盖）。
func (r *Registry) Register(e Engine) {
	r.engines[e.Name()] = e
}

// Get 按名称取引擎，不存在返回 nil。
func (r *Registry) Get(name string) Engine {
	return r.engines[name]
}

// Names 返回全部已注册引擎名。
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.engines))
	for n := range r.engines {
		out = append(out, n)
	}
	return out
}

// EnabledNames 返回策略下启用的引擎名列表。
func (r *Registry) EnabledNames(p Policy) []string {
	var out []string
	for n, e := range r.engines {
		if e.Enabled(p) {
			out = append(out, n)
		}
	}
	return out
}
