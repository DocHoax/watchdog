package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DocHoax/watchdog/internal/config"
)

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)
	RootCmd.SetArgs([]string{"version"})

	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestVersionCommandFlags(t *testing.T) {
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetArgs([]string{"version", "--short"})

	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("version --short command failed: %v", err)
	}

	buf.Reset()
	RootCmd.SetArgs([]string{"version", "--json"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("version --json command failed: %v", err)
	}

	// Verify VersionInfo struct fields
	vInfo := VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		Commit:    Commit,
		BuildDate: BuildDate,
		BuiltBy:   BuiltBy,
	}
	data, err := json.Marshal(vInfo)
	if err != nil {
		t.Fatalf("failed to marshal VersionInfo: %v", err)
	}
	if !strings.Contains(string(data), `"version"`) {
		t.Errorf("expected json to contain version field")
	}
}

func TestConfigCommands(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// 1. Init config
	RootCmd.SetArgs([]string{"config", "init", cfgPath})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("config init failed: %v", err)
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatalf("expected config file at %s to exist", cfgPath)
	}

	// 2. Validate config
	globalCfg = config.DefaultConfig()
	RootCmd.SetArgs([]string{"config", "validate"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("config validate failed: %v", err)
	}

	// 3. Show config
	RootCmd.SetArgs([]string{"config", "show", "--json"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("config show --json failed: %v", err)
	}

	// 4. Config path
	RootCmd.SetArgs([]string{"config", "path"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("config path failed: %v", err)
	}
}

func TestConfigShowRedaction(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.DefaultConfig()
	cfg.Agent.Token = "super-secret-auth-token"
	cfg.Agent.TLSKey = "private-key-material"
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Test YAML output with --config
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetArgs([]string{"--config", cfgPath, "config", "show"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("config show failed: %v", err)
	}

	if globalCfg.Agent.Token != "super-secret-auth-token" {
		t.Errorf("expected globalCfg to have original token, got %q", globalCfg.Agent.Token)
	}
	redacted := globalCfg.Redacted()
	if redacted.Agent.Token != "[REDACTED]" {
		t.Errorf("expected redacted token [REDACTED], got %q", redacted.Agent.Token)
	}
	if redacted.Agent.TLSKey != "[REDACTED]" {
		t.Errorf("expected redacted TLSKey [REDACTED], got %q", redacted.Agent.TLSKey)
	}
}

func TestDiagnoseCommand(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	RootCmd.SetArgs([]string{"diagnose", "--json"})
	err := RootCmd.Execute()
	// Diagnose may return an error if live system health detects critical thresholds (e.g. CPU or RAM near limit)
	if err != nil && !strings.Contains(err.Error(), "critical issue") {
		t.Fatalf("diagnose --json command failed unexpectedly: %v", err)
	}
}

func TestReportCommand(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	tmpDir := t.TempDir()
	outHTML := filepath.Join(tmpDir, "report.html")

	RootCmd.SetArgs([]string{"report", "--format", "html", "--output", outHTML})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("report html command failed: %v", err)
	}

	if _, err := os.Stat(outHTML); os.IsNotExist(err) {
		t.Fatalf("expected report HTML at %s to exist", outHTML)
	}
}

func TestAlertCommands(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	// Test alert list
	RootCmd.SetArgs([]string{"alert", "list"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("alert list failed: %v", err)
	}

	// Test alert test
	RootCmd.SetArgs([]string{"alert", "test"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("alert test failed: %v", err)
	}
}

func TestExportCommand(t *testing.T) {
	globalCfg = config.DefaultConfig()
	globalCfg.Storage.Enabled = false

	RootCmd.SetArgs([]string{"export", "--format", "json"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("export json failed: %v", err)
	}
}
