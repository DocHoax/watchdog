package util

import (
	"context"
	"fmt"
	"net"
	"time"
)

// CheckDNSResolution verifies if DNS resolution works and measures resolution latency.
func CheckDNSResolution(ctx context.Context, host string) (time.Duration, error) {
	if host == "" {
		host = "cloudflare.com"
	}
	start := time.Now()
	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctx, host)
	latency := time.Since(start)
	if err != nil {
		return latency, fmt.Errorf("failed to resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return latency, fmt.Errorf("no IP addresses returned for %s", host)
	}
	return latency, nil
}

// CheckTCPConnectivity verifies TCP connectivity to a host:port and measures latency.
func CheckTCPConnectivity(ctx context.Context, address string, timeout time.Duration) (time.Duration, error) {
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", address)
	latency := time.Since(start)
	if err != nil {
		return latency, fmt.Errorf("TCP connect failed to %s: %w", address, err)
	}
	_ = conn.Close()
	return latency, nil
}
