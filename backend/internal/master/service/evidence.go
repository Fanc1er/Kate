package service

import (
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/storage"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// chunkBuffer 分片上传暂存（按 upload_id 聚合）。
type chunkBuffer struct {
	orgID       int64
	kind        string
	total       int
	sha256      string
	chunks      map[int][]byte
	lastSeen    time.Time
}

// EvidenceService 证据服务：gzip 落盘 + MD5 去重 + SHA-256 防篡改 + 截图上传 + 分片上传。
type EvidenceService struct {
	DB    *gorm.DB
	Store *storage.Store
	TTLDays int
	mu    sync.Mutex
	bufs  map[string]*chunkBuffer
}

// NewEvidenceService 构造 EvidenceService。
func NewEvidenceService(db *gorm.DB, store *storage.Store, ttlDays int) *EvidenceService {
	return &EvidenceService{DB: db, Store: store, TTLDays: ttlDays, bufs: map[string]*chunkBuffer{}}
}

// ChunkUpload 分片上传：chunk_index=-1 表示收齐后合并校验落库。
// 返回 evidence_id（完成时）与 complete 标记。
func (s *EvidenceService) ChunkUpload(orgID int64, uploadID, kind string, total, index int, data []byte, sha256 string) (int64, bool, error) {
	if uploadID == "" || total <= 0 || index < 0 {
		return 0, false, errs.New(errs.CodeValidationFailed, "分片参数非法")
	}
	if len(data) > 8<<20 {
		return 0, false, errs.New(errs.CodeValidationFailed, "单分片不能超过 8MB")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// TTL 清理超过 10 分钟未收齐的暂存。
	now := time.Now()
	for id, b := range s.bufs {
		if now.Sub(b.lastSeen) > 10*time.Minute {
			delete(s.bufs, id)
		}
	}
	b, ok := s.bufs[uploadID]
	if !ok {
		b = &chunkBuffer{orgID: orgID, kind: kind, total: total, sha256: sha256, chunks: map[int][]byte{}}
		s.bufs[uploadID] = b
	}
	if b.total != total {
		return 0, false, errs.New(errs.CodeValidationFailed, "分片总数不一致")
	}
	b.chunks[index] = data
	b.lastSeen = now
	if len(b.chunks) < b.total {
		return 0, false, nil
	}
	// 收齐：按 index 合并 + SHA-256 校验。
	merged := make([]byte, 0, b.total*8<<10)
	idx := make([]int, 0, len(b.chunks))
	for i := range b.chunks {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		merged = append(merged, b.chunks[i]...)
	}
	if utils.SHA256Hex(merged) != b.sha256 {
		delete(s.bufs, uploadID)
		return 0, false, errs.New(errs.CodeEvidenceTampered, "分片合并后 SHA-256 不一致")
	}
	id, err := s.CreateFromBytes(orgID, b.kind, merged)
	delete(s.bufs, uploadID)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// CreateFromBytes 保存内联证据（gzip 落盘 + MD5 去重 + SHA-256 入库）。
func (s *EvidenceService) CreateFromBytes(orgID int64, kind string, data []byte) (int64, error) {
	// MD5 去重。
	md5Hex := utils.MD5Hex(string(data))
	var existing models.Evidence
	if err := s.DB.Where("org_id = ? AND md5 = ?", orgID, md5Hex).First(&existing).Error; err == nil {
		return existing.ID, nil
	}
	rel, size, sha256, err := s.Store.Save(data)
	if err != nil {
		return 0, err
	}
	ev := &models.Evidence{
		OrgID: orgID, MD5: md5Hex, SHA256: sha256, FilePath: rel,
		MimeType: mimeFor(kind), Size: size,
	}
	if err := s.DB.Create(ev).Error; err != nil {
		return 0, err
	}
	ef := &models.EvidenceFile{
		EvidenceID: ev.ID, OrgID: orgID, Kind: kind, FilePath: rel,
		MD5: md5Hex, SHA256: sha256, Size: size, MimeType: mimeFor(kind),
		ExpiresAt: time.Now().AddDate(0, 0, s.TTLDays),
	}
	if err := s.DB.Create(ef).Error; err != nil {
		return 0, err
	}
	return ev.ID, nil
}

// Get 证据元数据。
func (s *EvidenceService) Get(orgID, id int64) (*models.Evidence, error) {
	var ev models.Evidence
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	return &ev, nil
}

// Files 证据链子文件列表。
func (s *EvidenceService) Files(orgID, id int64) ([]models.EvidenceFile, error) {
	var files []models.EvidenceFile
	if err := s.DB.Where("evidence_id = ? AND org_id = ?", id, orgID).Order("kind ASC").Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

// Read 读取证据并强制校验 SHA-256，不一致返回 EVIDENCE_TAMPERED。
func (s *EvidenceService) Read(orgID, id int64) ([]byte, *models.Evidence, error) {
	ev, err := s.Get(orgID, id)
	if err != nil {
		return nil, nil, err
	}
	data, err := s.Store.Read(ev.FilePath, ev.SHA256)
	if err != nil {
		if errors.Is(err, storage.ErrTampered) {
			return nil, ev, errs.New(errs.CodeEvidenceTampered, "")
		}
		return nil, ev, err
	}
	return data, ev, nil
}

// Download 下载证据文件（下载前同样校验 Hash）。
func (s *EvidenceService) Download(orgID, id int64, format string) ([]byte, string, error) {
	ev, err := s.Get(orgID, id)
	if err != nil {
		return nil, "", err
	}
	data, err := s.Store.Read(ev.FilePath, ev.SHA256)
	if err != nil {
		if errors.Is(err, storage.ErrTampered) {
			return nil, "", errs.New(errs.CodeEvidenceTampered, "")
		}
		return nil, "", err
	}
	if format == "har" || strings.Contains(ev.MimeType, "har") {
		return data, "evidence.har", nil
	}
	return data, "evidence.html", nil
}

// UploadScreenshot 截图上传：MIME 校验（png/jpeg/webp）+ 大小 ≤10MB + 防路径穿越。
func (s *EvidenceService) UploadScreenshot(orgID int64, kind string, header *multipart.FileHeader) (*models.Evidence, error) {
	if header == nil {
		return nil, errs.New(errs.CodeValidationFailed, "缺少文件")
	}
	if header.Size > 10<<20 {
		return nil, errs.New(errs.CodeValidationFailed, "截图不能超过 10MB")
	}
	mime := header.Header.Get("Content-Type")
	if !isAllowedImageMIME(mime) {
		return nil, errs.New(errs.CodeValidationFailed, "仅支持 png/jpeg/webp 格式")
	}
	f, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data := make([]byte, header.Size)
	if _, err := f.Read(data); err != nil {
		return nil, err
	}
	id, err := s.CreateFromBytes(orgID, "screenshot", data)
	if err != nil {
		return nil, err
	}
	return s.Get(orgID, id)
}

func isAllowedImageMIME(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/webp":
		return true
	}
	return false
}

func mimeFor(kind string) string {
	switch kind {
	case "html":
		return "text/html"
	case "har":
		return "application/json"
	case "screenshot":
		return "image/png"
	case "req", "resp":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// Cleanup 清理过期证据文件与孤儿文件。
func (s *EvidenceService) Cleanup() {
	var expired []models.EvidenceFile
	s.DB.Where("expires_at < ?", time.Now()).Find(&expired)
	for _, ef := range expired {
		_ = s.DB.Delete(&models.EvidenceFile{}, "id = ?", ef.ID)
		if err := os.Remove(filepath.Join(s.Store.EvidenceDir, ef.FilePath)); err == nil {
			// 无证据记录引用时回收空间。
		}
	}
}
