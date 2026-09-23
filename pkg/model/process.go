package model

import "time"

// ProcessInfo represents a single operating system process.
type ProcessInfo struct {
	PID           int32          `json:"pid" yaml:"pid"`
	PPID          int32          `json:"ppid" yaml:"ppid"`
	Name          string         `json:"name" yaml:"name"`
	Username      string         `json:"username,omitempty" yaml:"username,omitempty"`
	CPUPercent    float64        `json:"cpu_percent" yaml:"cpu_percent"`
	MemoryPercent float32        `json:"memory_percent" yaml:"memory_percent"`
	MemoryRSS     uint64         `json:"memory_rss_bytes" yaml:"memory_rss_bytes"`
	MemoryVMS     uint64         `json:"memory_vms_bytes" yaml:"memory_vms_bytes"`
	Status        string         `json:"status" yaml:"status"` // Running, Sleeping, Stopped, Zombie
	NumThreads    int32          `json:"num_threads" yaml:"num_threads"`
	CreateTime    time.Time      `json:"create_time" yaml:"create_time"`
	CommandLine   string         `json:"command_line,omitempty" yaml:"command_line,omitempty"`
	ExePath       string         `json:"exe_path,omitempty" yaml:"exe_path,omitempty"`
	Nice          int32          `json:"nice,omitempty" yaml:"nice,omitempty"`
	ReadBytesSec  float64        `json:"read_bytes_sec,omitempty" yaml:"read_bytes_sec,omitempty"`
	WriteBytesSec float64        `json:"write_bytes_sec,omitempty" yaml:"write_bytes_sec,omitempty"`
	Children      []*ProcessInfo `json:"children,omitempty" yaml:"children,omitempty"`
}

// ProcessSummary provides overall process statistics.
type ProcessSummary struct {
	TotalCount    int           `json:"total_count" yaml:"total_count"`
	RunningCount  int           `json:"running_count" yaml:"running_count"`
	SleepingCount int           `json:"sleeping_count" yaml:"sleeping_count"`
	StoppedCount  int           `json:"stopped_count" yaml:"stopped_count"`
	ZombieCount   int           `json:"zombie_count" yaml:"zombie_count"`
	Processes     []ProcessInfo `json:"processes" yaml:"processes"`
	CollectedAt   time.Time     `json:"collected_at" yaml:"collected_at"`
}
