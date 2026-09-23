package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/anomaly"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/storage"
	"github.com/watchdog-cli/watchdog/pkg/model"
)

type Tab int

const (
	TabDashboard Tab = iota
	TabProcesses
	TabDiagnostics
	TabAlerts
	TabNetwork
	TabContainers
)

var TabNames = []string{
	"1: Dashboard",
	"2: Processes",
	"3: Diagnostics",
	"4: Alerts",
	"5: Network & Ports",
	"6: Containers",
}

type ProcessSortField int

const (
	SortCPU ProcessSortField = iota
	SortMemory
	SortPID
	SortName
)

type TickMsg time.Time
type DiagResultMsg *model.DiagnosticReport
type KillResultMsg struct {
	PID int
	Err error
}

type Model struct {
	cfg       *config.Config
	collector *collector.Manager
	storage   storage.Storage
	diagEng   *diagnostics.Engine
	alertEng  *alerts.Engine
	anomDet   *anomaly.Detector

	width  int
	height int

	activeTab Tab
	showHelp  bool

	// Live Data
	currentSnapshot *model.SystemSnapshot
	lastDiagReport  *model.DiagnosticReport
	activeAlerts    []model.AlertEvent
	anomalyReport   *model.AnomalyReport
	ports           []model.PortInfo

	// History for sparklines
	cpuHistory     []float64
	memHistory     []float64
	diskReadHist   []float64
	diskWriteHist  []float64
	netRxHist      []float64
	netTxHist      []float64
	historyMaxLens int

	// Process Tab State
	processFilter   textinput.Model
	isFiltering     bool
	filteredProcs   []model.ProcessInfo
	processSort     ProcessSortField
	sortDesc        bool
	selectedProcIdx int
	procScrollOff   int
	confirmKillPID  int

	// Diagnostics Tab State
	selectedDiagIdx int
	diagRunning     bool

	// Alerts Tab State
	selectedAlertIdx int

	// Network Tab State
	selectedPortIdx int
	portScrollOff   int

	// Containers Tab State
	selectedContIdx int

	// Status toast
	statusMessage string
	statusExpiry  time.Time
}

func NewModel(
	cfg *config.Config,
	col *collector.Manager,
	store storage.Storage,
	diagEng *diagnostics.Engine,
	alertEng *alerts.Engine,
	anomDet *anomaly.Detector,
) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter processes (regex or name)..."
	ti.CharLimit = 64
	ti.Width = 30

	return Model{
		cfg:            cfg,
		collector:      col,
		storage:        store,
		diagEng:        diagEng,
		alertEng:       alertEng,
		anomDet:        anomDet,
		activeTab:      TabDashboard,
		processFilter:  ti,
		processSort:    SortCPU,
		sortDesc:       true,
		historyMaxLens: 40,
		confirmKillPID: -1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(m.getTickInterval()),
		m.fetchSnapshotCmd(),
		m.runDiagnosticsCmd(),
	)
}

func (m Model) getTickInterval() time.Duration {
	if m.cfg != nil && m.cfg.RefreshInterval > 0 {
		return m.cfg.RefreshInterval
	}
	return 1 * time.Second
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) fetchSnapshotCmd() tea.Cmd {
	return func() tea.Msg {
		if m.collector == nil {
			return nil
		}
		snap, err := m.collector.CollectAll(context.Background())
		if err != nil {
			return nil
		}
		return snap
	}
}

func (m Model) runDiagnosticsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.diagEng == nil || m.collector == nil {
			return nil
		}
		snap, _ := m.collector.CollectAll(context.Background())
		report := m.diagEng.Run(context.Background(), snap)
		return DiagResultMsg(report)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TickMsg:
		cmds = append(cmds, m.fetchSnapshotCmd())
		cmds = append(cmds, tickCmd(m.getTickInterval()))

	case *model.SystemSnapshot:
		if msg != nil {
			m.currentSnapshot = msg
			m.recordHistory(msg)
			m.updateProcesses()

			// Anomaly evaluation
			if m.anomDet != nil {
				m.anomalyReport = m.anomDet.FeedSnapshot(msg)
			}

			// Alert evaluation
			if m.alertEng != nil {
				_, _, _ = m.alertEng.Evaluate(context.Background(), msg)
				m.activeAlerts = m.alertEng.GetActiveAlerts()
			}

			// Ports
			if msg.Ports != nil {
				m.ports = msg.Ports
			}
		}

	case DiagResultMsg:
		m.lastDiagReport = (*model.DiagnosticReport)(msg)
		m.diagRunning = false

	case KillResultMsg:
		if msg.Err != nil {
			m.SetStatus(fmt.Sprintf("Failed to terminate PID %d: %v", msg.PID, msg.Err), 4*time.Second)
		} else {
			m.SetStatus(fmt.Sprintf("Successfully terminated PID %d", msg.PID), 3*time.Second)
		}
		m.confirmKillPID = -1
		cmds = append(cmds, m.fetchSnapshotCmd())

	case tea.KeyMsg:
		if m.isFiltering {
			switch msg.String() {
			case "enter", "esc":
				m.isFiltering = false
				m.processFilter.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.processFilter, cmd = m.processFilter.Update(msg)
				m.updateProcesses()
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		case "tab":
			m.activeTab = (m.activeTab + 1) % Tab(len(TabNames))
			m.showHelp = false
			return m, nil

		case "shift+tab":
			if m.activeTab == 0 {
				m.activeTab = Tab(len(TabNames) - 1)
			} else {
				m.activeTab--
			}
			m.showHelp = false
			return m, nil

		case "1":
			m.activeTab = TabDashboard
			return m, nil
		case "2":
			m.activeTab = TabProcesses
			return m, nil
		case "3":
			m.activeTab = TabDiagnostics
			return m, nil
		case "4":
			m.activeTab = TabAlerts
			return m, nil
		case "5":
			m.activeTab = TabNetwork
			return m, nil
		case "6":
			m.activeTab = TabContainers
			return m, nil

		case "r":
			m.SetStatus("Refreshing data...", 2*time.Second)
			m.diagRunning = true
			cmds = append(cmds, m.fetchSnapshotCmd(), m.runDiagnosticsCmd())
			return m, tea.Batch(cmds...)

		case "/":
			if m.activeTab == TabProcesses {
				m.isFiltering = true
				m.processFilter.Focus()
				return m, textinput.Blink
			}

		case "c":
			if m.activeTab == TabProcesses {
				m.processSort = SortCPU
				m.sortDesc = true
				m.updateProcesses()
				m.SetStatus("Sorted by CPU % (desc)", 2*time.Second)
			}
		case "m":
			if m.activeTab == TabProcesses {
				m.processSort = SortMemory
				m.sortDesc = true
				m.updateProcesses()
				m.SetStatus("Sorted by Memory % (desc)", 2*time.Second)
			}
		case "p":
			if m.activeTab == TabProcesses {
				m.processSort = SortPID
				m.sortDesc = false
				m.updateProcesses()
				m.SetStatus("Sorted by PID (asc)", 2*time.Second)
			}
		case "n":
			if m.activeTab == TabProcesses {
				m.processSort = SortName
				m.sortDesc = false
				m.updateProcesses()
				m.SetStatus("Sorted by Name", 2*time.Second)
			}

		case "k":
			if m.activeTab == TabProcesses && len(m.filteredProcs) > 0 && m.selectedProcIdx < len(m.filteredProcs) {
				p := m.filteredProcs[m.selectedProcIdx]
				m.confirmKillPID = p.PID
				m.SetStatus(fmt.Sprintf("Press 'y' to terminate PID %d (%s), or 'n' to cancel", p.PID, p.Name), 10*time.Second)
			}

		case "y":
			if m.confirmKillPID > 0 {
				pidToKill := m.confirmKillPID
				m.confirmKillPID = -1
				return m, func() tea.Msg {
					err := collector.KillProcess(pidToKill)
					return KillResultMsg{PID: pidToKill, Err: err}
				}
			}

		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			if m.confirmKillPID > 0 {
				m.confirmKillPID = -1
				m.SetStatus("Process termination cancelled", 2*time.Second)
				return m, nil
			}

		case "up":
			m.navigateUp()
		case "down":
			m.navigateDown()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) navigateUp() {
	switch m.activeTab {
	case TabProcesses:
		if m.selectedProcIdx > 0 {
			m.selectedProcIdx--
			if m.selectedProcIdx < m.procScrollOff {
				m.procScrollOff = m.selectedProcIdx
			}
		}
	case TabDiagnostics:
		if m.selectedDiagIdx > 0 {
			m.selectedDiagIdx--
		}
	case TabAlerts:
		if m.selectedAlertIdx > 0 {
			m.selectedAlertIdx--
		}
	case TabNetwork:
		if m.selectedPortIdx > 0 {
			m.selectedPortIdx--
			if m.selectedPortIdx < m.portScrollOff {
				m.portScrollOff = m.selectedPortIdx
			}
		}
	case TabContainers:
		if m.selectedContIdx > 0 {
			m.selectedContIdx--
		}
	}
}

func (m *Model) navigateDown() {
	switch m.activeTab {
	case TabProcesses:
		if m.selectedProcIdx < len(m.filteredProcs)-1 {
			m.selectedProcIdx++
			maxVisible := m.height - 12
			if maxVisible < 5 {
				maxVisible = 5
			}
			if m.selectedProcIdx >= m.procScrollOff+maxVisible {
				m.procScrollOff = m.selectedProcIdx - maxVisible + 1
			}
		}
	case TabDiagnostics:
		if m.lastDiagReport != nil && m.selectedDiagIdx < len(m.lastDiagReport.Results)-1 {
			m.selectedDiagIdx++
		}
	case TabAlerts:
		if m.selectedAlertIdx < len(m.activeAlerts)-1 {
			m.selectedAlertIdx++
		}
	case TabNetwork:
		if m.selectedPortIdx < len(m.ports)-1 {
			m.selectedPortIdx++
			maxVisible := m.height - 10
			if maxVisible < 5 {
				maxVisible = 5
			}
			if m.selectedPortIdx >= m.portScrollOff+maxVisible {
				m.portScrollOff = m.selectedPortIdx - maxVisible + 1
			}
		}
	case TabContainers:
		totalContainers := 0
		if m.currentSnapshot != nil && m.currentSnapshot.Docker != nil {
			totalContainers = len(m.currentSnapshot.Docker.Containers)
		}
		if m.selectedContIdx < totalContainers-1 {
			m.selectedContIdx++
		}
	}
}

func (m *Model) recordHistory(s *model.SystemSnapshot) {
	if s == nil {
		return
	}
	if s.CPU != nil {
		m.cpuHistory = append(m.cpuHistory, s.CPU.OverallUsage)
		if len(m.cpuHistory) > m.historyMaxLens {
			m.cpuHistory = m.cpuHistory[1:]
		}
	}
	if s.Memory != nil {
		m.memHistory = append(m.memHistory, s.Memory.UsedPercent)
		if len(m.memHistory) > m.historyMaxLens {
			m.memHistory = m.memHistory[1:]
		}
	}
	if s.Disk != nil {
		var rRate, wRate float64
		for _, io := range s.Disk.IOCounters {
			rRate += io.ReadRate
			wRate += io.WriteRate
		}
		m.diskReadHist = append(m.diskReadHist, rRate)
		m.diskWriteHist = append(m.diskWriteHist, wRate)
		if len(m.diskReadHist) > m.historyMaxLens {
			m.diskReadHist = m.diskReadHist[1:]
			m.diskWriteHist = m.diskWriteHist[1:]
		}
	}
	if s.Network != nil {
		m.netRxHist = append(m.netRxHist, s.Network.TotalRxRate)
		m.netTxHist = append(m.netTxHist, s.Network.TotalTxRate)
		if len(m.netRxHist) > m.historyMaxLens {
			m.netRxHist = m.netRxHist[1:]
			m.netTxHist = m.netTxHist[1:]
		}
	}
}

func (m *Model) updateProcesses() {
	if m.currentSnapshot == nil || m.currentSnapshot.Processes == nil {
		m.filteredProcs = nil
		return
	}

	procs := m.currentSnapshot.Processes.Processes
	query := strings.ToLower(strings.TrimSpace(m.processFilter.Value()))

	var filtered []model.ProcessInfo
	for _, p := range procs {
		if query == "" || strings.Contains(strings.ToLower(p.Name), query) || strings.Contains(strings.ToLower(p.CommandLine), query) || fmt.Sprintf("%d", p.PID) == query {
			filtered = append(filtered, p)
		}
	}

	// Sort
	sort.Slice(filtered, func(i, j int) bool {
		switch m.processSort {
		case SortCPU:
			if m.sortDesc {
				return filtered[i].CPUPercent > filtered[j].CPUPercent
			}
			return filtered[i].CPUPercent < filtered[j].CPUPercent
		case SortMemory:
			if m.sortDesc {
				return filtered[i].MemoryPercent > filtered[j].MemoryPercent
			}
			return filtered[i].MemoryPercent < filtered[j].MemoryPercent
		case SortPID:
			if m.sortDesc {
				return filtered[i].PID > filtered[j].PID
			}
			return filtered[i].PID < filtered[j].PID
		case SortName:
			if m.sortDesc {
				return strings.ToLower(filtered[i].Name) > strings.ToLower(filtered[j].Name)
			}
			return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		}
		return false
	})

	m.filteredProcs = filtered
	if m.selectedProcIdx >= len(filtered) && len(filtered) > 0 {
		m.selectedProcIdx = len(filtered) - 1
	}
}

func (m *Model) SetStatus(msg string, duration time.Duration) {
	m.statusMessage = msg
	m.statusExpiry = time.Now().Add(duration)
}
