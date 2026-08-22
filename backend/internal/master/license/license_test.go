package license

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var (
	testKeyOnce sync.Once
	testPriv    *rsa.PrivateKey
	testPubPEM  string
)

// sharedKeys 生成一次开发测试用 RSA-2048 密钥对，避免每个用例重复生成。
func sharedKeys() (*rsa.PrivateKey, string) {
	testKeyOnce.Do(func() {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			panic(err)
		}
		testPriv = priv
		testPubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	})
	return testPriv, testPubPEM
}

// newTestManager 用临时密钥对公钥构造 Manager（独立临时数据目录）。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	_, pubPEM := sharedKeys()
	t.Setenv("CINSIGHT_LICENSE_PUBLIC_KEY", pubPEM)
	dir := t.TempDir()
	m, err := NewManager(filepath.Join(dir, "license.lic"), filepath.Join(dir, "machine.salt"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return m
}

// issueFor 用当前机器的机器码签发指定时间范围的授权文件。
func issueFor(t *testing.T, m *Manager, notBefore, notAfter time.Time) []byte {
	t.Helper()
	priv, _ := sharedKeys()
	code, _, err := m.MachineCode()
	if err != nil {
		t.Fatalf("machine code: %v", err)
	}
	data, err := Issue(code, IssueOptions{NotBefore: notBefore, NotAfter: notAfter, MaxAssets: 100, MaxWorkers: 5}, priv)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return data
}

func TestMachineCodeStable(t *testing.T) {
	m := newTestManager(t)
	code1, _, err := m.MachineCode()
	if err != nil {
		t.Fatalf("machine code: %v", err)
	}
	code2, _, err := m.MachineCode()
	if err != nil {
		t.Fatalf("machine code: %v", err)
	}
	if code1 == "" || code1 != code2 {
		t.Fatalf("machine code unstable: %q != %q", code1, code2)
	}
}

func TestImportValid(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	data := issueFor(t, m, now.Add(-time.Hour), now.Add(24*time.Hour))
	if st := m.Import(data); st != StatusValid {
		t.Fatalf("import status = %q, want valid", st)
	}
	if m.Status() != StatusValid {
		t.Fatalf("status = %q, want valid", m.Status())
	}
	if m.MaxAssets() != 100 || m.MaxWorkers() != 5 {
		t.Fatalf("quota = (%d,%d), want (100,5)", m.MaxAssets(), m.MaxWorkers())
	}
}

func TestImportTampered(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	data := issueFor(t, m, now.Add(-time.Hour), now.Add(time.Hour))
	var env fileEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env.Signature = "AAAAAAAA"
	tampered, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if st := m.Import(tampered); st != StatusInvalid {
		t.Fatalf("import status = %q, want invalid", st)
	}
}

func TestImportMachineMismatch(t *testing.T) {
	m := newTestManager(t)
	_, _, _ = m.MachineCode()
	priv, _ := sharedKeys()
	now := time.Now()
	data, err := Issue("deadbeef", IssueOptions{NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}, priv)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if st := m.Import(data); st != StatusMachineMismatch {
		t.Fatalf("import status = %q, want machine_mismatch", st)
	}
}

func TestImportNotYetActive(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	data := issueFor(t, m, now.Add(24*time.Hour), now.Add(48*time.Hour))
	if st := m.Import(data); st != StatusNotYetActive {
		t.Fatalf("import status = %q, want not_yet_active", st)
	}
	if m.Status() != StatusNotYetActive {
		t.Fatalf("status = %q, want not_yet_active", m.Status())
	}
}

func TestImportExpired(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	data := issueFor(t, m, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	if st := m.Import(data); st != StatusExpired {
		t.Fatalf("import status = %q, want expired", st)
	}
}

func TestLoadNoFile(t *testing.T) {
	m := newTestManager(t)
	if err := m.Load(); err != nil {
		t.Fatalf("load no file: %v", err)
	}
	if m.Status() != StatusMissing {
		t.Fatalf("status = %q, want missing", m.Status())
	}
	if m.Check() != StatusMissing {
		t.Fatalf("check = %q, want missing", m.Check())
	}
}

func TestLoadBadFile(t *testing.T) {
	m := newTestManager(t)
	if err := os.WriteFile(m.filePath, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := m.Load(); err != nil {
		t.Fatalf("load bad file: %v", err)
	}
	if m.Status() != StatusInvalid {
		t.Fatalf("status = %q, want invalid", m.Status())
	}
}

func TestCheckStateMachine(t *testing.T) {
	m := newTestManager(t)

	if st := m.Check(); st != StatusMissing {
		t.Fatalf("check missing = %q", st)
	}

	m.setStatus(StatusNotYetActive, &Payload{NotBefore: time.Now().Add(time.Hour), NotAfter: time.Now().Add(2 * time.Hour)})
	if st := m.Check(); st != StatusNotYetActive {
		t.Fatalf("check not_yet_active = %q", st)
	}

	m.setStatus(StatusValid, &Payload{NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)})
	if st := m.Check(); st != StatusValid {
		t.Fatalf("check valid = %q", st)
	}

	m.setStatus(StatusValid, &Payload{NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour)})
	if st := m.Check(); st != StatusExpired {
		t.Fatalf("check runtime expired = %q", st)
	}

	m.setStatus(StatusMachineMismatch, nil)
	if st := m.Check(); st != StatusMachineMismatch {
		t.Fatalf("check machine_mismatch = %q", st)
	}
}
