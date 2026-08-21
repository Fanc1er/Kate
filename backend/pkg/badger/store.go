// Package badger 封装 BadgerDB 元数据缓存与去重存储。
package badger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	badger "github.com/dgraph-io/badger/v3"
)

// Store BadgerDB 封装，Key 前缀约定见 design「BadgerDB 存储设计」。
type Store struct {
	db *badger.DB
}

// Open 打开（或创建）BadgerDB。
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭底层库。
func (s *Store) Close() error {
	return s.db.Close()
}

// Set 写入 key-value。
func (s *Store) Set(key, value string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

// SetTTL 写入带 TTL 的 key-value。
func (s *Store) SetTTL(key, value string, ttlSeconds int64) error {
	return s.db.Update(func(txn *badger.Txn) error {
		e := &badger.Entry{Key: []byte(key), Value: []byte(value)}
		if ttlSeconds > 0 {
			e.ExpiresAt = uint64(time.Now().Unix() + ttlSeconds)
		}
		return txn.SetEntry(e)
	})
}

// Get 读取 key，不存在返回 ok=false。
func (s *Store) Get(key string) (string, bool, error) {
	var val []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return "", false, err
	}
	if val == nil {
		return "", false, nil
	}
	return string(val), true, nil
}

// Has 判断 key 是否存在。
func (s *Store) Has(key string) (bool, error) {
	var found bool
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	return found, err
}

// Delete 删除 key。
func (s *Store) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// ScanPrefix 按前缀遍历所有 key-value。
func (s *Store) ScanPrefix(prefix string) ([]KeyValue, error) {
	var out []KeyValue
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		pk := []byte(prefix)
		for it.Seek(pk); it.ValidForPrefix(pk); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			out = append(out, KeyValue{Key: string(item.Key()), Value: string(val)})
		}
		return nil
	})
	return out, err
}

// KeyValue 前缀扫描结果项。
type KeyValue struct {
	Key   string
	Value string
}

// DBPath 返回底层 db 目录（供测试/清理使用）。
func (s *Store) DBPath() string {
	return s.db.Opts().Dir
}

// DataDir 返回数据目录路径。
func DataDir(base string) string {
	return filepath.Join(base, "badger")
}
