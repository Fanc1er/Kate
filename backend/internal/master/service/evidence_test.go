package service

import (
	"path/filepath"
	"testing"

	"github.com/Fanc1er/Kate/backend/pkg/db"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/storage"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// newEvidenceTestSvc 构造测试用 EvidenceService（临时 sqlite + 临时证据目录）。
func newEvidenceTestSvc(t *testing.T) *EvidenceService {
	t.Helper()
	gdb, err := db.Init(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return NewEvidenceService(gdb, store, 30)
}

func TestChunkUploadCollectAndVerify(t *testing.T) {
	svc := newEvidenceTestSvc(t)
	data := []byte("chunk-a-data|chunk-b-data|chunk-c-data")
	sha := utils.SHA256Hex(data)
	total := 3
	for i, part := range []string{"chunk-a-data", "|chunk-b-data", "|chunk-c-data"} {
		id, complete, err := svc.ChunkUpload("up-1", "html", total, i, []byte(part), sha)
		if err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
		if i < total-1 && complete {
			t.Fatalf("chunk %d 不应提前 complete", i)
		}
		if i == total-1 && (!complete || id == 0) {
			t.Fatalf("末片应 complete 且返回 evidence_id, got complete=%v id=%d", complete, id)
		}
	}
	ev, err := svc.Get(1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _, err := svc.Read(ev.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("合并内容不一致: got %q want %q", got, data)
	}
}

func TestChunkUploadTamperedSHA256(t *testing.T) {
	svc := newEvidenceTestSvc(t)
	data := []byte("real-content")
	wrong := utils.SHA256Hex([]byte("tampered"))
	_, complete, err := svc.ChunkUpload("up-2", "html", 2, 0, data, wrong)
	if err != nil {
		t.Fatalf("首片: %v", err)
	}
	if complete {
		t.Fatal("单片不应 complete")
	}
	_, _, err = svc.ChunkUpload("up-2", "html", 2, 1, data, wrong)
	if err == nil {
		t.Fatal("SHA-256 不一致应报错")
	}
	if errs.CodeOf(err) != errs.CodeEvidenceTampered {
		t.Fatalf("应返回 EVIDENCE_TAMPERED, got %v", err)
	}
}

func TestCreateFromBytesDedup(t *testing.T) {
	svc := newEvidenceTestSvc(t)
	id1, err := svc.CreateFromBytes("html", []byte("same-content"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	id2, err := svc.CreateFromBytes("html", []byte("same-content"))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("MD5 去重应返回相同 evidence_id, got %d vs %d", id1, id2)
	}
}
