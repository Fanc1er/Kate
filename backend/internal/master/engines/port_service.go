package engines

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const NamePortService = "port_service"

// PortServiceEngine 端口服务监测引擎：TCP Connect 检测常见端口暴露。
// 非特权环境自动降级 TCP Connect，支持策略并发控制。
type PortServiceEngine struct{}

// NewPortServiceEngine 构造端口服务监测引擎。
func NewPortServiceEngine() *PortServiceEngine { return &PortServiceEngine{} }

// Name 返回引擎名。
func (e *PortServiceEngine) Name() string { return NamePortService }

// Enabled 策略开关：port_service 未显式关闭即启用。
func (e *PortServiceEngine) Enabled(p Policy) bool {
	if p.Enabled == nil {
		return true
	}
	en, ok := p.Enabled[NamePortService]
	if !ok {
		return true
	}
	return en
}

// CommonPorts 常见监测端口列表。
var CommonPorts = []int{21, 22, 23, 25, 53, 80, 110, 135, 139, 143,
	443, 445, 993, 995, 1433, 1521, 3306, 3389, 5432, 5900,
	6379, 8080, 8443, 27017, 9200, 11211}

// Run 执行端口服务检测：对目标主机扫描 CommonPorts 列表与显式指定端口。
func (e *PortServiceEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	host, explicitPort, err := parseHostPort(target.URL)
	if err != nil {
		return nil, err
	}

	// 合并待检测端口：显式端口 + CommonPorts 列表。
	portSet := map[int]bool{}
	if explicitPort > 0 {
		portSet[explicitPort] = true
	}
	for _, port := range CommonPorts {
		portSet[port] = true
	}

	// 并发控制。
	conc := p.Concurrency
	if conc <= 0 {
		conc = 8
	}
	sem := make(chan struct{}, conc)
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for port := range portSet {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()
			addr := net.JoinHostPort(host, strconv.Itoa(port))
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return
			}
			_ = conn.Close()
			service := detectService(port)
			mu.Lock()
			findings = append(findings, Finding{
				Type:        "port_exposed",
				Severity:    portSeverity(port),
				Title:       fmt.Sprintf("端口 %d 开放（%s）", port, service),
				Description: fmt.Sprintf("目标 %s 的端口 %d 处于开放状态，服务类型：%s", host, port, service),
				URL:         target.URL,
				Confidence:  0.9,
				Extra: map[string]any{
					"port":      port,
					"service":   service,
					"scan_mode": "connect",
				},
			})
			mu.Unlock()
		}(port)
	}
	wg.Wait()

	if ctx.Err() != nil && len(findings) == 0 {
		return findings, ctx.Err()
	}
	return findings, nil
}

// parseHostPort 从目标字符串解析主机与显式端口（无端口时为 0）。
func parseHostPort(raw string) (host string, port int, err error) {
	// 先作为 URL 解析（http://host:port/path 或 https://host）。
	if u, perr := url.Parse(raw); perr == nil && u.Hostname() != "" {
		host = u.Hostname()
		if p := u.Port(); p != "" {
			port, _ = strconv.Atoi(p)
		}
		return host, port, nil
	}
	// 再尝试 host:port。
	if h, p, serr := net.SplitHostPort(raw); serr == nil {
		host = h
		port, _ = strconv.Atoi(p)
		return host, port, nil
	}
	// 裸主机名。
	if h := strings.TrimSpace(raw); h != "" {
		return h, 0, nil
	}
	return "", 0, fmt.Errorf("无效目标: %s", raw)
}

// detectService 根据端口号推断服务类型。
func detectService(port int) string {
	services := map[int]string{
		21:   "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP",
		53:   "DNS", 80: "HTTP", 110: "POP3", 135: "RPC",
		139:  "NetBIOS", 143: "IMAP", 443: "HTTPS", 445: "SMB",
		993:  "IMAPS", 995: "POP3S", 1433: "MSSQL", 1521: "Oracle",
		3306: "MySQL", 3389: "RDP", 5432: "PostgreSQL", 5900: "VNC",
		6379: "Redis", 8080: "HTTP-Proxy", 8443: "HTTPS-Alt",
		27017: "MongoDB", 9200: "Elasticsearch", 11211: "Memcached",
	}
	if svc, ok := services[port]; ok {
		return svc
	}
	return "unknown"
}

// portSeverity 根据端口返回严重度。
func portSeverity(port int) string {
	highRisk := map[int]bool{
		21: true, 22: true, 23: true, 135: true, 139: true, 445: true,
		3389: true, 5900: true, 6379: true, 27017: true,
		9200: true, 11211: true,
	}
	if highRisk[port] {
		return SeverityHigh
	}
	return SeverityLow
}
