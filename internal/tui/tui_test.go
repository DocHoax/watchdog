package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

func TestTUI_Helpers(t *testing.T) {
	t.Run("FormatBytes", func(t *testing.T) {
		assert.Equal(t, "500 B", FormatBytes(500))
		assert.Equal(t, "1.0 KiB", FormatBytes(1024))
		assert.Equal(t, "1.5 MiB", FormatBytes(1024*1024+512*1024))
		assert.Equal(t, "2.0 GiB", FormatBytes(2*1024*1024*1024))
	})

	t.Run("FormatRate", func(t *testing.T) {
		assert.Equal(t, "500 B/s", FormatRate(500))
		assert.Equal(t, "1.0 KiB/s", FormatRate(1024))
		assert.Equal(t, "2.0 MiB/s", FormatRate(2*1024*1024))
		assert.Equal(t, "1.0 GiB/s", FormatRate(1024*1024*1024))
	})

	t.Run("FormatDuration", func(t *testing.T) {
		assert.Equal(t, "45s", FormatDuration(45*time.Second))
		assert.Equal(t, "12m 30s", FormatDuration(12*time.Minute+30*time.Second))
		assert.Equal(t, "5h 15m 0s", FormatDuration(5*time.Hour+15*time.Minute))
		assert.Equal(t, "3d 4h 0m", FormatDuration(76*time.Hour))
	})

	t.Run("TruncateString", func(t *testing.T) {
		assert.Equal(t, "hello", TruncateString("hello", 10))
		assert.Equal(t, "he...", TruncateString("hello world", 5))
		assert.Equal(t, "ab", TruncateString("abcdef", 2))
	})

	t.Run("RenderProgressBar", func(t *testing.T) {
		bar := RenderProgressBar(50.0, 20)
		assert.NotEmpty(t, bar)
		assert.Contains(t, bar, "█")
	})

	t.Run("RenderSparkline", func(t *testing.T) {
		spark := RenderSparkline([]float64{10, 20, 50, 80, 100}, 100.0)
		assert.NotEmpty(t, spark)
		assert.Equal(t, 5, len([]rune(spark)))
	})

	t.Run("Badges", func(t *testing.T) {
		assert.NotEmpty(t, StatusBadge(model.StatusPass))
		assert.NotEmpty(t, StatusBadge(model.StatusWarning))
		assert.NotEmpty(t, StatusBadge(model.StatusFail))
		assert.NotEmpty(t, SeverityBadge(model.SeverityCritical))
		assert.NotEmpty(t, SeverityBadge(model.SeverityWarning))
		assert.NotEmpty(t, SeverityBadge(model.SeverityInfo))
	})
}

func TestTUI_ModelInitAndUpdate(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewModel(cfg, nil, nil, nil, nil, nil)
	m.width = 120
	m.height = 40

	// Initial check
	assert.Equal(t, TabDashboard, m.activeTab)
	assert.False(t, m.showHelp)

	// Window resize
	updatedModel, cmd := m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	assert.Nil(t, cmd)
	m = updatedModel.(Model)
	assert.Equal(t, 140, m.width)
	assert.Equal(t, 50, m.height)

	// Toggle help
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updatedModel.(Model)
	assert.True(t, m.showHelp)
	rendered := m.View()
	assert.Contains(t, rendered, "WATCHDOG - INTERACTIVE KEYBOARD SHORTCUTS")

	// Close help
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(Model)
	assert.False(t, m.showHelp)

	// Switch tabs via numbers
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updatedModel.(Model)
	assert.Equal(t, TabProcesses, m.activeTab)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updatedModel.(Model)
	assert.Equal(t, TabDiagnostics, m.activeTab)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = updatedModel.(Model)
	assert.Equal(t, TabAlerts, m.activeTab)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m = updatedModel.(Model)
	assert.Equal(t, TabNetwork, m.activeTab)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	m = updatedModel.(Model)
	assert.Equal(t, TabContainers, m.activeTab)

	// Switch tabs via Tab / Shift+Tab
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updatedModel.(Model)
	assert.Equal(t, TabDashboard, m.activeTab)

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updatedModel.(Model)
	assert.Equal(t, TabContainers, m.activeTab)
}

func TestTUI_SnapshotAndProcessManagement(t *testing.T) {
	cfg := config.DefaultConfig()
	m := NewModel(cfg, nil, nil, nil, nil, nil)
	m.width = 120
	m.height = 40

	snap := &model.SystemSnapshot{
		Timestamp: time.Now(),
		System: &model.SystemInfo{
			Hostname:        "test-node",
			OS:              "linux",
			Platform:        "ubuntu",
			PlatformVersion: "22.04",
			Uptime:          7200 * time.Second,
			BootTime:        time.Now().Add(-2 * time.Hour),
		},
		CPU: &model.CPUInfo{
			OverallUsage: 45.2,
			LogicalCores: 8,
			LoadAverage:  model.LoadAvg{Load1: 1.5, Load5: 1.2, Load15: 0.9},
		},
		Memory: &model.MemoryInfo{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      8 * 1024 * 1024 * 1024,
			AvailableBytes: 8 * 1024 * 1024 * 1024,
			UsedPercent:    50.0,
		},
		Disk: &model.DiskInfo{
			TotalBytes: 500 * 1024 * 1024 * 1024,
			UsedBytes:  250 * 1024 * 1024 * 1024,
			FreeBytes:  250 * 1024 * 1024 * 1024,
			UsedPercent: 50.0,
			IOCounters: []model.DiskIOCounters{
				{Name: "sda", ReadRate: 1024 * 1024, WriteRate: 2 * 1024 * 1024},
			},
		},
		Network: &model.NetworkInfo{
			TotalRxRate: 500 * 1024,
			TotalTxRate: 200 * 1024,
			Interfaces: []model.NetworkInterfaceInfo{
				{Name: "eth0", HardwareAddr: "00:11:22:33:44:55", Addrs: []string{"192.168.1.100"}},
			},
			IOStats: []model.NetworkIOInfo{
				{Name: "eth0", RxRate: 500 * 1024, TxRate: 200 * 1024},
			},
		},
		Processes: &model.ProcessSummary{
			TotalCount: 3,
			Processes: []model.ProcessInfo{
				{PID: 100, Name: "nginx", CPUPercent: 12.5, MemoryPercent: 4.2, MemoryRSS: 100 * 1024 * 1024, CommandLine: "nginx: master"},
				{PID: 200, Name: "postgres", CPUPercent: 35.0, MemoryPercent: 15.0, MemoryRSS: 500 * 1024 * 1024, CommandLine: "postgres: writer"},
				{PID: 300, Name: "redis", CPUPercent: 5.0, MemoryPercent: 2.0, MemoryRSS: 50 * 1024 * 1024, CommandLine: "redis-server"},
			},
		},
		Ports: []model.PortInfo{
			{Port: 80, Protocol: "tcp", BindAddress: "0.0.0.0", State: "LISTEN", PID: 100, ProcessName: "nginx"},
			{Port: 5432, Protocol: "tcp", BindAddress: "127.0.0.1", State: "LISTEN", PID: 200, ProcessName: "postgres"},
		},
		Docker: &model.DockerSummary{
			Containers: []model.DockerContainer{
				{ID: "c1234567890a", Names: []string{"/web-app"}, Image: "nginx:alpine", Status: "running", Ports: []string{"80/tcp"}},
			},
		},
	}

	// Ingest snapshot
	updatedModel, _ := m.Update(snap)
	m = updatedModel.(Model)

	require.NotNil(t, m.currentSnapshot)
	assert.Equal(t, 3, len(m.filteredProcs))
	assert.Equal(t, int32(200), m.filteredProcs[0].PID) // Sorted by CPU % desc (postgres 35.0%)

	// Test Sorting Keys
	m.activeTab = TabProcesses
	// Sort by Memory (m)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = updatedModel.(Model)
	assert.Equal(t, SortMemory, m.processSort)
	assert.Equal(t, int32(200), m.filteredProcs[0].PID) // postgres has highest mem

	// Sort by PID (p)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updatedModel.(Model)
	assert.Equal(t, SortPID, m.processSort)
	assert.Equal(t, int32(100), m.filteredProcs[0].PID) // nginx (100) is lowest PID

	// Sort by Name (n)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updatedModel.(Model)
	assert.Equal(t, SortName, m.processSort)
	assert.Equal(t, "nginx", m.filteredProcs[0].Name)

	// Test Navigation
	m.navigateDown()
	assert.Equal(t, 1, m.selectedProcIdx)
	m.navigateUp()
	assert.Equal(t, 0, m.selectedProcIdx)

	// Test Process Kill Initiation
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updatedModel.(Model)
	assert.Equal(t, 100, m.confirmKillPID)
	assert.Contains(t, m.statusMessage, "Press 'y' to terminate PID 100")

	// Cancel kill with esc
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(Model)
	assert.Equal(t, -1, m.confirmKillPID)
	assert.Contains(t, m.statusMessage, "cancelled")

	// Verify all tab views render without crashing
	for _, tab := range []Tab{TabDashboard, TabProcesses, TabDiagnostics, TabAlerts, TabNetwork, TabContainers} {
		m.activeTab = tab
		rendered := m.View()
		assert.NotEmpty(t, rendered)
	}
}
