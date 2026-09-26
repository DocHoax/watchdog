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

	if !cfg.Audit.Enabled {
		t.Errorf("Audit should be enabled by default")
	}

	if cfg.Audit.RetentionDays != 90 {
		t.Errorf("Expected 90 days audit retention, got %d", cfg.Audit.RetentionDays)
	}

	if cfg.Audit.MaxQueryLimit != 1000 {
		t.Errorf("Expected 1000 audit max query limit, got %d", cfg.Audit.MaxQueryLimit)
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
	cfg.Audit.RetentionDays = 180
	cfg.Audit.MaxQueryLimit = 500

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

	if loaded.Audit.RetentionDays != 180 {
		t.Errorf("Expected audit retention 180 days, got %d", loaded.Audit.RetentionDays)
	}

	if loaded.Audit.MaxQueryLimit != 500 {
		t.Errorf("Expected audit max query limit 500, got %d", loaded.Audit.MaxQueryLimit)
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

	// Audit validation tests
	cfg = DefaultConfig()
	cfg.Audit.Enabled = true
	cfg.Audit.RetentionDays = 0
	if err := cfg.Validate(); err == nil {
		t.Errorf("Expected error for audit retention days < 1")
	}

	cfg = DefaultConfig()
	cfg.Audit.Enabled = true
	cfg.Audit.MaxQueryLimit = 0
	if err := cfg.Validate(); err == nil {
		t.Errorf("Expected error for audit max query limit < 1")
	}

	cfg = DefaultConfig()
	cfg.Audit.Enabled = true
	cfg.Audit.MaxQueryLimit = 5001
	if err := cfg.Validate(); err == nil {
		t.Errorf("Expected error for audit max query limit > 5000")
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

func TestResolveToken_Precedence(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "token.secret")
	if err := os.WriteFile(tokenFile, []byte("  file-token-secret\r\n  "), 0600); err != nil {
		t.Fatalf("Failed to write test token file: %v", err)
	}

	// 1. TokenFile takes precedence over TokenEnv and Token
	t.Setenv("TEST_AGENT_TOKEN_ENV", "env-token-secret")
	t.Setenv("WATCHDOG_AGENT_TOKEN", "default-agent-token")
	t.Setenv("WATCHDOG_AUTH_TOKEN", "default-auth-token")

	agent := AgentConfig{
		Token:     "plain-token",
		TokenEnv:  "TEST_AGENT_TOKEN_ENV",
		TokenFile: tokenFile,
	}
	resolved, err := agent.ResolveToken()
	if err != nil {
		t.Fatalf("Unexpected error resolving token: %v", err)
	}
	if resolved != "file-token-secret" {
		t.Errorf("Expected token from file 'file-token-secret', got %q", resolved)
	}

	// 2. TokenEnv takes precedence over default env vars and plain token
	agent.TokenFile = ""
	resolved, err = agent.ResolveToken()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resolved != "env-token-secret" {
		t.Errorf("Expected token from TokenEnv 'env-token-secret', got %q", resolved)
	}

	// 3. WATCHDOG_AGENT_TOKEN takes precedence over WATCHDOG_AUTH_TOKEN and plain token
	agent.TokenEnv = ""
	resolved, err = agent.ResolveToken()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resolved != "default-agent-token" {
		t.Errorf("Expected token from WATCHDOG_AGENT_TOKEN, got %q", resolved)
	}

	// 4. WATCHDOG_AUTH_TOKEN fallback
	t.Setenv("WATCHDOG_AGENT_TOKEN", "")
	resolved, err = agent.ResolveToken()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resolved != "default-auth-token" {
		t.Errorf("Expected token from WATCHDOG_AUTH_TOKEN, got %q", resolved)
	}

	// 5. Plain config token
	t.Setenv("WATCHDOG_AUTH_TOKEN", "")
	resolved, err = agent.ResolveToken()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resolved != "plain-token" {
		t.Errorf("Expected token from plain config, got %q", resolved)
	}

	// 6. Empty default
	agent.Token = ""
	resolved, err = agent.ResolveToken()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resolved != "" {
		t.Errorf("Expected empty token, got %q", resolved)
	}
}

func TestResolveTLSKeyAndCert_Precedence(t *testing.T) {
	tempDir := t.TempDir()
	keyFile := filepath.Join(tempDir, "key.pem")
	certFile := filepath.Join(tempDir, "cert.pem")
	if err := os.WriteFile(keyFile, []byte("key-data"), 0600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}
	if err := os.WriteFile(certFile, []byte("cert-data"), 0644); err != nil {
		t.Fatalf("Failed to create cert file: %v", err)
	}

	t.Setenv("CUSTOM_KEY_ENV", keyFile)
	t.Setenv("CUSTOM_CERT_ENV", certFile)

	agent := AgentConfig{
		TLSKey:      "plain-key",
		TLSKeyEnv:   "CUSTOM_KEY_ENV",
		TLSKeyFile:  keyFile,
		TLSCert:     "plain-cert",
		TLSCertEnv:  "CUSTOM_CERT_ENV",
		TLSCertFile: certFile,
	}

	// Key file precedence
	resolvedKey, err := agent.ResolveTLSKey()
	if err != nil {
		t.Fatalf("Unexpected error resolving TLS key: %v", err)
	}
	if resolvedKey != keyFile {
		t.Errorf("Expected key file %q, got %q", keyFile, resolvedKey)
	}

	// Cert file precedence
	resolvedCert, err := agent.ResolveTLSCert()
	if err != nil {
		t.Fatalf("Unexpected error resolving TLS cert: %v", err)
	}
	if resolvedCert != certFile {
		t.Errorf("Expected cert file %q, got %q", certFile, resolvedCert)
	}

	// ResolveSecrets on agent
	if err := agent.ResolveSecrets(); err != nil {
		t.Fatalf("ResolveSecrets failed: %v", err)
	}
	if agent.TLSKey != keyFile || agent.TLSCert != certFile {
		t.Errorf("ResolveSecrets did not set fields correctly: key=%s, cert=%s", agent.TLSKey, agent.TLSCert)
	}
}

func TestSecretFile_ValidationErrors(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Non-existent file
	agent := AgentConfig{TokenFile: filepath.Join(tempDir, "missing.token")}
	if _, err := agent.ResolveToken(); err == nil {
		t.Errorf("Expected error for non-existent token file")
	}

	// 2. Empty file
	emptyFile := filepath.Join(tempDir, "empty.token")
	_ = os.WriteFile(emptyFile, []byte(""), 0600)
	agent.TokenFile = emptyFile
	if _, err := agent.ResolveToken(); err == nil {
		t.Errorf("Expected error for empty token file")
	}

	// 3. Whitespace-only file
	wsFile := filepath.Join(tempDir, "ws.token")
	_ = os.WriteFile(wsFile, []byte("  \t\r\n  "), 0600)
	agent.TokenFile = wsFile
	if _, err := agent.ResolveToken(); err == nil {
		t.Errorf("Expected error for whitespace-only token file")
	}
}

func TestConfig_Redacted(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Token = "super-secret-token"
	cfg.Agent.TLSKey = "private-key-data"
	cfg.Agent.TLSCert = "/path/to/cert.pem"
	cfg.Agent.TokenFile = "/path/to/token.secret"

	redacted := cfg.Redacted()

	// Verify original is untouched
	if cfg.Agent.Token != "super-secret-token" {
		t.Errorf("Original config was mutated!")
	}
	if cfg.Agent.TLSKey != "private-key-data" {
		t.Errorf("Original config was mutated!")
	}

	// Verify redacted clone
	if redacted.Agent.Token != "[REDACTED]" {
		t.Errorf("Expected redacted token, got %q", redacted.Agent.Token)
	}
	if redacted.Agent.TLSKey != "[REDACTED]" {
		t.Errorf("Expected redacted TLSKey, got %q", redacted.Agent.TLSKey)
	}
	if redacted.Agent.TLSCert != "/path/to/cert.pem" {
		t.Errorf("TLSCert should not be redacted, got %q", redacted.Agent.TLSCert)
	}
	if redacted.Agent.TokenFile != "/path/to/token.secret" {
		t.Errorf("TokenFile path should be preserved, got %q", redacted.Agent.TokenFile)
	}

	// CloneRedacted alias
	clone := cfg.CloneRedacted()
	if clone.Agent.Token != "[REDACTED]" || clone.Agent.TLSKey != "[REDACTED]" {
		t.Errorf("CloneRedacted failed to mask credentials")
	}
}

func TestConfig_Save_SafeSerialization(t *testing.T) {
	tempDir := t.TempDir()
	savePath := filepath.Join(tempDir, "safe_saved_config.yaml")

	cfg := DefaultConfig()
	cfg.Agent.Token = "resolved-in-memory-token"
	cfg.Agent.TokenFile = "/etc/watchdog/token"
	cfg.Agent.TLSKey = "resolved-in-memory-key"
	cfg.Agent.TLSKeyFile = "/etc/watchdog/key.pem"

	if err := cfg.Save(savePath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Read raw saved YAML file
	data, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("Failed to read saved config file: %v", err)
	}
	savedStr := string(data)

	// Ensure plaintext tokens and keys are NOT present in the saved YAML
	if stringContains(savedStr, "resolved-in-memory-token") {
		t.Errorf("Plaintext token was leaked into saved config file!")
	}
	if stringContains(savedStr, "resolved-in-memory-key") {
		t.Errorf("Plaintext TLS key was leaked into saved config file!")
	}
	if !stringContains(savedStr, "token_file: /etc/watchdog/token") {
		t.Errorf("token_file reference should be preserved in saved config file")
	}
}

func stringContains(s, substr string) bool {
	return filepath.Clean(s) != "" && len(s) >= len(substr) && (s == substr || filepath.Base(s) == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
