package collector

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
	netutil "github.com/shirou/gopsutil/v4/net"
)

// NetworkCollector gathers network interfaces, I/O rates, and connections.
type NetworkCollector struct {
	mu           sync.Mutex
	lastCheck    time.Time
	lastIOCounts map[string]netutil.IOCountersStat
}

// NewNetworkCollector creates a new NetworkCollector instance.
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		lastIOCounts: make(map[string]netutil.IOCountersStat),
	}
}

// Name returns the identifier of the collector.
func (c *NetworkCollector) Name() string {
	return "network"
}

// Collect retrieves current network metrics.
func (c *NetworkCollector) Collect(ctx context.Context) (any, error) {
	return c.GetNetworkInfo(ctx)
}

// GetNetworkInfo collects interface details, throughput rates, and active sockets.
func (c *NetworkCollector) GetNetworkInfo(ctx context.Context) (*model.NetworkInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	timeDelta := now.Sub(c.lastCheck).Seconds()
	if timeDelta <= 0 {
		timeDelta = 1.0
	}

	// 1. Interfaces
	ifaces, err := net.Interfaces()
	var ifaceInfos []model.NetworkInterfaceInfo
	if err == nil {
		for _, iface := range ifaces {
			var addrsList []string
			if addrs, err := iface.Addrs(); err == nil {
				for _, a := range addrs {
					addrsList = append(addrsList, a.String())
				}
			}

			var flagsList []string
			flagStr := iface.Flags.String()
			flagsList = append(flagsList, flagStr)

			isUp := (iface.Flags & net.FlagUp) != 0
			isLoop := (iface.Flags & net.FlagLoopback) != 0

			ifaceInfos = append(ifaceInfos, model.NetworkInterfaceInfo{
				Index:        iface.Index,
				MTU:          iface.MTU,
				Name:         iface.Name,
				HardwareAddr: iface.HardwareAddr.String(),
				Flags:        flagsList,
				Addrs:        addrsList,
				IsUp:         isUp,
				IsLoopback:   isLoop,
			})
		}
	}

	// 2. IO Counters (per-interface)
	ioStats, _ := netutil.IOCountersWithContext(ctx, true)
	var ioList []model.NetworkIOInfo
	var totalTxRate, totalRxRate float64
	var totalSent, totalRecv uint64

	for _, stat := range ioStats {
		var txRate, rxRate, txPktsRate, rxPktsRate float64

		if prev, exists := c.lastIOCounts[stat.Name]; exists && !c.lastCheck.IsZero() && timeDelta > 0.1 {
			if stat.BytesSent >= prev.BytesSent {
				txRate = float64(stat.BytesSent-prev.BytesSent) / timeDelta
			}
			if stat.BytesRecv >= prev.BytesRecv {
				rxRate = float64(stat.BytesRecv-prev.BytesRecv) / timeDelta
			}
			if stat.PacketsSent >= prev.PacketsSent {
				txPktsRate = float64(stat.PacketsSent-prev.PacketsSent) / timeDelta
			}
			if stat.PacketsRecv >= prev.PacketsRecv {
				rxPktsRate = float64(stat.PacketsRecv-prev.PacketsRecv) / timeDelta
			}
		}

		c.lastIOCounts[stat.Name] = stat

		totalTxRate += txRate
		totalRxRate += rxRate
		totalSent += stat.BytesSent
		totalRecv += stat.BytesRecv

		ioList = append(ioList, model.NetworkIOInfo{
			Name:         stat.Name,
			BytesSent:    stat.BytesSent,
			BytesRecv:    stat.BytesRecv,
			PacketsSent:  stat.PacketsSent,
			PacketsRecv:  stat.PacketsRecv,
			ErrIn:        stat.Errin,
			ErrOut:       stat.Errout,
			DropIn:       stat.Dropin,
			DropOut:      stat.Dropout,
			TxRate:       txRate,
			RxRate:       rxRate,
			TxPacketsSec: txPktsRate,
			RxPacketsSec: rxPktsRate,
			Timestamp:    now,
		})
	}

	c.lastCheck = now

	// 3. Connections / Sockets (optional, gracefully handle errors)
	var connections []model.ConnectionInfo
	conns, _ := netutil.ConnectionsWithContext(ctx, "all")
	for _, conn := range conns {
		connType := "TCP"
		if conn.Type == 2 {
			connType = "UDP"
		}

		connections = append(connections, model.ConnectionInfo{
			Fd:            conn.Fd,
			Family:        fmt.Sprintf("%d", conn.Family),
			Type:          connType,
			LocalAddress:  conn.Laddr.IP,
			LocalPort:     conn.Laddr.Port,
			RemoteAddress: conn.Raddr.IP,
			RemotePort:    conn.Raddr.Port,
			Status:        conn.Status,
			PID:           conn.Pid,
		})
	}

	return &model.NetworkInfo{
		Interfaces:     ifaceInfos,
		IOStats:        ioList,
		TotalBytesSent: totalSent,
		TotalBytesRecv: totalRecv,
		TotalTxRate:    totalTxRate,
		TotalRxRate:    totalRxRate,
		Connections:    connections,
		CollectedAt:    now,
	}, nil
}
