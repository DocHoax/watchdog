package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/watchdog-cli/watchdog/pkg/model"
)

func TestSystemCollector(t *testing.T) {
	c := NewSystemCollector()
	if c.Name() != "system" {
		t.Fatalf("expected collector name 'system', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sys, err := c.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting system info: %v", err)
	}

	if sys.Hostname == "" {
		t.Errorf("expected non-empty hostname")
	}
	if sys.OS == "" {
		t.Errorf("expected non-empty OS")
	}
	if sys.KernelArch == "" {
		t.Errorf("expected non-empty KernelArch")
	}

	res, err := c.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
	if _, ok := res.(*model.SystemInfo); !ok {
		t.Errorf("expected *model.SystemInfo from Collect(), got %T", res)
	}
}

func TestCPUCollector(t *testing.T) {
	c := NewCPUCollector()
	if c.Name() != "cpu" {
		t.Fatalf("expected collector name 'cpu', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cpuInfo, err := c.GetCPUInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting CPU info: %v", err)
	}

	if cpuInfo.PhysicalCores <= 0 && cpuInfo.LogicalCores <= 0 {
		t.Errorf("expected positive core count, got phys=%d, log=%d", cpuInfo.PhysicalCores, cpuInfo.LogicalCores)
	}
}

func TestMemoryCollector(t *testing.T) {
	c := NewMemoryCollector()
	if c.Name() != "memory" {
		t.Fatalf("expected collector name 'memory', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memInfo, err := c.GetMemoryInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting memory info: %v", err)
	}

	if memInfo.TotalBytes == 0 {
		t.Errorf("expected non-zero total memory")
	}
	if memInfo.UsedPercent < 0 || memInfo.UsedPercent > 100 {
		t.Errorf("invalid memory percent: %f", memInfo.UsedPercent)
	}
}

func TestDiskCollector(t *testing.T) {
	c := NewDiskCollector()
	if c.Name() != "disk" {
		t.Fatalf("expected collector name 'disk', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diskInfo, err := c.GetDiskInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting disk info: %v", err)
	}

	if len(diskInfo.Partitions) == 0 {
		t.Logf("no partitions returned (could happen in restricted environments)")
	}
}

func TestNetworkCollector(t *testing.T) {
	c := NewNetworkCollector()
	if c.Name() != "network" {
		t.Fatalf("expected collector name 'network', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	netInfo, err := c.GetNetworkInfo(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting network info: %v", err)
	}

	if len(netInfo.Interfaces) == 0 {
		t.Logf("no interfaces returned")
	}
}

func TestProcessCollector(t *testing.T) {
	c := NewProcessCollector()
	if c.Name() != "process" {
		t.Fatalf("expected collector name 'process', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	procSummary, err := c.GetProcesses(ctx, 10, "cpu", "")
	if err != nil {
		t.Fatalf("unexpected error collecting processes: %v", err)
	}

	if procSummary.TotalCount <= 0 {
		t.Errorf("expected positive process count, got %d", procSummary.TotalCount)
	}

	// Test getting current process details
	currentPID := int32(os.Getpid())
	procDetail, err := c.GetProcessDetails(ctx, currentPID)
	if err != nil {
		t.Fatalf("failed to get details for current process: %v", err)
	}
	if procDetail.PID != currentPID {
		t.Errorf("expected PID %d, got %d", currentPID, procDetail.PID)
	}

	// Test process tree
	tree, err := c.GetProcessTree(ctx)
	if err != nil {
		t.Fatalf("failed to build process tree: %v", err)
	}
	if tree == nil {
		t.Errorf("expected non-nil process tree")
	}
}

func TestPortCollector(t *testing.T) {
	c := NewPortCollector()
	if c.Name() != "port" {
		t.Fatalf("expected collector name 'port', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ports, err := c.GetListeningPorts(ctx)
	if err != nil {
		t.Fatalf("unexpected error collecting ports: %v", err)
	}
	t.Logf("collected %d listening ports", len(ports))
}

func TestDockerCollectorGracefulFallback(t *testing.T) {
	c := NewDockerCollector()
	if c.Name() != "docker" {
		t.Fatalf("expected collector name 'docker', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dockerInfo, err := c.GetDockerSummary(ctx)
	if err != nil {
		t.Fatalf("unexpected fatal error from docker collector: %v", err)
	}
	if dockerInfo == nil {
		t.Fatalf("expected non-nil dockerInfo")
	}
	t.Logf("docker available: %v, total containers: %d", dockerInfo.Available, dockerInfo.ContainersTotal)
}

func TestKubernetesCollectorGracefulFallback(t *testing.T) {
	c := NewKubernetesCollector()
	if c.Name() != "kubernetes" {
		t.Fatalf("expected collector name 'kubernetes', got '%s'", c.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	k8sInfo, err := c.GetClusterSummary(ctx)
	if err != nil {
		t.Fatalf("unexpected fatal error from k8s collector: %v", err)
	}
	if k8sInfo == nil {
		t.Fatalf("expected non-nil k8sInfo")
	}
	t.Logf("k8s available: %v, total nodes: %d", k8sInfo.Available, k8sInfo.TotalNodes)
}

func TestCollectorManager(t *testing.T) {
	mgr := NewManager()

	mgr.Register(NewSystemCollector())
	mgr.Register(NewCPUCollector())
	mgr.Register(NewMemoryCollector())
	mgr.Register(NewDiskCollector())
	mgr.Register(NewNetworkCollector())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snapshot, err := mgr.CollectAll(ctx)
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}

	if snapshot.System == nil {
		t.Errorf("expected System in snapshot")
	}
	if snapshot.CPU == nil {
		t.Errorf("expected CPU in snapshot")
	}
	if snapshot.Memory == nil {
		t.Errorf("expected Memory in snapshot")
	}
	if snapshot.Disk == nil {
		t.Errorf("expected Disk in snapshot")
	}
	if snapshot.Network == nil {
		t.Errorf("expected Network in snapshot")
	}
}
