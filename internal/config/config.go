package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for Watchdog.
type Config struct {
	RefreshInterval time.Duration    `yaml:"refresh_interval"`
	Storage         StorageConfig    `yaml:"storage"`
	Alerts          AlertsConfig     `yaml:"alerts"`
	Anomaly         AnomalyConfig    `yaml:"anomaly"`
	Collectors      CollectorsConfig `yaml:"collectors"`
	Dashboard       DashboardConfig  `yaml:"dashboard"`
	Prometheus      PrometheusConfig `yaml:"prometheus"`
	Agent           AgentConfig      `yaml:"agent"`
	Docker          DockerConfig     `yaml:"docker"`
	Kubernetes      KubernetesConfig `yaml:"kubernetes"`
}

// StorageConfig configures SQLite storage for metric history.
type StorageConfig struct {
	Enabled            bool          `yaml:"enabled"`
	DBPath             string        `yaml:"db_path"`
	RetentionDays      int           `yaml:"retention_days"`
	CollectionInterval time.Duration `yaml:"collection_interval"`
}

// ThresholdAlert defines metric alert threshold.
type ThresholdAlert struct {
	Enabled   bool          `yaml:"enabled"`
	Threshold float64       `yaml:"threshold"`
	Duration  time.Duration `yaml:"duration"`
	Cooldown  time.Duration `yaml:"cooldown"`
}

// ProcessAlert defines process-specific alert thresholds.
type ProcessAlert struct {
	Enabled         bool          `yaml:"enabled"`
	CPUThreshold    float64       `yaml:"cpu_threshold"`
	MemoryThreshold float64       `yaml:"memory_threshold"`
	Cooldown        time.Duration `yaml:"cooldown"`
}

// AlertsConfig bundles all alert thresholds.
type AlertsConfig struct {
	CPU     ThresholdAlert `yaml:"cpu"`
	Memory  ThresholdAlert `yaml:"memory"`
	Disk    ThresholdAlert `yaml:"disk"`
	Process ProcessAlert   `yaml:"process"`
	Network ThresholdAlert `yaml:"network"`
}

// AnomalyConfig configures the statistical anomaly detector.
type AnomalyConfig struct {
	Enabled         bool    `yaml:"enabled"`
	ZScoreThreshold float64 `yaml:"z_score_threshold"`
	WindowSize      int     `yaml:"window_size"`
	Alpha           float64 `yaml:"alpha"` // EWMA factor
}

// CollectorsConfig specifies which collectors are enabled.
type CollectorsConfig struct {
	CPU        bool `yaml:"cpu"`
	Memory     bool `yaml:"memory"`
	Disk       bool `yaml:"disk"`
	Network    bool `yaml:"network"`
	Process    bool `yaml:"process"`
	Service    bool `yaml:"service"`
	Port       bool `yaml:"port"`
	Docker     bool `yaml:"docker"`
	Kubernetes bool `yaml:"kubernetes"`
}

// DashboardConfig configures TUI appearance and defaults.
type DashboardConfig struct {
	Theme          string `yaml:"theme"`
	ShowPerCoreCPU bool   `yaml:"show_per_core_cpu"`
	ProcessSortBy  string `yaml:"process_sort_by"`
	ProcessLimit   int    `yaml:"process_limit"`
	PauseOnStart   bool   `yaml:"pause_on_start"`
}

// PrometheusConfig configures Prometheus metrics exporter.
type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// AgentConfig configures the remote agent server.
type AgentConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Port        int    `yaml:"port"`
	BindAddress string `yaml:"bind_address"`
	Token       string `yaml:"token"`
	TLSCert     string `yaml:"tls_cert"`
	TLSKey      string `yaml:"tls_key"`
}

// DockerConfig configures Docker container monitoring.
type DockerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
}

// KubernetesConfig configures Kubernetes monitoring.
type KubernetesConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
}

// DefaultConfig returns a fully populated, production-ready configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	defaultDBPath := filepath.Join(homeDir, ".watchdog", "watchdog.db")

	return &Config{
		RefreshInterval: 1 * time.Second,
		Storage: StorageConfig{
			Enabled:            true,
			DBPath:             defaultDBPath,
			RetentionDays:      7,
			CollectionInterval: 10 * time.Second,
		},
		Alerts: AlertsConfig{
			CPU: ThresholdAlert{
				Enabled:   true,
				Threshold: 90.0,
				Duration:  30 * time.Second,
				Cooldown:  5 * time.Minute,
			},
			Memory: ThresholdAlert{
				Enabled:   true,
				Threshold: 85.0,
				Duration:  30 * time.Second,
				Cooldown:  5 * time.Minute,
			},
			Disk: ThresholdAlert{
				Enabled:   true,
				Threshold: 90.0,
				Duration:  1 * time.Minute,
				Cooldown:  15 * time.Minute,
			},
			Process: ProcessAlert{
				Enabled:         true,
				CPUThreshold:    80.0,
				MemoryThreshold: 70.0,
				Cooldown:        5 * time.Minute,
			},
			Network: ThresholdAlert{
				Enabled:   true,
				Threshold: 100.0, // error packets threshold
				Duration:  1 * time.Minute,
				Cooldown:  10 * time.Minute,
			},
		},
		Anomaly: AnomalyConfig{
			Enabled:         true,
			ZScoreThreshold: 2.5,
			WindowSize:      60,
			Alpha:           0.2,
		},
		Collectors: CollectorsConfig{
			CPU:        true,
			Memory:     true,
			Disk:       true,
			Network:    true,
			Process:    true,
			Service:    true,
			Port:       true,
			Docker:     true,
			Kubernetes: false,
		},
		Dashboard: DashboardConfig{
			Theme:          "default",
			ShowPerCoreCPU: true,
			ProcessSortBy:  "cpu",
			ProcessLimit:   50,
			PauseOnStart:   false,
		},
		Prometheus: PrometheusConfig{
			Enabled: false,
			Port:    9100,
			Path:    "/metrics",
		},
		Agent: AgentConfig{
			Enabled:     false,
			Port:        8443,
			BindAddress: "127.0.0.1",
			Token:       "",
			TLSCert:     "",
			TLSKey:      "",
		},
		Docker: DockerConfig{
			Enabled: true,
			Host:    "",
		},
		Kubernetes: KubernetesConfig{
			Enabled:    false,
			Kubeconfig: "",
			Namespace:  "",
		},
	}
}

// GetDefaultConfigPath returns the default location for config.yaml.
func GetDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "watchdog.yaml"
	}
	return filepath.Join(homeDir, ".watchdog", "config.yaml")
}

// FindConfigFile searches for config files in standard locations.
func FindConfigFile(overridePath string) string {
	if overridePath != "" {
		if _, err := os.Stat(overridePath); err == nil {
			return overridePath
		}
		return overridePath
	}

	// 1. Current directory: ./watchdog.yaml, ./watchdog.yml
	candidates := []string{
		"watchdog.yaml",
		"watchdog.yml",
		".watchdog.yaml",
		GetDefaultConfigPath(),
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return GetDefaultConfigPath()
}

// Load reads and parses config from disk or returns default if not found.
func Load(path string) (*Config, string, error) {
	cfg := DefaultConfig()
	resolvedPath := FindConfigFile(path)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			if path != "" {
				return nil, resolvedPath, fmt.Errorf("config file %s does not exist", resolvedPath)
			}
			// No config file found during auto-discovery; return default
			return cfg, resolvedPath, nil
		}
		return nil, resolvedPath, fmt.Errorf("error reading config file %s: %w", resolvedPath, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, resolvedPath, fmt.Errorf("error parsing yaml config %s: %w", resolvedPath, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, resolvedPath, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, resolvedPath, nil
}

// Save writes configuration to disk.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to serialize config to yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}

	return nil
}

// Validate checks all settings for correctness.
func (c *Config) Validate() error {
	if c.RefreshInterval < 100*time.Millisecond {
		return fmt.Errorf("refresh_interval must be at least 100ms")
	}
	if c.Storage.Enabled && c.Storage.RetentionDays < 1 {
		return fmt.Errorf("storage.retention_days must be >= 1")
	}
	if c.Alerts.CPU.Threshold < 0 || c.Alerts.CPU.Threshold > 100 {
		return fmt.Errorf("alerts.cpu.threshold must be between 0 and 100")
	}
	if c.Alerts.Memory.Threshold < 0 || c.Alerts.Memory.Threshold > 100 {
		return fmt.Errorf("alerts.memory.threshold must be between 0 and 100")
	}
	if c.Alerts.Disk.Threshold < 0 || c.Alerts.Disk.Threshold > 100 {
		return fmt.Errorf("alerts.disk.threshold must be between 0 and 100")
	}
	if c.Anomaly.ZScoreThreshold <= 0 {
		return fmt.Errorf("anomaly.z_score_threshold must be positive")
	}
	if c.Anomaly.Alpha <= 0 || c.Anomaly.Alpha > 1.0 {
		return fmt.Errorf("anomaly.alpha must be between 0 and 1.0")
	}
	if c.Prometheus.Enabled && (c.Prometheus.Port < 1 || c.Prometheus.Port > 65535) {
		return fmt.Errorf("prometheus.port must be between 1 and 65535")
	}
	if c.Agent.Enabled && (c.Agent.Port < 1 || c.Agent.Port > 65535) {
		return fmt.Errorf("agent.port must be between 1 and 65535")
	}
	return nil
}
