package model

import "time"

// ContainerStats represents resource metrics for a container.
type ContainerStats struct {
	CPUPercent     float64 `json:"cpu_percent" yaml:"cpu_percent"`
	MemoryUsage    uint64  `json:"memory_usage_bytes" yaml:"memory_usage_bytes"`
	MemoryLimit    uint64  `json:"memory_limit_bytes" yaml:"memory_limit_bytes"`
	MemoryPercent  float64 `json:"memory_percent" yaml:"memory_percent"`
	NetworkRxBytes uint64  `json:"network_rx_bytes" yaml:"network_rx_bytes"`
	NetworkTxBytes uint64  `json:"network_tx_bytes" yaml:"network_tx_bytes"`
	BlockReadBytes uint64  `json:"block_read_bytes" yaml:"block_read_bytes"`
	BlockWriteByte uint64  `json:"block_write_bytes" yaml:"block_write_bytes"`
	PIDs           uint64  `json:"pids" yaml:"pids"`
}

// DockerContainer represents a Docker container.
type DockerContainer struct {
	ID           string          `json:"id" yaml:"id"`
	Names        []string        `json:"names" yaml:"names"`
	Image        string          `json:"image" yaml:"image"`
	ImageID      string          `json:"image_id" yaml:"image_id"`
	Command      string          `json:"command" yaml:"command"`
	Created      time.Time       `json:"created" yaml:"created"`
	State        string          `json:"state" yaml:"state"` // running, exited, paused
	Status       string          `json:"status" yaml:"status"` // "Up 2 hours"
	Ports        []string        `json:"ports,omitempty" yaml:"ports,omitempty"`
	RestartCount int             `json:"restart_count" yaml:"restart_count"`
	Stats        *ContainerStats `json:"stats,omitempty" yaml:"stats,omitempty"`
}

// DockerSummary represents overall Docker engine status.
type DockerSummary struct {
	Available       bool              `json:"available" yaml:"available"`
	Version         string            `json:"version,omitempty" yaml:"version,omitempty"`
	APIVersion      string            `json:"api_version,omitempty" yaml:"api_version,omitempty"`
	ContainersTotal int               `json:"containers_total" yaml:"containers_total"`
	RunningCount    int               `json:"running_count" yaml:"running_count"`
	PausedCount     int               `json:"paused_count" yaml:"paused_count"`
	StoppedCount    int               `json:"stopped_count" yaml:"stopped_count"`
	ImagesCount     int               `json:"images_count" yaml:"images_count"`
	Containers      []DockerContainer `json:"containers" yaml:"containers"`
	Error           string            `json:"error,omitempty" yaml:"error,omitempty"`
	CollectedAt     time.Time         `json:"collected_at" yaml:"collected_at"`
}
