package tui

import (
	"fmt"
	"strings"

	"github.com/great-magician-01/process_handler/internal/procinfo"
)

func renderTable(m Model) string {
	if m.Width < 60 {
		return MutedStyle.Render("Window too narrow. Please widen terminal.")
	}

	pidW := 8
	portW := 25
	userW := 18
	remainder := m.Width - pidW - portW - userW - 4
	nameW := remainder / 2
	if nameW < 10 {
		nameW = 10
	}
	parentW := remainder - nameW
	if parentW < 10 {
		parentW = 10
	}

	hdr := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
		pidW, "PID", nameW, "NAME", portW, "PORT", parentW, "PARENT", userW, "USER")
	sep := strings.Repeat("─", pidW+nameW+portW+parentW+userW+4)

	var sb strings.Builder
	sb.WriteString(HeaderStyle.Render(hdr))
	sb.WriteByte('\n')
	sb.WriteString(MutedStyle.Render(sep))
	sb.WriteByte('\n')

	if len(m.Rows) == 0 {
		sb.WriteString(MutedStyle.Render("No processes found"))
		return sb.String()
	}

	tableHeight := m.Height - 12
	if tableHeight < 1 {
		tableHeight = 1
	}
	start := m.Cursor / tableHeight * tableHeight
	end := start + tableHeight
	if end > len(m.Rows) {
		end = len(m.Rows)
	}

	for i := start; i < end; i++ {
		r := m.Rows[i]
		line := formatRow(r, pidW, nameW, portW, parentW, userW)

		if i == m.Cursor {
			line = CursorStyle.Render(line)
		} else if r.Critical == procinfo.CritBlocked {
			line = CriticalStyle.Render(line)
		} else if r.Critical == procinfo.CritWarn {
			line = WarnStyle.Render(line)
		}

		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return sb.String()
}

func formatRow(r procinfo.Row, pidW, nameW, portW, parentW, userW int) string {
	pid := fmt.Sprintf("%-*d", pidW, r.PID)
	name := truncPad(r.Proc.Name, nameW)
	addr := fmt.Sprintf("%s:%d", r.LocalAddr, r.LocalPort)
	addr = truncPad(addr, portW)
	parent := "---"
	if r.Proc.ParentPID != 0 && r.Proc.ParentName != "" {
		parent = fmt.Sprintf("%d(%s)", r.Proc.ParentPID, r.Proc.ParentName)
	}
	parent = truncPad(parent, parentW)
	user := truncPad(r.Proc.Username, userW)
	return fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s", pidW, pid, nameW, name, portW, addr, parentW, parent, userW, user)
}

func truncPad(s string, w int) string {
	if w <= 3 {
		if len(s) > w {
			return s[:w]
		}
		return fmt.Sprintf("%-*s", w, s)
	}
	if len(s) > w {
		return s[:w-3] + "..."
	}
	return fmt.Sprintf("%-*s", w, s)
}
