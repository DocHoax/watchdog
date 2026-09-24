package collector

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/DocHoax/watchdog/pkg/model"
)

// DiskCollector gathers partition usage and disk I/O metrics.
type DiskCollector struct {
	mu           sync.Mutex
	lastIOCheck  time.Time
	lastCounters map[string]disk.IOCountersStat
}

// NewDiskCollector creates a new DiskCollector instance.
func NewDiskCollector() *DiskCollector {
	return &DiskCollector{
		lastCounters: make(map[string]disk.IOCountersStat),
	}
}

// Name returns the identifier of the collector.
func (c *DiskCollector) Name() string {
	return "disk"
}

// Collect gathers disk partitions and I/O metrics.
func (c *DiskCollector) Collect(ctx context.Context) (any, error) {
	return c.GetDiskInfo(ctx)
}

// GetDiskInfo fetches partition usage and disk I/O throughput.
func (c *DiskCollector) GetDiskInfo(ctx context.Context) (*model.DiskInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// 1. Partitions and usages
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		// Fallback to all partitions
		partitions, _ = disk.PartitionsWithContext(ctx, true)
	}

	var partInfos []model.PartitionInfo
	var totalBytes, usedBytes, freeBytes uint64

	seenMounts := make(map[string]bool)
	for _, p := range partitions {
		if p.Mountpoint == "" || seenMounts[p.Mountpoint] {
			continue
		}
		seenMounts[p.Mountpoint] = true

		u, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			continue
		}

		partInfo := model.PartitionInfo{
			Device:      p.Device,
			Mountpoint:  p.Mountpoint,
			FSType:      p.Fstype,
			Opts:        strings.Join(p.Opts, ","),
			TotalBytes:  u.Total,
			UsedBytes:   u.Used,
			FreeBytes:   u.Free,
			UsedPercent: u.UsedPercent,
			InodesTotal: u.InodesTotal,
			InodesUsed:  u.InodesUsed,
			InodesFree:  u.InodesFree,
			InodesPct:   u.InodesUsedPercent,
		}
		partInfos = append(partInfos, partInfo)
		totalBytes += u.Total
		usedBytes += u.Used
		freeBytes += u.Free
	}

	var overallUsedPct float64
	if totalBytes > 0 {
		overallUsedPct = (float64(usedBytes) / float64(totalBytes)) * 100.0
	}

	// 2. Disk I/O Counters
	ioCounters, _ := disk.IOCountersWithContext(ctx)
	var ioList []model.DiskIOCounters

	timeDelta := now.Sub(c.lastIOCheck).Seconds()
	if timeDelta <= 0 {
		timeDelta = 1.0
	}

	for name, cur := range ioCounters {
		var readRate, writeRate, readIOPS, writeIOPS float64

		if prev, exists := c.lastCounters[name]; exists && !c.lastIOCheck.IsZero() && timeDelta > 0.1 {
			if cur.ReadBytes >= prev.ReadBytes {
				readRate = float64(cur.ReadBytes-prev.ReadBytes) / timeDelta
			}
			if cur.WriteBytes >= prev.WriteBytes {
				writeRate = float64(cur.WriteBytes-prev.WriteBytes) / timeDelta
			}
			if cur.ReadCount >= prev.ReadCount {
				readIOPS = float64(cur.ReadCount-prev.ReadCount) / timeDelta
			}
			if cur.WriteCount >= prev.WriteCount {
				writeIOPS = float64(cur.WriteCount-prev.WriteCount) / timeDelta
			}
		}

		c.lastCounters[name] = cur

		ioList = append(ioList, model.DiskIOCounters{
			Name:       name,
			ReadBytes:  cur.ReadBytes,
			WriteBytes: cur.WriteBytes,
			ReadCount:  cur.ReadCount,
			WriteCount: cur.WriteCount,
			ReadRate:   readRate,
			WriteRate:  writeRate,
			ReadIOPS:   readIOPS,
			WriteIOPS:  writeIOPS,
			Timestamp:  now,
		})
	}

	c.lastIOCheck = now

	return &model.DiskInfo{
		Partitions:  partInfos,
		IOCounters:  ioList,
		TotalBytes:  totalBytes,
		UsedBytes:   usedBytes,
		FreeBytes:   freeBytes,
		UsedPercent: overallUsedPct,
		CollectedAt: now,
	}, nil
}
