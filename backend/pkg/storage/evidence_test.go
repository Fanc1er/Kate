package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestSaveAndRead(t *testing.T) {
	s := newStore(t)
	// 使用大段可压缩内容，确保 gzip 后确实变小。
	data := []byte(`{"title":"test evidence","n":123,"payload":"` + string(make([]byte, 0)) + `"}`)
	for i := 0; i < 64; i++ {
		data = append(data, []byte(`{"a":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)...)
	}
	data = append(data, '}')

	rel, size, sha, err := s.Save(data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("size = %d, want %d", size, len(data))
	}
	if sha != utils.SHA256Hex(data) {
		t.Fatalf("sha256 mismatch")
	}

	got, err := s.Read(rel, sha)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("Read 内容不一致")
	}

	// 落盘文件应为 gzip 压缩。
	raw, err := s.ReadRaw(rel)
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if len(raw) >= len(data) {
		t.Fatal("证据应被 gzip 压缩")
	}
	// gzip magic.
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatal("文件头应为 gzip magic")
	}
}

func TestTamperDetection(t *testing.T) {
	s := newStore(t)
	data := []byte("original content")
	rel, _, sha, err := s.Save(data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 篡改文件内容。
	full := filepath.Join(s.EvidenceDir, rel)
	if err := os.WriteFile(full, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("write tampered: %v", err)
	}
	// 篡改后可能无法解压，也应报错。
	if _, err := s.Read(rel, sha); err == nil {
		t.Fatal("篡改后 Read 应报错")
	}

	// 直接篡改原始数据但用错误 SHA：解压成功但 hash 不匹配。
	rel2, _, _, err := s.Save(data)
	if err != nil {
		t.Fatalf("Save2: %v", err)
	}
	if _, err := s.Read(rel2, "0"+sha[1:]); err != ErrTampered {
		t.Fatalf("哈希不匹配应返回 ErrTampered，得到 %v", err)
	}
}

func TestPathTraversal(t *testing.T) {
	s := newStore(t)
	if _, err := s.Read("../secret", "sha"); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
	if _, err := s.Read("/etc/passwd", "sha"); err == nil {
		t.Fatal("绝对路径应被拒绝")
	}
}

func TestGzipRoundTrip(t *testing.T) {
	data := []byte("hello gzip " + string(make([]byte, 1000)))
	gz, err := Gzip(data)
	if err != nil {
		t.Fatalf("Gzip: %v", err)
	}
	back, err := Ungzip(gz)
	if err != nil {
		t.Fatalf("Ungzip: %v", err)
	}
	if string(back) != string(data) {
		t.Fatal("gzip 往返内容不一致")
	}
}
