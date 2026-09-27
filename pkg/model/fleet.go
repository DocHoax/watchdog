package model

import (
	"time"
)

// NodeStatus defines the operational lifecycle state of a fleet node.
type NodeStatus string

const (
	NodeStatusHealthy  NodeStatus = "healthy"
	NodeStatusWarning  NodeStatus = "warning"
	NodeStatusCritical NodeStatus = "critical"
	NodeStatusUnknown  NodeStatus = "unknown"
	NodeStatusStale    NodeStatus = "stale"
	NodeStatusOffline  NodeStatus = "offline"
)

// IsValid reports whether the node status is a recognized status constant.
func (s NodeStatus) IsValid() bool {
	switch s {
	case NodeStatusHealthy, NodeStatusWarning, NodeStatusCritical, NodeStatusUnknown, NodeStatusStale, NodeStatusOffline:
		return true
	default:
		return false
	}
}

// NodeIdentity encapsulates the permanent hardware and software identity of a node.
type NodeIdentity struct {
	NodeID       string            `json:"node_id" yaml:"node_id"`
	Hostname     string            `json:"hostname" yaml:"hostname"`
	OS           string            `json:"os" yaml:"os"`
	Platform     string            `json:"platform" yaml:"platform"`
	PlatformVer  string            `json:"platform_version,omitempty" yaml:"platform_version,omitempty"`
	Arch         string            `json:"arch" yaml:"arch"`
	KernelVer    string            `json:"kernel_version,omitempty" yaml:"kernel_version,omitempty"`
	Version      string            `json:"version" yaml:"version"`
	CPUCores     int               `json:"cpu_cores" yaml:"cpu_cores"`
	TotalMemory  uint64            `json:"total_memory" yaml:"total_memory"`
	IPAddresses  []string          `json:"ip_addresses,omitempty" yaml:"ip_addresses,omitempty"`
	MACAddresses []string          `json:"mac_addresses,omitempty" yaml:"mac_addresses,omitempty"`
	Tags         map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at" yaml:"created_at"`
}

// NodeSummary represents a concise point-in-time status of a node.
type NodeSummary struct {
	NodeID             string            `json:"node_id"`
	Hostname           string            `json:"hostname"`
	Status             NodeStatus        `json:"status"`
	StatusMessage      string            `json:"status_message,omitempty"`
	CPUUsagePercent    float64           `json:"cpu_usage_percent"`
	MemoryUsagePercent float64           `json:"memory_usage_percent"`
	DiskUsagePercent   float64           `json:"disk_usage_percent"`
	Load1              float64           `json:"load1,omitempty"`
	ActiveAlertsCount  int               `json:"active_alerts_count"`
	DiagnosticStatus   string            `json:"diagnostic_status,omitempty"`
	LastHeartbeat      time.Time         `json:"last_heartbeat"`
	LastTelemetry      *time.Time        `json:"last_telemetry,omitempty"`
	Version            string            `json:"version"`
	Tags               map[string]string `json:"tags,omitempty"`
}

// FleetNode represents a registered node in the central fleet registry.
type FleetNode struct {
	Identity      NodeIdentity      `json:"identity"`
	Status        NodeStatus        `json:"status"`
	StatusMessage string            `json:"status_message,omitempty"`
	RegisteredAt  time.Time         `json:"registered_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	LastTelemetry *time.Time        `json:"last_telemetry,omitempty"`
	Summary       *NodeSummary      `json:"summary,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HeartbeatRequest is submitted periodically by agents to attest liveness and current health.
type HeartbeatRequest struct {
	NodeID             string            `json:"node_id"`
	Timestamp          time.Time         `json:"timestamp"`
	Status             NodeStatus        `json:"status"`
	StatusMessage      string            `json:"status_message,omitempty"`
	CPUUsagePercent    float64           `json:"cpu_usage_percent"`
	MemoryUsagePercent float64           `json:"memory_usage_percent"`
	DiskUsagePercent   float64           `json:"disk_usage_percent"`
	Load1              float64           `json:"load1,omitempty"`
	ActiveAlertsCount  int               `json:"active_alerts_count"`
	DiagnosticStatus   string            `json:"diagnostic_status,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
	Version            string            `json:"version,omitempty"`
}

// HeartbeatResponse is returned by the fleet server upon receiving a heartbeat.
type HeartbeatResponse struct {
	Acknowledged          bool       `json:"acknowledged"`
	Timestamp             time.Time  `json:"timestamp"`
	NodeStatus            NodeStatus `json:"node_status"`
	NextHeartbeatInterval int        `json:"next_heartbeat_interval_seconds"`
	Directives            []string   `json:"directives,omitempty"` // Strictly read-only advisory instructions
}

// NodeRegistrationRequest is submitted by an agent to register its node identity with the central fleet server.
type NodeRegistrationRequest struct {
	Identity NodeIdentity      `json:"identity"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NodeRegistrationResponse confirms registration of a node and provides configuration parameters.
type NodeRegistrationResponse struct {
	Registered               bool      `json:"registered"`
	NodeID                   string    `json:"node_id"`
	RegisteredAt             time.Time `json:"registered_at"`
	HeartbeatIntervalSeconds int       `json:"heartbeat_interval_seconds"`
	TelemetryIntervalSeconds int       `json:"telemetry_interval_seconds"`
	Message                  string    `json:"message,omitempty"`
}

// TelemetrySubmission encapsulates a batch of metric and health observations from an agent.
type TelemetrySubmission struct {
	NodeID       string             `json:"node_id"`
	Timestamp    time.Time          `json:"timestamp"`
	Sequence     int64              `json:"sequence"`
	Snapshot     *SystemSnapshot    `json:"snapshot,omitempty"`
	Diagnostics  *DiagnosticReport  `json:"diagnostics,omitempty"`
	ActiveAlerts []AlertEvent       `json:"active_alerts,omitempty"`
	Metrics      map[string]float64 `json:"metrics,omitempty"`
	Tags         map[string]string  `json:"tags,omitempty"`
	Metadata     map[string]string  `json:"metadata,omitempty"`
}

// TelemetryResponse confirms receipt and processing of a telemetry payload.
type TelemetryResponse struct {
	Accepted      bool      `json:"accepted"`
	IngestedAt    time.Time `json:"ingested_at"`
	PointsCount   int       `json:"points_count"`
	DroppedPoints int       `json:"dropped_points,omitempty"`
	Message       string    `json:"message,omitempty"`
}

// FleetFilter specifies query filters for listing fleet nodes.
type FleetFilter struct {
	Status        NodeStatus        `json:"status,omitempty"`
	Search        string            `json:"search,omitempty"` // Matches hostname, node_id, or IP
	Tags          map[string]string `json:"tags,omitempty"`
	Since         time.Time         `json:"since,omitempty"`
	Limit         int               `json:"limit,omitempty"`
	Offset        int               `json:"offset,omitempty"`
	SortBy        string            `json:"sort_by,omitempty"`        // hostname, last_heartbeat, cpu, memory, status
	SortDirection string            `json:"sort_direction,omitempty"` // asc, desc
}

// FleetSummary provides aggregated overview metrics for the entire managed fleet.
type FleetSummary struct {
	TotalNodes    int         `json:"total_nodes"`
	HealthyNodes  int         `json:"healthy_nodes"`
	WarningNodes  int         `json:"warning_nodes"`
	CriticalNodes int         `json:"critical_nodes"`
	StaleNodes    int         `json:"stale_nodes"`
	OfflineNodes  int         `json:"offline_nodes"`
	UnknownNodes  int         `json:"unknown_nodes"`
	AvgCPUPercent float64     `json:"avg_cpu_percent"`
	AvgMemoryPct  float64     `json:"avg_memory_percent"`
	TotalAlerts   int         `json:"total_alerts"`
	LastUpdated   time.Time   `json:"last_updated"`
	Nodes         []FleetNode `json:"nodes,omitempty"`
}

// FleetListResponse represents a paginated list of fleet nodes with summary counts.
type FleetListResponse struct {
	Nodes     []FleetNode  `json:"nodes"`
	Total     int          `json:"total"`
	Count     int          `json:"count"`
	Limit     int          `json:"limit"`
	Offset    int          `json:"offset"`
	Summary   FleetSummary `json:"summary"`
	Timestamp time.Time    `json:"timestamp"`
}

// NodeDetailResponse provides comprehensive point-in-time telemetry and state for a single node.
type NodeDetailResponse struct {
	Node              FleetNode             `json:"node"`
	LatestSnapshot    *SystemSnapshot       `json:"latest_snapshot,omitempty"`
	LatestDiagnostics *DiagnosticReport     `json:"latest_diagnostics,omitempty"`
	ActiveAlerts      []AlertEvent          `json:"active_alerts,omitempty"`
	RecentTelemetry   []TelemetrySubmission `json:"recent_telemetry,omitempty"`
}

// APIErrorDetail encapsulates structured error context for API clients.
type APIErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Details   map[string]string `json:"details,omitempty"`
}

// APIErrorResponse represents the standard standardized JSON error payload.
type APIErrorResponse struct {
	Error APIErrorDetail `json:"error"`
}
