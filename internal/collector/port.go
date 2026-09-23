package collector

import (
	"context"
	"sort"
	"strings"
	"time"

	netutil "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

// PortCollector gathers listening network ports and binding processes.
type PortCollector struct{}

// NewPortCollector creates a new PortCollector.
func NewPortCollector() *PortCollector {
	return &PortCollector{}
}

// Name returns the identifier of the collector.
func (c *PortCollector) Name() string {
	return "port"
}

// Collect retrieves listening ports.
func (c *PortCollector) Collect(ctx context.Context) (any, error) {
	return c.GetListeningPorts(ctx)
}

// GetListeningPorts retrieves all active listening ports with PID and process name.
func (c *PortCollector) GetListeningPorts(ctx context.Context) ([]model.PortInfo, error) {
	conns, err := netutil.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, err
	}

	procNameCache := make(map[int32]string)
	var ports []model.PortInfo
	seen := make(map[string]bool)

	for _, conn := range conns {
		isListening := false
		if strings.ToUpper(conn.Status) == "LISTEN" || conn.Status == "NONE" || conn.Status == "" {
			if conn.Laddr.Port > 0 {
				isListening = true
			}
		}

		if !isListening {
			continue
		}

		proto := "TCP"
		if conn.Type == 2 {
			proto = "UDP"
		}

		key := proto + ":" + conn.Laddr.IP + ":" + string(rune(conn.Laddr.Port))
		if seen[key] {
			continue
		}
		seen[key] = true

		procName := "Unknown"
		if conn.Pid > 0 {
			if name, ok := procNameCache[conn.Pid]; ok {
				procName = name
			} else {
				p, err := process.NewProcessWithContext(ctx, conn.Pid)
				if err == nil {
					name, err := p.NameWithContext(ctx)
					if err == nil && name != "" {
						procName = name
						procNameCache[conn.Pid] = name
					}
				}
			}
		}

		ports = append(ports, model.PortInfo{
			Protocol:    proto,
			Port:        conn.Laddr.Port,
			BindAddress: conn.Laddr.IP,
			State:       conn.Status,
			PID:         conn.Pid,
			ProcessName: procName,
			CollectedAt: time.Now(),
		})
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	return ports, nil
}
