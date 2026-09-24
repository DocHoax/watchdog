package tui

import (
	"github.com/DocHoax/watchdog/internal/alerts"
	"github.com/DocHoax/watchdog/internal/anomaly"
	"github.com/DocHoax/watchdog/internal/collector"
	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/internal/diagnostics"
	"github.com/DocHoax/watchdog/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
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
