package license

import (
	"net"
	"os"
	"path/filepath"
	"strings"
)

// collectFingerprint 采集机器四类特征：硬盘序列号、CPU ID、MAC、OS 版本。
func collectFingerprint() (diskSerial, cpuID, mac, osVer string) {
	return readDiskSerial(), readCPUID(), readMAC(), readOSVersion()
}

// readDiskSerial 读取首个物理磁盘序列号。
func readDiskSerial() string {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if isVirtualBlock(name) {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/sys/block", name, "device/serial"))
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(b)); s != "" {
			return s
		}
	}
	return ""
}

func isVirtualBlock(name string) bool {
	return strings.HasPrefix(name, "loop") ||
		strings.HasPrefix(name, "ram") ||
		strings.HasPrefix(name, "dm-") ||
		strings.HasPrefix(name, "sr") ||
		strings.HasPrefix(name, "fd")
}

// readCPUID 组合 /proc/cpuinfo 中的处理器标识字段（Linux 通常不暴露序列号）。
func readCPUID() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	var vendor, model, stepping, physical string
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "vendor_id":
			if vendor == "" {
				vendor = val
			}
		case "model name":
			if model == "" {
				model = val
			}
		case "stepping":
			if stepping == "" {
				stepping = val
			}
		case "physical id":
			if physical == "" {
				physical = val
			}
		}
	}
	return strings.Join([]string{vendor, model, stepping, physical}, "|")
}

// readMAC 读取首个非回环、已启用的网卡物理地址。
func readMAC() string {
	ifs, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifs {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		if len(i.HardwareAddr) > 0 {
			return i.HardwareAddr.String()
		}
	}
	return ""
}

// readOSVersion 读取操作系统版本与内核版本。
func readOSVersion() string {
	pretty := ""
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				pretty = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
	}
	kernel := ""
	if kb, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		kernel = strings.TrimSpace(string(kb))
	}
	return strings.Join([]string{pretty, kernel}, "|")
}
