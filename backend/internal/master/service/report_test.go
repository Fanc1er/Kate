package service

import (
	"bytes"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/db"
)

func newReportTestSvc(t *testing.T) (*ReportService, *gorm.DB) {
	t.Helper()
	gdb, err := db.Init(t.TempDir() + "/report.db")
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	now := time.Now()
	gdb.Create(&models.Vulnerability{
		OrgID: 10, AssetID: 1, CVEID: "CVE-2026-0001", EngineName: "vuln_scan",
		Severity: "critical", Status: "open", Title: "SQL Injection",
		Description: "parameter injection in login", FirstSeenAt: now, LastSeenAt: now,
	})
	gdb.Create(&models.Vulnerability{
		OrgID: 10, AssetID: 2, CVEID: "CVE-2026-0002", EngineName: "vuln_scan",
		Severity: "low", Status: "open", Title: "Info Leak",
		Description: "version banner disclosure", FirstSeenAt: now, LastSeenAt: now,
	})
	// 另一组织数据，验证隔离。
	gdb.Create(&models.Vulnerability{
		OrgID: 20, AssetID: 3, CVEID: "CVE-2026-0003", EngineName: "vuln_scan",
		Severity: "high", Status: "open", Title: "Other Org", FirstSeenAt: now, LastSeenAt: now,
	})
	return NewReportService(gdb), gdb
}

func TestReportExportExcel(t *testing.T) {
	svc, _ := newReportTestSvc(t)
	data, filename, err := svc.Export(10, ExportParams{Format: "excel"})
	if err != nil {
		t.Fatalf("Export excel: %v", err)
	}
	if filename != "vuln_list.xlsx" {
		t.Fatalf("filename = %s", filename)
	}
	// xlsx = zip，校验魔数 PK。
	if len(data) < 2 || !bytes.HasPrefix(data, []byte("PK")) {
		t.Fatalf("xlsx 应以 PK 开头, got %d bytes", len(data))
	}
}

func TestReportExportPDF(t *testing.T) {
	svc, _ := newReportTestSvc(t)
	data, filename, err := svc.Export(10, ExportParams{Format: "pdf"})
	if err != nil {
		t.Fatalf("Export pdf: %v", err)
	}
	if filename != "vuln_report.pdf" {
		t.Fatalf("filename = %s", filename)
	}
	// PDF 魔数 %PDF。
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("pdf 应以 %%PDF 开头, got %d bytes", len(data))
	}
}

func TestReportExportOrgIsolation(t *testing.T) {
	svc, _ := newReportTestSvc(t)
	// org=20 只有 1 条。
	data, _, err := svc.Export(20, ExportParams{Format: "excel"})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("PK")) {
		t.Fatal("应为 xlsx")
	}
	// org=10 有 2 条（severity 过滤 critical → 1 条）。
	_, _, err = svc.Export(10, ExportParams{Format: "excel", Severity: "critical"})
	if err != nil {
		t.Fatalf("Export filtered: %v", err)
	}
}

func TestReportExportInvalidFormat(t *testing.T) {
	svc, _ := newReportTestSvc(t)
	_, _, err := svc.Export(10, ExportParams{Format: "csv"})
	if err == nil {
		t.Fatal("非法 format 应报错")
	}
}
