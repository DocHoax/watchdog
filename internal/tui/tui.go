package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/watchdog-cli/watchdog/internal/alerts"
	"github.com/watchdog-cli/watchdog/internal/anomaly"
	"github.com/watchdog-cli/watchdog/internal/collector"
	"github.com/watchdog-cli/watchdog/internal/config"
	"github.com/watchdog-cli/watchdog/internal/diagnostics"
	"github.com/watchdog-cli/watchdog/internal/storage"
)

// Run initializes and executes the interactive Bubbletea TUI application.
func Run(
	cfg *config.Config,
	col *collector.Manager,
	store storage.Storage,
	diagEng *diagnostics.Engine,
	alertEng *alerts.Engine,
	anomDet *anomaly.Detector,
) error {
	m := NewModel(cfg, col, store, diagEng, alertEng, anomDet)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
