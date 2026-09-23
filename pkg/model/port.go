package model

import "time"

// PortInfo represents an open/listening network port.
type PortInfo struct {
	Protocol    string    `json:"protocol" yaml:"protocol"` // tcp, tcp6, udp, udp6
	Port        uint32    `json:"port" yaml:"port"`
	BindAddress string    `json:"bind_address" yaml:"bind_address"`
	State       string    `json:"state" yaml:"state"` // LISTEN, etc.
	PID         int32     `json:"pid" yaml:"pid"`
	ProcessName string    `json:"process_name" yaml:"process_name"`
	CollectedAt time.Time `json:"collected_at" yaml:"collected_at"`
}
