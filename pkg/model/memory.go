package model

import "time"

// MemoryInfo represents virtual memory and swap statistics.
type MemoryInfo struct {
	TotalBytes       uint64    `json:"total_bytes" yaml:"total_bytes"`
	UsedBytes        uint64    `json:"used_bytes" yaml:"used_bytes"`
	FreeBytes        uint64    `json:"free_bytes" yaml:"free_bytes"`
	AvailableBytes   uint64    `json:"available_bytes" yaml:"available_bytes"`
	UsedPercent      float64   `json:"used_percent" yaml:"used_percent"`
	CachedBytes      uint64    `json:"cached_bytes" yaml:"cached_bytes"`
	BuffersBytes     uint64    `json:"buffers_bytes" yaml:"buffers_bytes"`
	SwapTotalBytes   uint64    `json:"swap_total_bytes" yaml:"swap_total_bytes"`
	SwapUsedBytes    uint64    `json:"swap_used_bytes" yaml:"swap_used_bytes"`
	SwapFreeBytes    uint64    `json:"swap_free_bytes" yaml:"swap_free_bytes"`
	SwapUsedPercent  float64   `json:"swap_used_percent" yaml:"swap_used_percent"`
	CollectedAt      time.Time `json:"collected_at" yaml:"collected_at"`
}
