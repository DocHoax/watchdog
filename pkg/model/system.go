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
