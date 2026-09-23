package model

import "time"

// NetworkInterfaceInfo represents a single network interface.
type NetworkInterfaceInfo struct {
	Index        int      `json:"index" yaml:"index"`
	MTU          int      `json:"mtu" yaml:"mtu"`
	Name         string   `json:"name" yaml:"name"`
	HardwareAddr string   `json:"hardware_addr" yaml:"hardware_addr"`
	Flags        []string `json:"flags" yaml:"flags"`
	Addrs        []string `json:"addrs" yaml:"addrs"`
	IsUp         bool     `json:"is_up" yaml:"is_up"`
	IsLoopback   bool     `json:"is_loopback" yaml:"is_loopback"`
}

// NetworkIOInfo represents I/O and packet stats for an interface.
type NetworkIOInfo struct {
	Name         string    `json:"name" yaml:"name"`
	BytesSent    uint64    `json:"bytes_sent" yaml:"bytes_sent"`
	BytesRecv    uint64    `json:"bytes_recv" yaml:"bytes_recv"`
	PacketsSent  uint64    `json:"packets_sent" yaml:"packets_sent"`
	PacketsRecv  uint64    `json:"packets_recv" yaml:"packets_recv"`
	ErrIn        uint64    `json:"err_in" yaml:"err_in"`
	ErrOut       uint64    `json:"err_out" yaml:"err_out"`
	DropIn       uint64    `json:"drop_in" yaml:"drop_in"`
	DropOut      uint64    `json:"drop_out" yaml:"drop_out"`
	TxRate       float64   `json:"tx_bytes_sec" yaml:"tx_bytes_sec"` // bytes/sec
	RxRate       float64   `json:"rx_bytes_sec" yaml:"rx_bytes_sec"` // bytes/sec
	TxPacketsSec float64   `json:"tx_packets_sec" yaml:"tx_packets_sec"`
	RxPacketsSec float64   `json:"rx_packets_sec" yaml:"rx_packets_sec"`
	Timestamp    time.Time `json:"timestamp" yaml:"timestamp"`
}

// ConnectionInfo represents an active socket connection.
type ConnectionInfo struct {
	Fd            uint32 `json:"fd,omitempty" yaml:"fd,omitempty"`
	Family        string `json:"family" yaml:"family"`
	Type          string `json:"type" yaml:"type"` // TCP, UDP
	LocalAddress  string `json:"local_address" yaml:"local_address"`
	LocalPort     uint32 `json:"local_port" yaml:"local_port"`
	RemoteAddress string `json:"remote_address" yaml:"remote_address"`
	RemotePort    uint32 `json:"remote_port" yaml:"remote_port"`
	Status        string `json:"status" yaml:"status"` // LISTEN, ESTABLISHED, etc.
	PID           int32  `json:"pid" yaml:"pid"`
	ProcessName   string `json:"process_name,omitempty" yaml:"process_name,omitempty"`
}

// NetworkInfo represents consolidated network data.
type NetworkInfo struct {
	Interfaces     []NetworkInterfaceInfo `json:"interfaces" yaml:"interfaces"`
	IOStats        []NetworkIOInfo        `json:"io_stats" yaml:"io_stats"`
	TotalBytesSent uint64                 `json:"total_bytes_sent" yaml:"total_bytes_sent"`
	TotalBytesRecv uint64                 `json:"total_bytes_recv" yaml:"total_bytes_recv"`
	TotalTxRate    float64                `json:"total_tx_bytes_sec" yaml:"total_tx_bytes_sec"`
	TotalRxRate    float64                `json:"total_rx_bytes_sec" yaml:"total_rx_bytes_sec"`
	Connections    []ConnectionInfo       `json:"connections,omitempty" yaml:"connections,omitempty"`
	CollectedAt    time.Time              `json:"collected_at" yaml:"collected_at"`
}
