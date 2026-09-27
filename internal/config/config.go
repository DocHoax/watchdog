package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for Watchdog.
type Config struct {
	RefreshInterval time.Duration    `yaml:"refresh_interval"`
	Storage         StorageConfig    `yaml:"storage"`
	Audit           AuditConfig      `yaml:"audit"`
	Alerts          AlertsConfig     `yaml:"alerts"`
	Anomaly         AnomalyConfig    `yaml:"anomaly"`
	Collectors      CollectorsConfig `yaml:"collectors"`
	Dashboard       DashboardConfig  `yaml:"dashboard"`
	Prometheus      PrometheusConfig `yaml:"prometheus"`
	Agent           AgentConfig      `yaml:"agent"`
	Fleet           FleetConfig      `yaml:"fleet"`
	MCP             MCPConfig        `yaml:"mcp"`
	Docker          DockerConfig     `yaml:"docker"`
	Kubernetes      KubernetesConfig `yaml:"kubernetes"`
}

// AuditConfig configures security audit logging and retention.
type AuditConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	RetentionDays int  `yaml:"retention_days" json:"retention_days"`
	MaxQueryLimit int  `yaml:"max_query_limit" json:"max_query_limit"`
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
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	Port        int    `yaml:"port" json:"port"`
	BindAddress string `yaml:"bind_address" json:"bind_address"`
	Token       string `yaml:"token,omitempty" json:"token,omitempty"`
	TokenFile   string `yaml:"token_file,omitempty" json:"token_file,omitempty"`
	TokenEnv    string `yaml:"token_env,omitempty" json:"token_env,omitempty"`
	TLSCert     string `yaml:"tls_cert,omitempty" json:"tls_cert,omitempty"`
	TLSCertFile string `yaml:"tls_cert_file,omitempty" json:"tls_cert_file,omitempty"`
	TLSCertEnv  string `yaml:"tls_cert_env,omitempty" json:"tls_cert_env,omitempty"`
	TLSKey      string `yaml:"tls_key,omitempty" json:"tls_key,omitempty"`
	TLSKeyFile  string `yaml:"tls_key_file,omitempty" json:"tls_key_file,omitempty"`
	TLSKeyEnv   string `yaml:"tls_key_env,omitempty" json:"tls_key_env,omitempty"`
}

// FleetConfig configures node fleet management, registration, and telemetry forwarding.
type FleetConfig struct {
	Enabled           bool              `yaml:"enabled" json:"enabled"`
	ServerURL         string            `yaml:"server_url" json:"server_url"`                         // Central fleet controller URL
	NodeID            string            `yaml:"node_id,omitempty" json:"node_id,omitempty"`           // Explicit override for node UUID
	NodeIDFile        string            `yaml:"node_id_file,omitempty" json:"node_id_file,omitempty"` // Path to node ID file (default ~/.watchdog/node_id)
	HeartbeatInterval time.Duration     `yaml:"heartbeat_interval" json:"heartbeat_interval"`         // Frequency of liveness heartbeats
	TelemetryInterval time.Duration     `yaml:"telemetry_interval" json:"telemetry_interval"`         // Frequency of telemetry pushes
	BufferCapacity    int               `yaml:"buffer_capacity" json:"buffer_capacity"`               // Max telemetry items to buffer offline
	MaxBufferBytes    int64             `yaml:"max_buffer_bytes" json:"max_buffer_bytes"`             // Max buffer size in bytes
	Tags              map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`                 // Node metadata tags
	Token             string            `yaml:"token,omitempty" json:"token,omitempty"`               // Auth token for fleet server
	TokenFile         string            `yaml:"token_file,omitempty" json:"token_file,omitempty"`     // Path to fleet auth token file
	TokenEnv          string            `yaml:"token_env,omitempty" json:"token_env,omitempty"`       // Env var containing fleet auth token
	TLSCACert         string            `yaml:"tls_ca_cert,omitempty" json:"tls_ca_cert,omitempty"`   // Custom CA cert file for server TLS verification
	RateLimitRate     float64           `yaml:"rate_limit_rate" json:"rate_limit_rate"`               // Server-side max requests per second per node
	RateLimitBurst    int               `yaml:"rate_limit_burst" json:"rate_limit_burst"`             // Server-side rate limit burst
	StaleThreshold    time.Duration     `yaml:"stale_threshold" json:"stale_threshold"`               // Mark node stale if no heartbeat after duration
	OfflineThreshold  time.Duration     `yaml:"offline_threshold" json:"offline_threshold"`           // Mark node offline if no heartbeat after duration
}

// MCPConfig configures the Model Context Protocol (MCP) server for AI client integrations.
type MCPConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	Transport           string        `yaml:"transport" json:"transport"` // "stdio", "http", or "sse"
	Port                int           `yaml:"port" json:"port"`
	BindAddress         string        `yaml:"bind_address" json:"bind_address"`
	Token               string        `yaml:"token,omitempty" json:"token,omitempty"`
	TokenFile           string        `yaml:"token_file,omitempty" json:"token_file,omitempty"`
	TokenEnv            string        `yaml:"token_env,omitempty" json:"token_env,omitempty"`
	TLSCert             string        `yaml:"tls_cert,omitempty" json:"tls_cert,omitempty"`
	TLSCertFile         string        `yaml:"tls_cert_file,omitempty" json:"tls_cert_file,omitempty"`
	TLSCertEnv          string        `yaml:"tls_cert_env,omitempty" json:"tls_cert_env,omitempty"`
	TLSKey              string        `yaml:"tls_key,omitempty" json:"tls_key,omitempty"`
	TLSKeyFile          string        `yaml:"tls_key_file,omitempty" json:"tls_key_file,omitempty"`
	TLSKeyEnv           string        `yaml:"tls_key_env,omitempty" json:"tls_key_env,omitempty"`
	RateLimitRate       float64       `yaml:"rate_limit_rate" json:"rate_limit_rate"`
	RateLimitBurst      int           `yaml:"rate_limit_burst" json:"rate_limit_burst"`
	MaxRequestBodyBytes int64         `yaml:"max_request_body_bytes" json:"max_request_body_bytes"`
	ReadTimeout         time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout        time.Duration `yaml:"write_timeout" json:"write_timeout"`
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
		Audit: AuditConfig{
			Enabled:       true,
			RetentionDays: 90,
			MaxQueryLimit: 1000,
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
		Fleet: FleetConfig{
			Enabled:           false,
			ServerURL:         "",
			NodeIDFile:        filepath.Join(homeDir, ".watchdog", "node_id"),
			HeartbeatInterval: 30 * time.Second,
			TelemetryInterval: 60 * time.Second,
			BufferCapacity:    1000,
			MaxBufferBytes:    10 * 1024 * 1024,
			Tags:              map[string]string{},
			RateLimitRate:     10.0,
			RateLimitBurst:    20,
			StaleThreshold:    2 * time.Minute,
			OfflineThreshold:  10 * time.Minute,
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
			if err := cfg.ResolveSecrets(); err != nil {
				return nil, resolvedPath, err
			}
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

	if err := cfg.ResolveSecrets(); err != nil {
		return nil, resolvedPath, fmt.Errorf("error resolving secrets: %w", err)
	}

	return cfg, resolvedPath, nil
}

// Save writes configuration to disk safely without persisting resolved plaintext secrets
// that originated from secret files or environment variables.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Prepare safe copy for saving
	saveCopy := *c
	saveAgent := c.Agent

	// If Token was configured via TokenFile or TokenEnv, do not write the resolved plaintext Token to disk
	if saveAgent.TokenFile != "" || saveAgent.TokenEnv != "" {
		saveAgent.Token = ""
	}
	// If TLSKey was configured via TLSKeyFile or TLSKeyEnv, do not write resolved TLSKey to disk
	if saveAgent.TLSKeyFile != "" || saveAgent.TLSKeyEnv != "" {
		saveAgent.TLSKey = ""
	}
	// If TLSCert was configured via TLSCertFile or TLSCertEnv, do not write resolved TLSCert to disk
	if saveAgent.TLSCertFile != "" || saveAgent.TLSCertEnv != "" {
		saveAgent.TLSCert = ""
	}
	saveCopy.Agent = saveAgent

	saveFleet := c.Fleet
	if saveFleet.TokenFile != "" || saveFleet.TokenEnv != "" {
		saveFleet.Token = ""
	}
	saveCopy.Fleet = saveFleet

	data, err := yaml.Marshal(&saveCopy)
	if err != nil {
		return fmt.Errorf("failed to serialize config to yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", path, err)
	}

	return nil
}

// CheckSecretFilePermissions verifies that a sensitive file does not have overly permissive file modes.
// On POSIX operating systems, files containing credentials should have permissions <= 0600 (read/write by owner only).
// If permissions allow group or other access (perm & 0077 != 0), an error is returned.
// On Windows, POSIX permission bits are simulated and this check is a graceful no-op.
func CheckSecretFilePermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		return fmt.Errorf("secret file %q has overly permissive permissions (%04o); should be 0600 or stricter", path, perm)
	}

	return nil
}

// readSecretFile reads and validates the content of a secret file.
func readSecretFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if err := CheckSecretFilePermissions(path); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: %v\n", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read secret file %q: %w", path, err)
	}

	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("secret file %q is empty or contains only whitespace", path)
	}

	return secret, nil
}

// ResolveToken resolves the authentication token according to precedence:
// 1. Secret file (TokenFile)
// 2. Explicit environment variable name (TokenEnv)
// 3. Default environment variables (WATCHDOG_AGENT_TOKEN, WATCHDOG_AUTH_TOKEN)
// 4. Plain config value (Token)
func (a *AgentConfig) ResolveToken() (string, error) {
	if a.TokenFile != "" {
		token, err := readSecretFile(a.TokenFile)
		if err != nil {
			return "", err
		}
		return token, nil
	}

	if a.TokenEnv != "" {
		if val := os.Getenv(a.TokenEnv); strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val), nil
		}
	}

	if val := os.Getenv("WATCHDOG_AGENT_TOKEN"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}
	if val := os.Getenv("WATCHDOG_AUTH_TOKEN"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}

	if strings.TrimSpace(a.Token) != "" {
		return strings.TrimSpace(a.Token), nil
	}

	return "", nil
}

// ResolveTLSKey resolves the TLS private key file path according to precedence:
// 1. Secret file path (TLSKeyFile)
// 2. Explicit environment variable name (TLSKeyEnv)
// 3. Default environment variables (WATCHDOG_AGENT_TLS_KEY, WATCHDOG_TLS_KEY)
// 4. Plain config value (TLSKey)
func (a *AgentConfig) ResolveTLSKey() (string, error) {
	if a.TLSKeyFile != "" {
		if _, err := os.Stat(a.TLSKeyFile); err != nil {
			return "", fmt.Errorf("failed to access tls_key_file %q: %w", a.TLSKeyFile, err)
		}
		if err := CheckSecretFilePermissions(a.TLSKeyFile); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: %v\n", err)
		}
		return a.TLSKeyFile, nil
	}

	if a.TLSKeyEnv != "" {
		if val := os.Getenv(a.TLSKeyEnv); strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val), nil
		}
	}

	if val := os.Getenv("WATCHDOG_AGENT_TLS_KEY"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}
	if val := os.Getenv("WATCHDOG_TLS_KEY"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}

	if strings.TrimSpace(a.TLSKey) != "" {
		return strings.TrimSpace(a.TLSKey), nil
	}

	return "", nil
}

// ResolveTLSCert resolves the TLS certificate file path according to precedence:
// 1. Secret file path (TLSCertFile)
// 2. Explicit environment variable name (TLSCertEnv)
// 3. Default environment variables (WATCHDOG_AGENT_TLS_CERT, WATCHDOG_TLS_CERT)
// 4. Plain config value (TLSCert)
func (a *AgentConfig) ResolveTLSCert() (string, error) {
	if a.TLSCertFile != "" {
		if _, err := os.Stat(a.TLSCertFile); err != nil {
			return "", fmt.Errorf("failed to access tls_cert_file %q: %w", a.TLSCertFile, err)
		}
		return a.TLSCertFile, nil
	}

	if a.TLSCertEnv != "" {
		if val := os.Getenv(a.TLSCertEnv); strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val), nil
		}
	}

	if val := os.Getenv("WATCHDOG_AGENT_TLS_CERT"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}
	if val := os.Getenv("WATCHDOG_TLS_CERT"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}

	if strings.TrimSpace(a.TLSCert) != "" {
		return strings.TrimSpace(a.TLSCert), nil
	}

	return "", nil
}

// ResolveSecrets resolves all sensitive configuration fields.
func (a *AgentConfig) ResolveSecrets() error {
	token, err := a.ResolveToken()
	if err != nil {
		return fmt.Errorf("failed to resolve agent token: %w", err)
	}
	a.Token = token

	key, err := a.ResolveTLSKey()
	if err != nil {
		return fmt.Errorf("failed to resolve agent tls_key: %w", err)
	}
	a.TLSKey = key

	cert, err := a.ResolveTLSCert()
	if err != nil {
		return fmt.Errorf("failed to resolve agent tls_cert: %w", err)
	}
	a.TLSCert = cert

	return nil
}

// ResolveToken resolves the fleet authentication token following the precedence hierarchy:
// 1. Secret file path (TokenFile)
// 2. Explicit environment variable name (TokenEnv)
// 3. Default environment variables (WATCHDOG_FLEET_TOKEN, WATCHDOG_TOKEN)
// 4. Plain config value (Token)
func (f *FleetConfig) ResolveToken() (string, error) {
	if f.TokenFile != "" {
		resolvedPath := f.TokenFile
		if strings.HasPrefix(resolvedPath, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to expand home dir for token_file %q: %w", f.TokenFile, err)
			}
			resolvedPath = filepath.Join(home, resolvedPath[1:])
		}
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read token_file %q: %w", f.TokenFile, err)
		}
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("token_file %q is empty", f.TokenFile)
		}
		return token, nil
	}

	if f.TokenEnv != "" {
		if val := os.Getenv(f.TokenEnv); strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val), nil
		}
	}

	if val := os.Getenv("WATCHDOG_FLEET_TOKEN"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}
	if val := os.Getenv("WATCHDOG_TOKEN"); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val), nil
	}

	if strings.TrimSpace(f.Token) != "" {
		return strings.TrimSpace(f.Token), nil
	}

	return "", nil
}

// ResolveSecrets resolves all sensitive configuration fields in FleetConfig.
func (f *FleetConfig) ResolveSecrets() error {
	token, err := f.ResolveToken()
	if err != nil {
		return fmt.Errorf("failed to resolve fleet token: %w", err)
	}
	f.Token = token
	return nil
}

// Redacted returns a deep copy of FleetConfig with sensitive credentials masked as "[REDACTED]".
func (f *FleetConfig) Redacted() *FleetConfig {
	clone := *f
	if clone.Token != "" {
		clone.Token = "[REDACTED]"
	}
	return &clone
}

// CloneRedacted returns a deep copy of FleetConfig with sensitive credentials masked as "[REDACTED]".
func (f *FleetConfig) CloneRedacted() *FleetConfig {
	return f.Redacted()
}

// ResolveSecrets resolves all sensitive configuration fields in the root Config.
func (c *Config) ResolveSecrets() error {
	if err := c.Agent.ResolveSecrets(); err != nil {
		return err
	}
	return c.Fleet.ResolveSecrets()
}

// Redacted returns a deep copy of Config with sensitive credentials masked as "[REDACTED]".
func (c *Config) Redacted() *Config {
	clone := *c
	clone.Agent = *c.Agent.Redacted()
	clone.Fleet = *c.Fleet.Redacted()
	return &clone
}

// CloneRedacted returns a deep copy of Config with sensitive credentials masked as "[REDACTED]".
func (c *Config) CloneRedacted() *Config {
	return c.Redacted()
}

// Redacted returns a deep copy of AgentConfig with sensitive credentials masked as "[REDACTED]".
func (a *AgentConfig) Redacted() *AgentConfig {
	clone := *a
	if clone.Token != "" {
		clone.Token = "[REDACTED]"
	}
	if clone.TLSKey != "" {
		clone.TLSKey = "[REDACTED]"
	}
	return &clone
}

// CloneRedacted returns a deep copy of AgentConfig with sensitive credentials masked as "[REDACTED]".
func (a *AgentConfig) CloneRedacted() *AgentConfig {
	return a.Redacted()
}

// Validate checks all settings for correctness.
func (c *Config) Validate() error {
	if c.RefreshInterval < 100*time.Millisecond {
		return fmt.Errorf("refresh_interval must be at least 100ms")
	}
	if c.Storage.Enabled && c.Storage.RetentionDays < 1 {
		return fmt.Errorf("storage.retention_days must be >= 1")
	}
	if c.Audit.Enabled {
		if c.Audit.RetentionDays < 1 {
			return fmt.Errorf("audit.retention_days must be >= 1")
		}
		if c.Audit.MaxQueryLimit < 1 || c.Audit.MaxQueryLimit > 5000 {
			return fmt.Errorf("audit.max_query_limit must be between 1 and 5000")
		}
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
	if c.Fleet.Enabled {
		if c.Fleet.HeartbeatInterval < 500*time.Millisecond {
			return fmt.Errorf("fleet.heartbeat_interval must be at least 500ms")
		}
		if c.Fleet.TelemetryInterval < 500*time.Millisecond {
			return fmt.Errorf("fleet.telemetry_interval must be at least 500ms")
		}
		if c.Fleet.BufferCapacity < 1 {
			return fmt.Errorf("fleet.buffer_capacity must be >= 1")
		}
		if c.Fleet.MaxBufferBytes < 1024 {
			return fmt.Errorf("fleet.max_buffer_bytes must be >= 1024")
		}
		if c.Fleet.RateLimitRate < 0 {
			return fmt.Errorf("fleet.rate_limit_rate must be >= 0")
		}
		if c.Fleet.RateLimitBurst < 0 {
			return fmt.Errorf("fleet.rate_limit_burst must be >= 0")
		}
		if c.Fleet.StaleThreshold < time.Second {
			return fmt.Errorf("fleet.stale_threshold must be at least 1s")
		}
		if c.Fleet.OfflineThreshold < c.Fleet.StaleThreshold {
			return fmt.Errorf("fleet.offline_threshold must be >= fleet.stale_threshold")
		}
	}
	return nil
}
