// Package license 实现离线授权文件的签发、验证与机器特征绑定。
package license

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/license/keys"
)

// Status 授权有效性状态。
type Status string

const (
	StatusValid           Status = "valid"
	StatusMissing         Status = "missing"
	StatusInvalid         Status = "invalid"
	StatusNotYetActive    Status = "not_yet_active"
	StatusExpired         Status = "expired"
	StatusMachineMismatch Status = "machine_mismatch"
)

const (
	fileFormat  = "cinsight-license"
	fileVersion = 1
	cipherName  = "AES-256-GCM"
	saltSize    = 16
)

// Payload 授权载荷（解密后的明文）。
type Payload struct {
	MachineHash string         `json:"machine_hash"`
	IssuedAt    time.Time      `json:"issued_at"`
	NotBefore   time.Time      `json:"not_before"`
	NotAfter    time.Time      `json:"not_after"`
	MaxAssets   int            `json:"max_assets"`
	MaxWorkers  int            `json:"max_workers"`
	Customer    string         `json:"customer"`
	Features    map[string]any `json:"features"`
}

type fileEnvelope struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	Cipher    string `json:"cipher"`
	Nonce     string `json:"nonce"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// IssueOptions 签发参数。
type IssueOptions struct {
	NotBefore time.Time
	NotAfter  time.Time
	MaxAssets int
	MaxWorkers int
	Customer  string
}

// Issue 使用厂商私钥签发授权文件，返回 .lic 文件内容。
func Issue(machineHash string, opts IssueOptions, priv *rsa.PrivateKey) ([]byte, error) {
	payload := Payload{
		MachineHash: machineHash,
		IssuedAt:    time.Now().UTC(),
		NotBefore:   opts.NotBefore,
		NotAfter:    opts.NotAfter,
		MaxAssets:   opts.MaxAssets,
		MaxWorkers:  opts.MaxWorkers,
		Customer:    opts.Customer,
		Features:    map[string]any{},
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	key := deriveAESKey()
	ciphertext, nonce, err := encryptPayload(plaintext, key)
	if err != nil {
		return nil, err
	}
	sig, err := signPayload(ciphertext, priv)
	if err != nil {
		return nil, err
	}
	env := fileEnvelope{
		Format:    fileFormat,
		Version:   fileVersion,
		Cipher:    cipherName,
		Nonce:     base64.StdEncoding.EncodeToString(nonce),
		Payload:   base64.StdEncoding.EncodeToString(ciphertext),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	return json.Marshal(env)
}

// Manager 管理授权文件状态，是授权判定的唯一真相源。
type Manager struct {
	mu        sync.RWMutex
	filePath  string
	saltPath  string
	publicKey *rsa.PublicKey
	aesKey    []byte
	status    Status
	payload   *Payload
}

// NewManager 构造 Manager，加载验签公钥。
func NewManager(filePath, saltPath string) (*Manager, error) {
	pub, err := keys.LoadPublicKey()
	if err != nil {
		return nil, err
	}
	return &Manager{
		filePath:  filePath,
		saltPath:  saltPath,
		publicKey: pub,
		aesKey:    deriveAESKey(),
		status:    StatusMissing,
	}, nil
}

// Load 启动时加载授权文件到内存。
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		m.setStatus(StatusMissing, nil)
		return nil
	}
	payload, status := m.verify(data)
	m.setStatus(status, payload)
	return nil
}

// Import 校验并导入授权文件，返回导入后的状态。
func (m *Manager) Import(data []byte) Status {
	payload, status := m.verify(data)
	if status != StatusValid && status != StatusNotYetActive {
		m.setStatus(status, nil)
		return status
	}
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0o755); err != nil {
		m.setStatus(StatusInvalid, nil)
		return StatusInvalid
	}
	if err := os.WriteFile(m.filePath, data, 0o644); err != nil {
		m.setStatus(StatusInvalid, nil)
		return StatusInvalid
	}
	m.setStatus(status, payload)
	return status
}

// Status 返回当前授权状态。
func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Check 运行时实时校验授权有效性。
// 签名与机器匹配在 Load/Import 时判定并缓存；运行中随时间变化的仅 not_before/not_after，
// 故仅对时间维度实时重判，其余维度（missing/invalid/machine_mismatch/expired）返回缓存状态。
func (m *Manager) Check() Status {
	m.mu.RLock()
	status := m.status
	p := m.payload
	m.mu.RUnlock()

	switch status {
	case StatusValid, StatusNotYetActive:
		if p == nil {
			return StatusMissing
		}
		now := time.Now()
		if now.Before(p.NotBefore) {
			return StatusNotYetActive
		}
		if now.After(p.NotAfter) {
			return StatusExpired
		}
		return StatusValid
	default:
		return status
	}
}

// Payload 返回当前授权载荷（无授权时为 nil）。
func (m *Manager) Payload() *Payload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.payload
}

// MaxAssets 返回授权资产上限（0 表示不限制）。
func (m *Manager) MaxAssets() int {
	p := m.Payload()
	if p == nil {
		return 0
	}
	return p.MaxAssets
}

// MaxWorkers 返回授权 Worker 上限（0 表示不限制）。
func (m *Manager) MaxWorkers() int {
	p := m.Payload()
	if p == nil {
		return 0
	}
	return p.MaxWorkers
}

// DaysRemaining 返回距到期剩余天数（负数表示已过期）。
func (m *Manager) DaysRemaining() int {
	p := m.Payload()
	if p == nil {
		return 0
	}
	d := time.Until(p.NotAfter)
	return int(d.Hours() / 24)
}

// NotBefore 返回延迟激活时间。
func (m *Manager) NotBefore() time.Time {
	p := m.Payload()
	if p == nil {
		return time.Time{}
	}
	return p.NotBefore
}

// NotAfter 返回授权截止时间。
func (m *Manager) NotAfter() time.Time {
	p := m.Payload()
	if p == nil {
		return time.Time{}
	}
	return p.NotAfter
}

// MachineCode 生成机器码（SHA-256 机器特征 + 盐值）与特征来源描述。
func (m *Manager) MachineCode() (code, source string, err error) {
	diskSerial, cpuID, mac, osVer := collectFingerprint()
	salt, err := m.loadOrCreateSalt()
	if err != nil {
		return "", "", err
	}
	combined := strings.Join([]string{diskSerial, cpuID, mac, osVer, hex.EncodeToString(salt)}, "|")
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:]), describeSource(diskSerial, cpuID, mac, osVer), nil
}

func (m *Manager) setStatus(s Status, p *Payload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = s
	m.payload = p
}

// verify 解析并校验授权文件，返回载荷与状态。
func (m *Manager) verify(data []byte) (*Payload, Status) {
	var env fileEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, StatusInvalid
	}
	if env.Format != fileFormat || env.Version != fileVersion {
		return nil, StatusInvalid
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return nil, StatusInvalid
	}
	ct, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		return nil, StatusInvalid
	}
	if err := verifyPayload(ct, sig, m.publicKey); err != nil {
		return nil, StatusInvalid
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, StatusInvalid
	}
	plaintext, err := decryptPayload(ct, nonce, m.aesKey)
	if err != nil {
		return nil, StatusInvalid
	}
	var p Payload
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, StatusInvalid
	}
	current, err := m.currentMachineHash()
	if err != nil {
		return &p, StatusInvalid
	}
	if current != p.MachineHash {
		return &p, StatusMachineMismatch
	}
	now := time.Now()
	if now.Before(p.NotBefore) {
		return &p, StatusNotYetActive
	}
	if now.After(p.NotAfter) {
		return &p, StatusExpired
	}
	return &p, StatusValid
}

// loadOrCreateSalt 读取或创建持久化盐值。
func (m *Manager) loadOrCreateSalt() ([]byte, error) {
	if b, err := os.ReadFile(m.saltPath); err == nil && len(b) > 0 {
		return b, nil
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(m.saltPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(m.saltPath, salt, 0o600); err != nil {
		return nil, err
	}
	return salt, nil
}

// currentMachineHash 计算当前机器哈希。
func (m *Manager) currentMachineHash() (string, error) {
	diskSerial, cpuID, mac, osVer := collectFingerprint()
	salt, err := m.loadOrCreateSalt()
	if err != nil {
		return "", err
	}
	combined := strings.Join([]string{diskSerial, cpuID, mac, osVer, hex.EncodeToString(salt)}, "|")
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:]), nil
}

func describeSource(diskSerial, cpuID, mac, osVer string) string {
	parts := make([]string, 0, 4)
	if diskSerial != "" {
		parts = append(parts, "disk_serial")
	}
	if cpuID != "" {
		parts = append(parts, "cpu_id")
	}
	if mac != "" {
		parts = append(parts, "mac")
	}
	if osVer != "" {
		parts = append(parts, "os_version")
	}
	return strings.Join(parts, ",")
}
