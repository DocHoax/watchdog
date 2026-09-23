package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default config failed validation: %v", err)
	}

	if cfg.RefreshInterval != 1*time.Second {
		t.Errorf("Expected 1s refresh interval, got %v", cfg.RefreshInterval)
	}

	if !cfg.Storage.Enabled {
		t.Errorf("Storage should be enabled by default")
	}

	if cfg.Alerts.CPU.Threshold != 90.0 {
		t.Errorf("Expected CPU alert threshold 90.0, got %f", cfg.Alerts.CPU.Threshold)
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	cfg := DefaultConfig()
	cfg.Alerts.CPU.Threshold = 75.5
	cfg.Storage.RetentionDays = 14

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, resolvedPath, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if resolvedPath != configPath {
		t.Errorf("Expected resolved path %s, got %s", configPath, resolvedPath)
	}

	if loaded.Alerts.CPU.Threshold != 75.5 {
		t.Errorf("Expected CPU threshold 75.5, got %f", loaded.Alerts.CPU.Threshold)
	}

	if loaded.Storage.RetentionDays != 14 {
		t.Errorf("Expected retention 14 days, got %d", loaded.Storage.RetentionDays)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Alerts.CPU.Threshold = 150.0 // Invalid > 100
	if err := cfg.Validate(); err == nil {
		t.Errorf("Expected error for invalid CPU threshold")
	}

	cfg = DefaultConfig()
	cfg.RefreshInterval = 10 * time.Millisecond // Too low
	if err := cfg.Validate(); err == nil {
		t.Errorf("Expected error for too low refresh interval")
	}

	cfg = DefaultConfig()
	cfg.Prometheus.Enabled = true
	cfg.Prometheus.Port = 99999 // Invalid port
	if err := cfg.Validate(); err == nil {
		t.Errorf("Expected error for invalid prometheus port")
	}
}

func TestLoadNonExistent(t *testing.T) {
	// Explicit non-existent path should return an error
	_, _, err := Load("/path/that/does/not/exist/watchdog_test.yaml")
	if err == nil {
		t.Fatalf("Expected error when explicit config path does not exist")
	}

	// Auto-discovery with empty string should fall back gracefully to default config
	cfg, _, err := Load("")
	if err != nil {
		t.Fatalf("Expected default config on auto-discovery, got error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("Expected non-nil default config")
	}
}

func TestFindConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	p := filepath.Join(tempDir, "custom.yaml")
	_ = os.WriteFile(p, []byte("refresh_interval: 2s\n"), 0644)

	found := FindConfigFile(p)
	if found != p {
		t.Errorf("Expected found %s, got %s", p, found)
	}
}
