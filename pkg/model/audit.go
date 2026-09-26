package model

import "time"

// Audit Event Type Constants
const (
	// Authentication Events
	EventAuthSuccess            = "auth.success"
	EventAuthFailure            = "auth.failure"
	EventAuthMissingCredentials = "auth.missing_credentials"
	EventAuthInvalidCredentials = "auth.invalid_credentials"

	// Server Lifecycle Events
	EventServerStart        = "server.start"
	EventServerStop         = "server.stop"
	EventServerStartFailure = "server.start.failure"
	EventServerShutdown     = "server.shutdown"

	// TLS Events
	EventTLSEnabled       = "tls.enabled"
	EventTLSDisabled      = "tls.disabled"
	EventTLSConfigFailure = "tls.configuration.failure"
	EventTLSCertFailure   = "tls.certificate.failure"
	EventTLSKeyFailure    = "tls.private_key.failure"

	// Configuration Events
	EventConfigLoaded            = "config.loaded"
	EventConfigValidationFailure = "config.validation.failure"
	EventConfigChanged           = "config.changed"
	EventConfigSave              = "config.save"
	EventConfigSaveFailure       = "config.save.failure"

	// Security Configuration Events
	EventSecurityAuthEnabled   = "security.auth.enabled"
	EventSecurityAuthFailure   = "security.auth.configuration.failure"
	EventSecurityTLSFailure    = "security.tls.configuration.failure"
	EventSecuritySecretFailure = "security.secret.source.failure"

	// Administrative Events
	EventAdminConfigChange = "admin.configuration.change"
	EventAdminServerStart  = "admin.server.start"
	EventAdminServerStop   = "admin.server.stop"
	EventAdminExport       = "admin.export"
	EventAdminAuditPurge   = "admin.audit.purge"
)

// Audit Severity Constants
const (
	AuditSeverityInfo     = "info"
	AuditSeverityNotice   = "notice"
	AuditSeverityWarning  = "warning"
	AuditSeverityError    = "error"
	AuditSeverityCritical = "critical"
)

// Audit Outcome Constants
const (
	AuditOutcomeSuccess = "success"
	AuditOutcomeFailure = "failure"
	AuditOutcomeDenied  = "denied"
)

// Audit Actor Type Constants
const (
	ActorTypeSystem              = "system"
	ActorTypeCLI                 = "cli"
	ActorTypeAuthenticatedClient = "authenticated_client"
	ActorTypeAnonymousClient     = "anonymous_client"
)

// AuditActor represents the entity that initiated the event.
type AuditActor struct {
	Type     string `json:"type" yaml:"type"`                   // system, cli, authenticated_client, anonymous_client
	Identity string `json:"identity,omitempty" yaml:"identity"` // safe identifier, e.g. "api-client" (NEVER tokens or secrets)
}

// AuditSource captures safe request origin and transport metadata.
type AuditSource struct {
	Address   string `json:"address,omitempty" yaml:"address"`       // IP address or "local"
	Transport string `json:"transport,omitempty" yaml:"transport"`   // tcp, unix, loopback
	Protocol  string `json:"protocol,omitempty" yaml:"protocol"`     // HTTP/1.1, HTTP/2.0
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent"` // client user agent
	Endpoint  string `json:"endpoint,omitempty" yaml:"endpoint"`     // requested URL path
	Method    string `json:"method,omitempty" yaml:"method"`         // GET, POST, etc.
	RequestID string `json:"request_id,omitempty" yaml:"request_id"` // request correlation ID
}

// AuditEvent represents a structured, security-relevant audit event record.
type AuditEvent struct {
	ID        string            `json:"id" yaml:"id"`
	Timestamp time.Time         `json:"timestamp" yaml:"timestamp"` // UTC timestamp
	EventType string            `json:"event_type" yaml:"event_type"`
	Severity  string            `json:"severity" yaml:"severity"`
	Outcome   string            `json:"outcome" yaml:"outcome"` // success, failure, denied
	Actor     AuditActor        `json:"actor" yaml:"actor"`
	Source    AuditSource       `json:"source" yaml:"source"`
	Resource  string            `json:"resource,omitempty" yaml:"resource,omitempty"`
	Action    string            `json:"action,omitempty" yaml:"action,omitempty"`
	Message   string            `json:"message,omitempty" yaml:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}
