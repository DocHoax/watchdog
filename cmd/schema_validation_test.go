package cmd

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func TestDiagnoseJSONSchema(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "diagnose.json")

	RootCmd.SetArgs([]string{"diagnose", "--json", "--output", outPath})
	_ = RootCmd.Execute()

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read diagnose JSON output: %v", err)
	}

	var report model.DiagnosticReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("diagnose JSON does not unmarshal to model.DiagnosticReport: %v", err)
	}

	// Verify required schema fields
	if report.TotalChecks == 0 {
		t.Errorf("expected TotalChecks > 0, got %d", report.TotalChecks)
	}
	if report.OverallStatus == "" {
		t.Errorf("expected non-empty OverallStatus")
	}
	if len(report.Results) == 0 {
		t.Errorf("expected at least 1 diagnostic result")
	}

	for i, res := range report.Results {
		if res.ID == "" {
			t.Errorf("result [%d] missing ID", i)
		}
		if res.Category == "" {
			t.Errorf("result [%d] missing Category", i)
		}
		if res.Name == "" {
			t.Errorf("result [%d] missing Name", i)
		}
		if res.Status == "" {
			t.Errorf("result [%d] missing Status", i)
		}
		if res.Severity == "" {
			t.Errorf("result [%d] missing Severity", i)
		}
	}
}

func TestExportJSONSchema(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "export.json")

	RootCmd.SetArgs([]string{"export", "--format", "json", "--output", outPath})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("export JSON command failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read export JSON output: %v", err)
	}

	var snap model.SystemSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("export JSON does not unmarshal to model.SystemSnapshot: %v", err)
	}

	if snap.Timestamp.IsZero() {
		t.Errorf("expected non-zero snapshot Timestamp")
	}
	if snap.System == nil || snap.System.Hostname == "" {
		t.Errorf("expected non-empty system hostname in snapshot")
	}
	if snap.CPU == nil {
		t.Errorf("expected CPU info in snapshot")
	}
	if snap.Memory == nil || snap.Memory.TotalBytes == 0 {
		t.Errorf("expected non-zero Memory info in snapshot")
	}
	if snap.Disk == nil {
		t.Errorf("expected Disk info in snapshot")
	}
	if snap.Network == nil {
		t.Errorf("expected Network info in snapshot")
	}
}

func TestReportJSONSchema(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "report.json")

	RootCmd.SetArgs([]string{"report", "--format", "json", "--output", outPath})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("report JSON command failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read report JSON output: %v", err)
	}

	var rep model.ReportData
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("report JSON does not unmarshal to model.ReportData: %v", err)
	}

	if rep.Title == "" {
		t.Errorf("expected non-empty Title in ReportData")
	}
	if rep.GeneratedAt.IsZero() {
		t.Errorf("expected non-zero GeneratedAt in ReportData")
	}
	if rep.Host.Hostname == "" {
		t.Errorf("expected non-empty Host.Hostname in ReportData")
	}
	if rep.Memory.TotalBytes == 0 {
		t.Errorf("expected non-zero Memory.TotalBytes in ReportData")
	}
}

func TestExportAndReportCSVSchema(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "metrics.csv")

	RootCmd.SetArgs([]string{"export", "--format", "csv", "--output", csvPath})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("export CSV command failed: %v", err)
	}

	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read CSV output: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(data)))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV records: %v", err)
	}

	if len(records) < 2 {
		t.Fatalf("expected at least header + 1 data row, got %d rows", len(records))
	}

	header := records[0]
	expectedHeaders := []string{
		"timestamp",
		"cpu_overall_usage",
		"cpu_load1",
		"cpu_load5",
		"cpu_load15",
		"memory_used_pct",
		"memory_used_bytes",
		"memory_total_bytes",
		"swap_used_pct",
		"disk_used_pct",
		"disk_read_bytes_sec",
		"disk_write_bytes_sec",
		"net_rx_bytes_sec",
		"net_tx_bytes_sec",
		"processes_total",
		"processes_zombies",
	}

	if len(header) != len(expectedHeaders) {
		t.Fatalf("expected %d columns in CSV header, got %d", len(expectedHeaders), len(header))
	}

	for i, h := range expectedHeaders {
		if header[i] != h {
			t.Errorf("header column %d mismatch: expected %q, got %q", i, h, header[i])
		}
	}

	// Verify data row columns
	dataRow := records[1]
	if len(dataRow) != len(expectedHeaders) {
		t.Fatalf("data row length %d does not match header length %d", len(dataRow), len(expectedHeaders))
	}

	// Check timestamp parsing RFC3339
	if _, err := time.Parse(time.RFC3339, dataRow[0]); err != nil {
		t.Errorf("data row timestamp %q is not valid RFC3339: %v", dataRow[0], err)
	}

	// Check CPU usage numeric parse
	if _, err := strconv.ParseFloat(dataRow[1], 64); err != nil {
		t.Errorf("cpu_overall_usage %q is not valid float: %v", dataRow[1], err)
	}

	// Check memory_used_bytes numeric parse
	if _, err := strconv.ParseUint(dataRow[6], 10, 64); err != nil {
		t.Errorf("memory_used_bytes %q is not valid uint: %v", dataRow[6], err)
	}
}

func TestHTMLReportSelfContainedAndResponsive(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	tmpDir := t.TempDir()
	htmlPath := filepath.Join(tmpDir, "report.html")

	RootCmd.SetArgs([]string{"report", "--format", "html", "--output", htmlPath, "--title", "Security Audit Report"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("report HTML command failed: %v", err)
	}

	data, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read HTML report: %v", err)
	}
	htmlStr := string(data)

	// 1. Standalone HTML validation (no external CDN script/link tags)
	if strings.Contains(htmlStr, "http://") || strings.Contains(htmlStr, "https://") {
		// Verify no external resources are loaded
		if strings.Contains(htmlStr, "<script src=\"http") || strings.Contains(htmlStr, "<link rel=\"stylesheet\" href=\"http") {
			t.Errorf("HTML report contains external CDN script or stylesheet dependencies")
		}
	}

	// 2. Self-contained CSS & Theme Toggle
	if !strings.Contains(htmlStr, "<style>") || !strings.Contains(htmlStr, "</style>") {
		t.Errorf("HTML report missing embedded <style> block")
	}
	if !strings.Contains(htmlStr, "--bg-primary:") {
		t.Errorf("HTML report missing dark mode CSS variables")
	}
	if !strings.Contains(htmlStr, "toggleTheme()") {
		t.Errorf("HTML report missing client-side theme toggle")
	}

	// 3. Responsive viewport meta
	if !strings.Contains(htmlStr, "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">") {
		t.Errorf("HTML report missing responsive viewport meta tag")
	}

	// 4. Custom title verification
	if !strings.Contains(htmlStr, "Security Audit Report") {
		t.Errorf("HTML report does not contain custom title")
	}
}
