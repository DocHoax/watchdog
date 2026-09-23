package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// UI Theme & Palette definitions
var (
	// Colors
	ColorPrimary   = lipgloss.Color("#6366F1") // Indigo
	ColorSecondary = lipgloss.Color("#3B82F6") // Blue
	ColorSuccess   = lipgloss.Color("#10B981") // Emerald
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorDanger    = lipgloss.Color("#EF4444") // Red
	ColorInfo      = lipgloss.Color("#06B6D4") // Cyan
	ColorDarkBg    = lipgloss.Color("#0F172A") // Slate 900
	ColorCardBg    = lipgloss.Color("#1E293B") // Slate 800
	ColorBorder    = lipgloss.Color("#334155") // Slate 700
	ColorText      = lipgloss.Color("#F8FAFC") // Slate 50
	ColorMuted     = lipgloss.Color("#94A3B8") // Slate 400
	ColorHighlight = lipgloss.Color("#8B5CF6") // Purple

	// Text Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 1)

	SubTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorInfo)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWarning)

	DangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger)

	InfoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorInfo)

	// Card & Container Styles
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Background(ColorCardBg).
			Padding(0, 1)

	CardActiveStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Background(ColorCardBg).
			Padding(0, 1)

	// Tabs
	TabStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 2)

	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 2)

	TabGapStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	// Badges & Pills
	PassBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#064E3B")).
			Background(ColorSuccess).
			Padding(0, 1)

	WarnBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#78350F")).
			Background(ColorWarning).
			Padding(0, 1)

	CritBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7F1D1D")).
			Background(ColorDanger).
			Padding(0, 1)

	// Header & Footer
	HeaderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			MarginBottom(1)

	FooterStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(ColorBorder).
			Foreground(ColorMuted).
			Padding(0, 1).
			MarginTop(1)

	// Search & Input
	SearchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	// Table Styles
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorText).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(ColorBorder)

	TableRowSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorText).
				Background(lipgloss.Color("#312E81")) // Indigo 900
)
