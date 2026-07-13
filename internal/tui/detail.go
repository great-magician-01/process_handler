package tui

import (
	"fmt"
	"strings"

	"github.com/great-magician-01/process_handler/internal/procinfo"
)

func renderDetail(m Model) string {
	if len(m.Rows) == 0 || m.Cursor >= len(m.Rows) {
		return MutedStyle.Render("No process selected")
	}

	r := m.Rows[m.Cursor]

	lines := []string{
		fmt.Sprintf("PID:        %d", r.PID),
		fmt.Sprintf("Name:       %s", r.Proc.Name),
		fmt.Sprintf("Port:       %s:%d", r.LocalAddr, r.LocalPort),
	}
	if r.Proc.ExePath != "" {
		lines = append(lines, fmt.Sprintf("ExePath:    %s", r.Proc.ExePath))
	}
	if r.Proc.CmdLine != "" {
		lines = append(lines, fmt.Sprintf("CmdLine:    %s", r.Proc.CmdLine))
	}
	lines = append(lines,
		fmt.Sprintf("Parent:     %d (%s)", r.Proc.ParentPID, r.Proc.ParentName),
		fmt.Sprintf("User:       %s", r.Proc.Username),
	)

	critLabel := "no"
	switch r.Critical {
	case procinfo.CritBlocked:
		critLabel = CriticalStyle.Render("BLOCKED (critical system process)")
	case procinfo.CritWarn:
		critLabel = WarnStyle.Render("WARN (system-owned)")
	}
	lines = append(lines, fmt.Sprintf("Critical:   %s", critLabel))

	return DetailStyle.Width(m.Width - 4).Render(strings.Join(lines, "\n"))
}
