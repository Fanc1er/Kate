package engines

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const NamePortService = "port_service"

// PortServiceEngine 端口服务监测引擎骨架：TCP 连接检测常见端口暴露。
// 完整实现需支持 TCP SYN 扫描与非特权环境降级 TCP Connect。
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

// Run 执行端口服务检测。
func (e *PortServiceEngine) Run(ctx context.Context, target Target, p Policy) ([]Finding, error) {
	var findings []Finding
	host, portStr, err := net.SplitHostPort(target.URL)
	if err != nil {
		// 尝试从 URL 解析主机
		host = strings.TrimPrefix(target.URL, "http://")
		host = strings.TrimPrefix(host, "https://")
		if idx := strings.Index(host, "/"); idx >= 0 {
			host = host[:idx]
		}
		if idx := strings.Index(host, ":"); idx >= 0 {
			host = host[:idx]
		}
		portStr = "80"
	}

	// 尝试从 URL 获取端口
	if portStr == "" || portStr == "80" || portStr == "443" {
		portStr = "80"
	}

	// 检测常见高危端口（骨架：仅检测目标自身端口）
	// 完整实现需对目标 IP 的 CommonPorts 列表进行扫描
	port, _ := fmt.Sscanf(portStr, "%d", new(int))
	if port > 0 {
		addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			_ = conn.Close()
			// 端口开放，检测服务指纹
			service := detectService(port)
			findings = append(findings, Finding{
				Type:        "port_exposed",
				Severity:    portSeverity(port),
				Title:       fmt.Sprintf("端口 %d 开放（%s）", port, service),
				Description: fmt.Sprintf("目标 %s 的端口 %d 处于开放状态，服务类型：%s", host, port, service),
				URL:         target.URL,
				Confidence:  0.9,
				Extra: map[string]any{
					"port":     port,
					"service":  service,
					"scan_mode": "connect", // 非特权环境降级
				},
			})
		}
	}

	return findings, nil
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
		21: true, 23: true, 135: true, 139: true, 445: true,
		3389: true, 5900: true, 6379: true, 27017: true,
		9200: true, 11211: true,
	}
	if highRisk[port] {
		return SeverityHigh
	}
	return SeverityLow
}
