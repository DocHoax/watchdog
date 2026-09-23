package model

import "time"

// SystemInfo represents general host information.
type SystemInfo struct {
	Hostname        string        `json:"hostname" yaml:"hostname"`
	OS              string        `json:"os" yaml:"os"`
	Platform        string        `json:"platform" yaml:"platform"`
	PlatformFamily  string        `json:"platform_family" yaml:"platform_family"`
	PlatformVersion string        `json:"platform_version" yaml:"platform_version"`
	KernelVersion   string        `json:"kernel_version" yaml:"kernel_version"`
	KernelArch      string        `json:"kernel_arch" yaml:"kernel_arch"`
	Uptime          time.Duration `json:"uptime" yaml:"uptime"`
	UptimeFormatted string        `json:"uptime_formatted" yaml:"uptime_formatted"`
	BootTime        time.Time     `json:"boot_time" yaml:"boot_time"`
	ProcsCount      uint64        `json:"procs_count" yaml:"procs_count"`
	HostID          string        `json:"host_id" yaml:"host_id"`
	CollectedAt     time.Time     `json:"collected_at" yaml:"collected_at"`
}

// SystemSnapshot represents a complete point-in-time state of all system metrics.
type SystemSnapshot struct {
	Timestamp  time.Time       `json:"timestamp" yaml:"timestamp"`
	System     *SystemInfo     `json:"system,omitempty" yaml:"system,omitempty"`
	CPU        *CPUInfo        `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory     *MemoryInfo     `json:"memory,omitempty" yaml:"memory,omitempty"`
	Disk       *DiskInfo       `json:"disk,omitempty" yaml:"disk,omitempty"`
	Network    *NetworkInfo    `json:"network,omitempty" yaml:"network,omitempty"`
	Processes  *ProcessSummary `json:"processes,omitempty" yaml:"processes,omitempty"`
	Services   []ServiceInfo   `json:"services,omitempty" yaml:"services,omitempty"`
	Ports      []PortInfo      `json:"ports,omitempty" yaml:"ports,omitempty"`
	Docker     *DockerSummary  `json:"docker,omitempty" yaml:"docker,omitempty"`
	Kubernetes *K8sSummary     `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
}
