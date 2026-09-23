package collector

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/sensors"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// CPUCollector collects CPU metrics across cores.
type CPUCollector struct {
	mu           sync.Mutex
	lastCorePerc []float64
}

// NewCPUCollector returns a new CPUCollector instance.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

// Name returns the identifier of the collector.
func (c *CPUCollector) Name() string {
	return "cpu"
}

// Collect retrieves current CPU metrics.
func (c *CPUCollector) Collect(ctx context.Context) (any, error) {
	return c.GetCPUInfo(ctx)
}

// GetCPUInfo collects aggregated and per-core CPU statistics.
func (c *CPUCollector) GetCPUInfo(ctx context.Context) (*model.CPUInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Physical & logical cores
	physCores, _ := cpu.CountsWithContext(ctx, false)
	logCores, _ := cpu.CountsWithContext(ctx, true)
	if logCores == 0 {
		logCores = runtime.NumCPU()
	}

	// Overall CPU usage percentage (interval 0 means instantaneous diff since last call)
	overallPerc, err := cpu.PercentWithContext(ctx, 0, false)
	var overall float64
	if err == nil && len(overallPerc) > 0 {
		overall = overallPerc[0]
	}

	// Per-core CPU percentage
	perCorePerc, _ := cpu.PercentWithContext(ctx, 0, true)
	if len(perCorePerc) == 0 && len(c.lastCorePerc) > 0 {
		perCorePerc = c.lastCorePerc
	} else if len(perCorePerc) > 0 {
		c.lastCorePerc = perCorePerc
	}

	// CPU hardware info
	cpuInfos, _ := cpu.InfoWithContext(ctx)
	modelName := "Unknown CPU"
	vendorID := ""
	var freqMHz float64

	if len(cpuInfos) > 0 {
		modelName = cpuInfos[0].ModelName
		vendorID = cpuInfos[0].VendorID
		freqMHz = cpuInfos[0].Mhz
	}

	// Build per-core list
	cores := make([]model.CPUCoreInfo, len(perCorePerc))
	for i, pct := range perCorePerc {
		coreModel := modelName
		coreMHz := freqMHz
		if i < len(cpuInfos) {
			if cpuInfos[i].ModelName != "" {
				coreModel = cpuInfos[i].ModelName
			}
			if cpuInfos[i].Mhz > 0 {
				coreMHz = cpuInfos[i].Mhz
			}
		}
		cores[i] = model.CPUCoreInfo{
			Index:     i,
			ModelName: coreModel,
			MHz:       coreMHz,
			UsagePct:  pct,
		}
	}

	// Load averages (supported on Unix; gracefully fall back on Windows)
	var loadAvg model.LoadAvg
	lInfo, err := load.AvgWithContext(ctx)
	if err == nil && lInfo != nil {
		loadAvg = model.LoadAvg{
			Load1:  lInfo.Load1,
			Load5:  lInfo.Load5,
			Load15: lInfo.Load15,
		}
	} else {
		// On Windows where unix load avg is not native, calculate proportional load from usage
		loadAvg = model.LoadAvg{
			Load1:  (overall / 100.0) * float64(logCores),
			Load5:  (overall / 100.0) * float64(logCores),
			Load15: (overall / 100.0) * float64(logCores),
		}
	}

	// Temperature sensors (graceful if unavailable)
	var tempAvg float64
	tempSensors, err := sensors.TemperaturesWithContext(ctx)
	if err == nil && len(tempSensors) > 0 {
		var totalTemp float64
		count := 0
		for _, s := range tempSensors {
			if s.Temperature > 0 && s.Temperature < 150 {
				totalTemp += s.Temperature
				count++
			}
		}
		if count > 0 {
			tempAvg = totalTemp / float64(count)
		}
	}

	return &model.CPUInfo{
		ModelName:      modelName,
		VendorID:       vendorID,
		PhysicalCores:  physCores,
		LogicalCores:   logCores,
		OverallUsage:   overall,
		Cores:          cores,
		LoadAverage:    loadAvg,
		FrequencyMHz:   freqMHz,
		TemperatureAvg: tempAvg,
		CollectedAt:    time.Now(),
	}, nil
}
