package collector

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/watchdog-cli/watchdog/pkg/model"
	"github.com/watchdog-cli/watchdog/pkg/util"
)

// SystemCollector collects host and operating system metadata.
type SystemCollector struct{}

// NewSystemCollector returns a new SystemCollector.
func NewSystemCollector() *SystemCollector {
	return &SystemCollector{}
}

// Name returns the identifier of the collector.
func (c *SystemCollector) Name() string {
	return "system"
}

// Collect gathers system information using gopsutil.
func (c *SystemCollector) Collect(ctx context.Context) (any, error) {
	return c.GetSystemInfo(ctx)
}

// GetSystemInfo fetches and formats host info.
func (c *SystemCollector) GetSystemInfo(ctx context.Context) (*model.SystemInfo, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	uptimeDur := time.Duration(info.Uptime) * time.Second
	bootTime := time.Unix(int64(info.BootTime), 0)

	return &model.SystemInfo{
		Hostname:        info.Hostname,
		OS:              info.OS,
		Platform:        info.Platform,
		PlatformFamily:  info.PlatformFamily,
		PlatformVersion: info.PlatformVersion,
		KernelVersion:   info.KernelVersion,
		KernelArch:      info.KernelArch,
		Uptime:          uptimeDur,
		UptimeFormatted: util.FormatDuration(uptimeDur),
		BootTime:        bootTime,
		ProcsCount:      info.Procs,
		HostID:          info.HostID,
		CollectedAt:     time.Now(),
	}, nil
}
