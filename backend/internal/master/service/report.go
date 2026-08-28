package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// ReportService 报告导出：PDF（含水印）/ Excel（漏洞清单）。
// 数据基于生成时刻快照，导出后处置变更不影响已生成内容。
// 同时管理定时报告模板与报告存档（PDF 落盘 Dir/reports/）。
type ReportService struct {
	DB        *gorm.DB
	Dir       string
	Dashboard *DashboardService
	Audit     *AuditWriter
}

// NewReportService 构造 ReportService。
func NewReportService(db *gorm.DB, dir string, dashboard *DashboardService, audit *AuditWriter) *ReportService {
	return &ReportService{DB: db, Dir: dir, Dashboard: dashboard, Audit: audit}
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

// ListTemplates 列出报告模板。
func (s *ReportService) ListTemplates() ([]models.ReportTemplate, error) {
	var list []models.ReportTemplate
	if err := s.DB.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// CreateTemplate 新建报告模板（cron 表达式须为合法 5 字段格式）。
func (s *ReportService) CreateTemplate(name, period, cronExpr, timezone string, enabled bool) (*models.ReportTemplate, error) {
	if name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "模板名称必填")
	}
	fields := splitFields(cronExpr)
	if len(fields) < 5 {
		return nil, errs.New(errs.CodeValidationFailed, "cron 表达式须为 5 字段（分 时 日 月 周）")
	}
	tpl := &models.ReportTemplate{
		Name: name, Period: period, CronExpr: cronExpr, Timezone: timezone, Enabled: enabled,
	}
	if err := s.DB.Create(tpl).Error; err != nil {
		return nil, err
	}
	return tpl, nil
}

// UpdateTemplate 更新模板配置。
func (s *ReportService) UpdateTemplate(id int64, name, cronExpr, timezone *string, enabled *bool) (*models.ReportTemplate, error) {
	var tpl models.ReportTemplate
	if err := s.DB.First(&tpl, id).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil && *name != "" {
		updates["name"] = *name
	}
	if cronExpr != nil && *cronExpr != "" {
		if len(splitFields(*cronExpr)) < 5 {
			return nil, errs.New(errs.CodeValidationFailed, "cron 表达式须为 5 字段（分 时 日 月 周）")
		}
		updates["cron_expr"] = *cronExpr
	}
	if timezone != nil {
		updates["timezone"] = *timezone
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if len(updates) > 0 {
		if err := s.DB.Model(&tpl).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &tpl, nil
}

// DeleteTemplate 删除模板（保留已生成的报告存档）。
func (s *ReportService) DeleteTemplate(id int64) error {
	return s.DB.Delete(&models.ReportTemplate{}, id).Error
}

// ListReports 列出报告存档。
func (s *ReportService) ListReports() ([]models.Report, error) {
	var list []models.Report
	if err := s.DB.Order("id DESC").Limit(200).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// reportSnapshot 报告快照统计。
type reportSnapshot struct {
	Assets     int64            `json:"assets"`
	Findings   int64            `json:"findings"`
	AlertsOpen int64            `json:"alerts_open"`
	Critical   int64            `json:"critical"`
	High       int64            `json:"high"`
	TopRisks   []models.Finding `json:"top_risks"`
}

// Generate 按模板生成报告存档：统计快照 + 漏洞 PDF 落盘。
func (s *ReportService) Generate(tpl *models.ReportTemplate, now time.Time) (*models.Report, error) {
	if s.Dashboard == nil {
		return nil, errs.New(errs.CodeValidationFailed, "报告服务未装配统计依赖")
	}
	stats, err := s.Dashboard.Stats()
	if err != nil {
		return nil, err
	}
	top, err := s.Dashboard.TopRisks(10)
	if err != nil {
		return nil, err
	}
	num := func(k string) int64 {
		v, _ := stats[k].(int64)
		return v
	}
	snap := reportSnapshot{
		Assets:     num("assets"),
		Findings:   num("findings"),
		AlertsOpen: num("alerts_open"),
		Critical:   num("critical"),
		High:       num("high"),
		TopRisks:   top,
	}
	snapJSON, _ := json.Marshal(snap)

	var vulns []models.Vulnerability
	if err := s.DB.Order("severity = 'critical' DESC, severity = 'high' DESC, id DESC").Limit(500).Find(&vulns).Error; err != nil {
		return nil, err
	}
	pdfData, err := scheduledReportPDF(tpl.Name, now, &snap, vulns)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(s.Dir, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	fileName := fmt.Sprintf("report-%d-%s.pdf", tpl.ID, now.Format("20060102150405"))
	if err := os.WriteFile(filepath.Join(dir, fileName), pdfData, 0o644); err != nil {
		return nil, err
	}
	rep := &models.Report{
		TemplateID: tpl.ID,
		Name:       tpl.Name,
		Title:      fmt.Sprintf("%s（%s）", tpl.Name, now.Format("2006-01-02 15:04")),
		Format:     "pdf",
		Status:     "completed",
		Progress:   100,
		FilePath:   fileName,
		Snapshot:   string(snapJSON),
	}
	if err := s.DB.Create(rep).Error; err != nil {
		return nil, err
	}
	return rep, nil
}

// Download 读取报告存档文件。
func (s *ReportService) Download(id int64) ([]byte, string, error) {
	var rep models.Report
	if err := s.DB.First(&rep, id).Error; err != nil {
		return nil, "", err
	}
	if rep.FilePath == "" {
		return nil, "", errs.New(errs.CodeNotFound, "报告文件不存在")
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, "reports", rep.FilePath))
	if err != nil {
		return nil, "", err
	}
	return data, fmt.Sprintf("report-%d-%s.pdf", rep.ID, rep.CreatedAt.Format("20060102")), nil
}

// DeleteReport 删除报告存档（连同 PDF 文件）。
func (s *ReportService) DeleteReport(id int64) error {
	var rep models.Report
	if err := s.DB.First(&rep, id).Error; err != nil {
		return err
	}
	if rep.FilePath != "" {
		_ = os.Remove(filepath.Join(s.Dir, "reports", rep.FilePath))
	}
	return s.DB.Delete(&models.Report{}, id).Error
}

// splitFields 按空白切分 cron 字段。
func splitFields(expr string) []string {
	var out []string
	for _, f := range strings.Split(expr, " ") {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
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

// scheduledReportPDF 生成定时报告 PDF：统计摘要 + 漏洞清单 + 对角线水印。
func scheduledReportPDF(tplName string, now time.Time, snap *reportSnapshot, vulns []models.Vulnerability) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("CInsight Scheduled Report", true)
	pdf.AddPage()

	// 页眉。
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, "CInsight Security Report")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Template: %s | Generated at %s", tplName, now.Format(time.RFC3339)))
	pdf.Ln(10)

	// 统计摘要。
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 7, "Summary")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 10)
	summary := fmt.Sprintf(
		"Assets: %d | Findings: %d | Open Alerts: %d\nCritical Findings: %d | High Findings: %d | Vulnerabilities Listed: %d",
		snap.Assets, snap.Findings, snap.AlertsOpen, snap.Critical, snap.High, len(vulns),
	)
	pdf.MultiCell(0, 6, summary, "", "L", false)
	pdf.Ln(4)

	// 风险 Top10。
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 7, "Top Risks")
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 9)
	for i, r := range snap.TopRisks {
		pdf.MultiCell(0, 5, fmt.Sprintf("%d. [%s] %s", i+1, r.Severity, r.Title), "", "L", false)
	}
	pdf.Ln(4)

	// 漏洞清单。
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
