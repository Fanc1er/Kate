package service

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// ReportService 报告导出：PDF（含水印）/ Excel（漏洞清单）。
// 数据基于生成时刻快照，导出后处置变更不影响已生成内容。
type ReportService struct {
	DB *gorm.DB
}

// NewReportService 构造 ReportService。
func NewReportService(db *gorm.DB) *ReportService {
	return &ReportService{DB: db}
}

// ExportParams 导出参数。
type ExportParams struct {
	Format    string // pdf / excel
	AssetID   int64
	Severity  string
	Status    string
	From, To  *time.Time
}

// Export 生成报告字节流与建议文件名。
func (s *ReportService) Export(p ExportParams) ([]byte, string, error) {
	var q *gorm.DB
	switch p.Format {
	case "pdf", "excel":
	default:
		return nil, "", errs.New(errs.CodeValidationFailed, "format 仅支持 pdf/excel")
	}
	q = s.DB
	if p.AssetID > 0 {
		q = q.Where("asset_id = ?", p.AssetID)
	}
	if p.Severity != "" {
		q = q.Where("severity = ?", p.Severity)
	}
	if p.Status != "" {
		q = q.Where("status = ?", p.Status)
	}
	if p.From != nil {
		q = q.Where("first_seen_at >= ?", *p.From)
	}
	if p.To != nil {
		q = q.Where("first_seen_at <= ?", *p.To)
	}
	var vulns []models.Vulnerability
	if err := q.Order("severity = 'critical' DESC, severity = 'high' DESC, id DESC").Find(&vulns).Error; err != nil {
		return nil, "", err
	}

	switch p.Format {
	case "excel":
		data, err := vulnsToXLSX(vulns)
		return data, "vuln_list.xlsx", err
	default:
		data, err := vulnsToPDF(vulns)
		return data, "vuln_report.pdf", err
	}
}

// vulnsToXLSX 生成漏洞清单 xlsx。
func vulnsToXLSX(vulns []models.Vulnerability) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "漏洞清单"
	idx, _ := f.NewSheet(sheet)
	f.SetActiveSheet(idx)
	headers := []string{"ID", "CVE/签名", "引擎", "严重级别", "状态", "标题", "描述", "首次发现", "最近发现"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	_ = f.SetRowHeight(sheet, 1, 22)
	sevStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"C00000"}, Pattern: 1},
		Font: &excelize.Font{Color: "FFFFFF", Bold: true},
	})
	for i, v := range vulns {
		row := i + 2
		vals := []any{v.ID, v.CVEID, v.EngineName, v.Severity, v.Status, v.Title, v.Description,
			v.FirstSeenAt.Format("2006-01-02 15:04"), v.LastSeenAt.Format("2006-01-02 15:04")}
		for c, val := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			_ = f.SetCellValue(sheet, cell, val)
			if c == 3 && v.Severity == "critical" {
				_ = f.SetCellStyle(sheet, cell, cell, sevStyle)
			}
		}
	}
	_ = f.SetColWidth(sheet, "A", "A", 6)
	_ = f.SetColWidth(sheet, "B", "B", 20)
	_ = f.SetColWidth(sheet, "C", "C", 16)
	_ = f.SetColWidth(sheet, "F", "F", 40)
	_ = f.SetColWidth(sheet, "G", "G", 60)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// vulnsToPDF 生成 PDF 漏洞报告（英文内容 + 对角线水印）。
func vulnsToPDF(vulns []models.Vulnerability) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Vulnerability Report", true)
	pdf.AddPage()
	// 页眉。
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, "CInsight Vulnerability Report")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Generated at %s | Total: %d", time.Now().Format(time.RFC3339), len(vulns)))
	pdf.Ln(10)

	if len(vulns) == 0 {
		pdf.SetFont("Helvetica", "", 12)
		pdf.Cell(0, 8, "No vulnerabilities found in the current scope.")
	}

	// 列表。
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 7, "Vulnerability List")
	pdf.Ln(7)
	for _, v := range vulns {
		sevColor := map[string][3]int{
			"critical": {200, 0, 0}, "high": {230, 120, 0},
			"medium": {200, 160, 0}, "low": {200, 200, 0},
		}
		pdf.SetFont("Helvetica", "B", 10)
		title := fmt.Sprintf("[%s] %s (engine: %s)", v.Severity, v.Title, v.EngineName)
		if c, ok := sevColor[v.Severity]; ok {
			pdf.SetTextColor(c[0], c[1], c[2])
		}
		pdf.MultiCell(0, 6, title, "", "L", false)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Helvetica", "", 9)
		desc := v.Description
		if len(desc) > 300 {
			desc = desc[:300] + "..."
		}
		pdf.MultiCell(0, 5, fmt.Sprintf("  %s\n  CVE: %s | Status: %s | First seen: %s",
			desc, v.CVEID, v.Status, v.FirstSeenAt.Format("2006-01-02")), "", "L", false)
		pdf.Ln(2)
	}

	// 水印（对角线循环）。
	pdf.SetFont("Helvetica", "B", 36)
	pdf.SetTextColor(220, 220, 220)
	w, h := pdf.GetPageSize()
	for y := 20.0; y < h; y += 50 {
		for x := 10.0; x < w; x += 90 {
			pdf.TransformBegin()
			pdf.TransformRotate(45, x+40, y+12)
			pdf.Text(x, y, "CONFIDENTIAL")
			pdf.TransformEnd()
		}
	}
	pdf.SetTextColor(0, 0, 0)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
