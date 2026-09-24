package collector

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/DocHoax/watchdog/pkg/model"
)

// MemoryCollector collects virtual memory and swap stats.
type MemoryCollector struct{}

// NewMemoryCollector returns a new MemoryCollector.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Name returns the identifier of the collector.
func (c *MemoryCollector) Name() string {
	return "memory"
}

// Collect gathers memory metrics.
func (c *MemoryCollector) Collect(ctx context.Context) (any, error) {
	return c.GetMemoryInfo(ctx)
}

// GetMemoryInfo fetches and formats virtual memory and swap info.
func (c *MemoryCollector) GetMemoryInfo(ctx context.Context) (*model.MemoryInfo, error) {
	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	swap, _ := mem.SwapMemoryWithContext(ctx)

	var swapTotal, swapUsed, swapFree uint64
	var swapUsedPct float64
	if swap != nil {
		swapTotal = swap.Total
		swapUsed = swap.Used
		swapFree = swap.Free
		swapUsedPct = swap.UsedPercent
	}

	return &model.MemoryInfo{
		TotalBytes:      vMem.Total,
		UsedBytes:       vMem.Used,
		FreeBytes:       vMem.Free,
		AvailableBytes:  vMem.Available,
		UsedPercent:     vMem.UsedPercent,
		CachedBytes:     vMem.Cached,
		BuffersBytes:    vMem.Buffers,
		SwapTotalBytes:  swapTotal,
		SwapUsedBytes:   swapUsed,
		SwapFreeBytes:   swapFree,
		SwapUsedPercent: swapUsedPct,
		CollectedAt:     time.Now(),
	}, nil
}
