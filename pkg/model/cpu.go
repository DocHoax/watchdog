package model

import "time"

// CPUCoreInfo represents per-core metrics.
type CPUCoreInfo struct {
	Index       int     `json:"index" yaml:"index"`
	ModelName   string  `json:"model_name,omitempty" yaml:"model_name,omitempty"`
	MHz         float64 `json:"mhz" yaml:"mhz"`
	UsagePct    float64 `json:"usage_pct" yaml:"usage_pct"`
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
}

// LoadAvg represents 1, 5, and 15-minute load averages.
type LoadAvg struct {
	Load1  float64 `json:"load1" yaml:"load1"`
	Load5  float64 `json:"load5" yaml:"load5"`
	Load15 float64 `json:"load15" yaml:"load15"`
}

// CPUInfo represents consolidated CPU metrics.
type CPUInfo struct {
	ModelName      string        `json:"model_name" yaml:"model_name"`
	VendorID       string        `json:"vendor_id" yaml:"vendor_id"`
	PhysicalCores  int           `json:"physical_cores" yaml:"physical_cores"`
	LogicalCores   int           `json:"logical_cores" yaml:"logical_cores"`
	OverallUsage   float64       `json:"overall_usage_pct" yaml:"overall_usage_pct"`
	Cores          []CPUCoreInfo `json:"cores" yaml:"cores"`
	LoadAverage    LoadAvg       `json:"load_average" yaml:"load_average"`
	FrequencyMHz   float64       `json:"frequency_mhz" yaml:"frequency_mhz"`
	TemperatureAvg float64       `json:"temperature_avg,omitempty" yaml:"temperature_avg,omitempty"`
	CollectedAt    time.Time     `json:"collected_at" yaml:"collected_at"`
}
