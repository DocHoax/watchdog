package fleet

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// DefaultNodeIDPath returns the default filesystem path used for persisting node ID.
func DefaultNodeIDPath() string {
	if envPath := os.Getenv("WATCHDOG_NODE_ID_PATH"); envPath != "" {
		return envPath
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), ".watchdog_node_id")
	}
	return filepath.Join(home, ".watchdog", "node_id")
}

// GetOrGenerateNodeID retrieves an existing persisted node ID or generates a new cryptographically secure UUID.
func GetOrGenerateNodeID(customPath string) (string, error) {
	// 1. Environment variable override
	if envID := strings.TrimSpace(os.Getenv("WATCHDOG_NODE_ID")); envID != "" {
		return envID, nil
	}

	targetPath := customPath
	if targetPath == "" {
		targetPath = DefaultNodeIDPath()
	}

	// 2. Read from persistent file if it exists
	if data, err := os.ReadFile(targetPath); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	// 3. Generate new UUID v4
	newID := uuid.New().String()

	// 4. Attempt to persist to disk
	if err := SaveNodeID(targetPath, newID); err != nil {
		// Non-fatal if filesystem is read-only, but return the generated ID
		return newID, nil
	}

	return newID, nil
}

// SaveNodeID writes the node ID to the designated file path with restricted permissions.
func SaveNodeID(path, id string) error {
	if path == "" || id == "" {
		return fmt.Errorf("invalid path or node ID")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(strings.TrimSpace(id)+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write node ID file %s: %w", path, err)
	}

	return nil
}

// DiscoverNodeIdentity collects system metadata, network interfaces, and hardware specs for the local node.
func DiscoverNodeIdentity(ctx context.Context, nodeID string, version string, customTags map[string]string) (*model.NodeIdentity, error) {
	if nodeID == "" {
		var err error
		nodeID, err = GetOrGenerateNodeID("")
		if err != nil {
			nodeID = uuid.New().String()
		}
	}

	hostname, _ := os.Hostname()
	osName := runtime.GOOS
	arch := runtime.GOARCH
	cpuCores := runtime.NumCPU()

	var platform, platformVer, kernelVer string
	if hostInfo, err := host.InfoWithContext(ctx); err == nil && hostInfo != nil {
		if hostInfo.Hostname != "" {
			hostname = hostInfo.Hostname
		}
		if hostInfo.OS != "" {
			osName = hostInfo.OS
		}
		platform = hostInfo.Platform
		platformVer = hostInfo.PlatformVersion
		kernelVer = hostInfo.KernelVersion
	}

	var totalMem uint64
	if vMem, err := mem.VirtualMemoryWithContext(ctx); err == nil && vMem != nil {
		totalMem = vMem.Total
	}

	ips, macs := discoverNetworkAddresses()

	tags := make(map[string]string)
	for k, v := range customTags {
		tags[k] = v
	}

	return &model.NodeIdentity{
		NodeID:       nodeID,
		Hostname:     hostname,
		OS:           osName,
		Platform:     platform,
		PlatformVer:  platformVer,
		Arch:         arch,
		KernelVer:    kernelVer,
		Version:      version,
		CPUCores:     cpuCores,
		TotalMemory:  totalMem,
		IPAddresses:  ips,
		MACAddresses: macs,
		Tags:         tags,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func discoverNetworkAddresses() (ips []string, macs []string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil
	}

	ipSet := make(map[string]struct{})
	macSet := make(map[string]struct{})

	for _, iface := range ifaces {
		// Ignore loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		mac := iface.HardwareAddr.String()
		if mac != "" {
			if _, exists := macSet[mac]; !exists {
				macSet[mac] = struct{}{}
				macs = append(macs, mac)
			}
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			ipStr := ip.String()
			if _, exists := ipSet[ipStr]; !exists {
				ipSet[ipStr] = struct{}{}
				ips = append(ips, ipStr)
			}
		}
	}

	return ips, macs
}
