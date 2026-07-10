package tui

import "github.com/charmbracelet/lipgloss"

var (
	theme = struct {
		Primary   lipgloss.Color
		Secondary lipgloss.Color
		Danger    lipgloss.Color
		Warning   lipgloss.Color
		Muted     lipgloss.Color
		Bg        lipgloss.Color
	}{
		Primary:   lipgloss.Color("#7C3AED"),
		Secondary: lipgloss.Color("#06B6D4"),
		Danger:    lipgloss.Color("#EF4444"),
		Warning:   lipgloss.Color("#F59E0B"),
		Muted:     lipgloss.Color("#6B7280"),
		Bg:        lipgloss.Color("#1F2937"),
	}
)

var (
	HeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Secondary)
	CriticalStyle = lipgloss.NewStyle().Foreground(theme.Danger).Bold(true)
	WarnStyle     = lipgloss.NewStyle().Foreground(theme.Warning)
	CursorStyle   = lipgloss.NewStyle().Background(theme.Primary).Foreground(lipgloss.Color("#FFFFFF"))
	NormalStyle   = lipgloss.NewStyle()
	MutedStyle    = lipgloss.NewStyle().Foreground(theme.Muted)
	DetailStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).BorderForeground(theme.Muted).Padding(0, 1)
	HelpStyle     = lipgloss.NewStyle().Foreground(theme.Muted)
	ToastSuccess  = lipgloss.NewStyle().Background(lipgloss.Color("#10B981")).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	ToastError    = lipgloss.NewStyle().Background(theme.Danger).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	ConfirmBox    = lipgloss.NewStyle().Border(lipgloss.DoubleBorder(), true).BorderForeground(theme.Warning).Padding(1, 2)
	TitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(theme.Primary).Padding(0, 1)
)
