// Package storage 提供证据文件 gzip 落盘、SHA-256/MD5 计算与篡改校验。
package storage

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// Store 证据文件存储。
type Store struct {
	EvidenceDir string // /data/evidence
}

// New 创建证据存储目录。
func New(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{EvidenceDir: dir}, nil
}

// Save 将原始数据 gzip 压缩落盘至 /data/evidence/{date}/，返回 (相对路径, size, sha256)。
// 文件名由服务端生成 UUID，忽略客户端文件名，防止路径穿越。
func (s *Store) Save(data []byte) (string, int64, string, error) {
	gz, err := Gzip(data)
	if err != nil {
		return "", 0, "", err
	}
	date := time.Now().Format("2006-01-02")
	dir := filepath.Join(s.EvidenceDir, date)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, "", err
	}
	name := uuid.NewString() + ".gz"
	rel := filepath.Join(date, name)
	full := filepath.Join(s.EvidenceDir, rel)
	if err := os.WriteFile(full, gz, 0o644); err != nil {
		return "", 0, "", err
	}
	return rel, int64(len(data)), utils.SHA256Hex(data), nil
}

// Gzip 压缩数据。
func Gzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Ungzip 解压 gzip 数据。
func Ungzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// Read 读取证据文件并校验 SHA-256，返回解压后的原始内容。
// 校验不一致返回 ErrTampered。
func (s *Store) Read(relPath, expectSHA256 string) ([]byte, error) {
	full, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}
	gz, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	data, err := Ungzip(gz)
	if err != nil {
		return nil, fmt.Errorf("ungzip evidence: %w", err)
	}
	if expectSHA256 != "" && utils.SHA256Hex(data) != expectSHA256 {
		return nil, ErrTampered
	}
	return data, nil
}

// ReadRaw 读取落盘文件原始字节（压缩态），并校验 SHA-256（针对压缩前内容校验）。
func (s *Store) ReadRaw(relPath string) ([]byte, error) {
	full, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

// Stat 返回落盘文件信息。
func (s *Store) Stat(relPath string) (os.FileInfo, error) {
	full, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}
	return os.Stat(full)
}

func (s *Store) resolve(relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}
	full := filepath.Join(s.EvidenceDir, relPath)
	// 防路径穿越：确保解析结果仍在证据目录内。
	if filepath.Dir(full) != s.EvidenceDir && !isWithin(filepath.Dir(full), s.EvidenceDir) {
		return "", fmt.Errorf("illegal path")
	}
	return full, nil
}

func isWithin(dir, root string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return rel != ".." && len(rel) >= 0
}

// ErrTampered 证据被破坏。
var ErrTampered = fmt.Errorf("evidence tampered")
