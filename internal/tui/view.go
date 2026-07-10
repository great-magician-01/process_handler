package tui

import (
	"fmt"
	"strings"

	"process_handler/internal/procinfo"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width < 60 {
		return MutedStyle.Render("Terminal too narrow. Please widen to at least 60 columns.\n") +
			HelpStyle.Render("Press q to quit")
	}

	var sections []string

	title := TitleStyle.Render("Process Handler")
	if m.AutoRefreshOn {
		title += MutedStyle.Render("  [auto-refresh: 3s]")
	}
	sections = append(sections, title)

	if m.Error != "" {
		sections = append(sections, ToastError.Render(m.Error))
	}

	switch m.State {
	case stateConfirm:
		if len(m.Rows) > 0 && m.Cursor < len(m.Rows) {
			sections = append(sections, renderConfirmDialog(&m))
		}
	case stateKilling:
		sections = append(sections, MutedStyle.Render(fmt.Sprintf("Killing PID %d...", m.ConfirmPID)))
	default:
		if len(m.AllRows) == 0 {
			sections = append(sections, MutedStyle.Render("Loading processes..."))
		} else if len(m.Rows) == 0 {
			sections = append(sections, MutedStyle.Render("No processes match filter \""+m.Filter+"\""))
		} else {
			sections = append(sections, renderTable(m))
			sections = append(sections, renderDetail(m))
		}
	}

	help := helpText[m.State]
	if m.State == stateFilter {
		help = HelpStyle.Render("filter: \""+m.Filter+"\"") + "  " + helpText[m.State]
	}
	sections = append(sections, HelpStyle.Render(help))

	if m.Toast != "" {
		if m.ToastError {
			sections = append(sections, ToastError.Render(m.Toast))
		} else {
			sections = append(sections, ToastSuccess.Render(m.Toast))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderConfirmDialog(m *Model) string {
	r := m.Rows[m.Cursor]

	var lines []string
	lines = append(lines, fmt.Sprintf("Terminate PID %d (%s) ?", r.PID, r.Proc.Name))
	lines = append(lines, fmt.Sprintf("Port: %s:%d", r.LocalAddr, r.LocalPort))

	if r.Critical == procinfo.CritBlocked {
		lines = append(lines, "")
		lines = append(lines, CriticalStyle.Render("This process CANNOT be terminated. It is a critical system process."))
		lines = append(lines, HelpStyle.Render("Press esc to go back"))
	} else {
		if r.Critical == procinfo.CritWarn {
			lines = append(lines, "")
			lines = append(lines, WarnStyle.Render("Warning: This appears to be a system-owned process."))
			lines = append(lines, WarnStyle.Render("Terminating it may cause system instability."))
			lines = append(lines, fmt.Sprintf("Type the PID number to confirm: %d", r.PID))
			if m.ConfirmInput != "" {
				lines = append(lines, fmt.Sprintf("Input: %s", m.ConfirmInput))
			}
		} else {
			lines = append(lines, fmt.Sprintf("User: %s", r.Proc.Username))
			if r.Proc.ExePath != "" {
				lines = append(lines, fmt.Sprintf("Path: %s", r.Proc.ExePath))
			}
			lines = append(lines, "")
			lines = append(lines, HelpStyle.Render("Press y to confirm, n/esc to cancel"))
		}
	}

	content := strings.Join(lines, "\n")
	width := m.Width - 4
	if width > 60 {
		width = 60
	}
	return ConfirmBox.Width(width).Render(content)
}
