package model

import "time"

// ServiceState defines the status of a service.
type ServiceState string

const (
	ServiceStateRunning ServiceState = "running"
	ServiceStateStopped ServiceState = "stopped"
	ServiceStatePaused  ServiceState = "paused"
	ServiceStateUnknown ServiceState = "unknown"
)

// ServiceInfo represents a system service/daemon (systemd, Windows service, launchd).
type ServiceInfo struct {
	Name        string       `json:"name" yaml:"name"`
	DisplayName string       `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Status      ServiceState `json:"status" yaml:"status"`
	PID         int32        `json:"pid,omitempty" yaml:"pid,omitempty"`
	StartType   string       `json:"start_type,omitempty" yaml:"start_type,omitempty"` // Auto, Manual, Disabled
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Manager     string       `json:"manager" yaml:"manager"` // windows, systemd, launchd
	CollectedAt time.Time    `json:"collected_at" yaml:"collected_at"`
}
