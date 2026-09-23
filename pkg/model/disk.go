package model

import "time"

// PartitionInfo represents disk partition usage.
type PartitionInfo struct {
	Device      string  `json:"device" yaml:"device"`
	Mountpoint  string  `json:"mountpoint" yaml:"mountpoint"`
	FSType      string  `json:"fs_type" yaml:"fs_type"`
	Opts        string  `json:"opts,omitempty" yaml:"opts,omitempty"`
	TotalBytes  uint64  `json:"total_bytes" yaml:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes" yaml:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes" yaml:"free_bytes"`
	UsedPercent float64 `json:"used_percent" yaml:"used_percent"`
	InodesTotal uint64  `json:"inodes_total,omitempty" yaml:"inodes_total,omitempty"`
	InodesUsed  uint64  `json:"inodes_used,omitempty" yaml:"inodes_used,omitempty"`
	InodesFree  uint64  `json:"inodes_free,omitempty" yaml:"inodes_free,omitempty"`
	InodesPct   float64 `json:"inodes_percent,omitempty" yaml:"inodes_percent,omitempty"`
}

// DiskIOCounters represents disk I/O metrics.
type DiskIOCounters struct {
	Name       string    `json:"name" yaml:"name"`
	ReadBytes  uint64    `json:"read_bytes" yaml:"read_bytes"`
	WriteBytes uint64    `json:"write_bytes" yaml:"write_bytes"`
	ReadCount  uint64    `json:"read_count" yaml:"read_count"`
	WriteCount uint64    `json:"write_count" yaml:"write_count"`
	ReadRate   float64   `json:"read_bytes_sec" yaml:"read_bytes_sec"`   // bytes/sec
	WriteRate  float64   `json:"write_bytes_sec" yaml:"write_bytes_sec"` // bytes/sec
	ReadIOPS   float64   `json:"read_iops" yaml:"read_iops"`             // ops/sec
	WriteIOPS  float64   `json:"write_iops" yaml:"write_iops"`           // ops/sec
	Timestamp  time.Time `json:"timestamp" yaml:"timestamp"`
}

// DiskInfo represents consolidated disk information.
type DiskInfo struct {
	Partitions  []PartitionInfo  `json:"partitions" yaml:"partitions"`
	IOCounters  []DiskIOCounters `json:"io_counters" yaml:"io_counters"`
	TotalBytes  uint64           `json:"total_bytes" yaml:"total_bytes"`
	UsedBytes   uint64           `json:"used_bytes" yaml:"used_bytes"`
	FreeBytes   uint64           `json:"free_bytes" yaml:"free_bytes"`
	UsedPercent float64          `json:"used_percent" yaml:"used_percent"`
	CollectedAt time.Time        `json:"collected_at" yaml:"collected_at"`
}
