package tui

import (
	"fmt"
	"strings"
	"time"

	"process_handler/internal/kill"
	"process_handler/internal/procinfo"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	switch m.State {
	case stateBrowse:
		cmd = updateBrowse(&m, msg)
	case stateFilter:
		cmd = updateFilter(&m, msg)
	case stateConfirm:
		cmd = updateConfirm(&m, msg)
	case stateHelp:
		if msg.String() == "esc" {
			m.State = stateBrowse
		}
	case stateKilling:
	}

	case collectResultMsg:
		if msg.Err != nil {
			m.Error = msg.Err.Error()
		} else {
			m.Error = ""
			if m.State != stateConfirm {
				m.AllRows = msg.Rows
				m.applyFilter()
				if m.Cursor >= len(m.Rows) && len(m.Rows) > 0 {
					m.Cursor = len(m.Rows) - 1
				}
				if len(m.Rows) == 0 {
					m.Cursor = 0
				}
			}
		}

	case tickMsg:
		if m.AutoRefreshOn {
			cmd = m.collectCmd()
			cmd = tea.Batch(cmd, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg{}
			}))
		}

	case killResultMsg:
		m.State = stateBrowse
		if msg.Err != nil {
			m.Toast = fmt.Sprintf("Failed to kill PID %d: %s", msg.PID, msg.Err.Error())
			m.ToastError = true
		} else {
			m.Toast = fmt.Sprintf("Killed PID %d", msg.PID)
			m.ToastError = false
		}
		cmd = m.collectCmd()
	}

	return m, cmd
}

func updateBrowse(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "j", "down":
		if m.Cursor < len(m.Rows)-1 {
			m.Cursor++
		}
	case "k", "up":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "r":
		m.Toast = ""
		return m.collectCmd()
	case "R":
		m.AutoRefreshOn = !m.AutoRefreshOn
		if m.AutoRefreshOn {
			return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg{}
			})
		}
	case "/":
		m.State = stateFilter
		m.Filter = ""
	case "enter", " ":
		if len(m.Rows) == 0 {
			return nil
		}
		row := m.Rows[m.Cursor]
		if row.Critical == procinfo.CritBlocked {
			m.Toast = fmt.Sprintf("Cannot kill critical system process (PID %d)", row.PID)
			m.ToastError = true
			return nil
		}
		m.State = stateConfirm
		m.ConfirmPID = row.PID
		m.ConfirmLevel = row.Critical
		m.ConfirmInput = ""
		m.Toast = ""
	case "?":
		m.State = stateHelp
	}
	return nil
}

func updateFilter(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.State = stateBrowse
		m.Filter = ""
	case "backspace":
		if len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.Filter += string(msg.Runes[0])
		}
	}
	m.applyFilter()
	if m.Cursor >= len(m.Rows) {
		m.Cursor = 0
	}
	return nil
}

func updateConfirm(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "n":
		m.State = stateBrowse
		m.ConfirmInput = ""
	case "y":
		if m.ConfirmLevel == procinfo.CritWarn {
			expected := fmt.Sprintf("%d", m.ConfirmPID)
			if m.ConfirmInput != expected {
				return nil
			}
		}
		m.State = stateKilling
		pid := m.ConfirmPID
		return func() tea.Msg {
			return killResultMsg{PID: pid, Err: kill.Terminate(pid)}
		}
	case "backspace":
		if len(m.ConfirmInput) > 0 {
			m.ConfirmInput = m.ConfirmInput[:len(m.ConfirmInput)-1]
		}
	default:
		r := msg.Runes
		if len(r) == 1 && r[0] >= '0' && r[0] <= '9' {
			m.ConfirmInput += string(r[0])
		}
	}
	return nil
}

func (m *Model) applyFilter() {
	var rows []procinfo.Row
	if m.Filter == "" {
		rows = m.AllRows
	} else {
		lower := strings.ToLower(m.Filter)
		rows = make([]procinfo.Row, 0)
		for _, r := range m.AllRows {
			if matchFilter(lower, r) {
				rows = append(rows, r)
			}
		}
	}
	filtered := make([]procinfo.Row, 0, len(rows))
	for _, r := range rows {
		if r.Critical != procinfo.CritBlocked {
			filtered = append(filtered, r)
		}
	}
	m.Rows = filtered
}

func matchFilter(filter string, r procinfo.Row) bool {
	if strings.Contains(strings.ToLower(r.Proc.Name), filter) {
		return true
	}
	if strings.Contains(strings.ToLower(fmt.Sprintf("%d", r.PID)), filter) {
		return true
	}
	if strings.Contains(strings.ToLower(fmt.Sprintf("%d", r.LocalPort)), filter) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Proc.ExePath), filter) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Proc.CmdLine), filter) {
		return true
	}
	return false
}
