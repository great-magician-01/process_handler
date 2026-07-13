package tui

import (
	"github.com/great-magician-01/process_handler/internal/portscan"
	"github.com/great-magician-01/process_handler/internal/procinfo"

	tea "github.com/charmbracelet/bubbletea"
)

type State int

const (
	stateBrowse State = iota
	stateFilter
	stateConfirm
	stateKilling
	stateHelp
)

type Model struct {
	Rows          []procinfo.Row
	AllRows       []procinfo.Row
	Cursor        int
	Filter        string
	AutoRefreshOn bool
	State         State
	Width         int
	Height        int
	Toast         string
	ToastError    bool
	ConfirmPID    uint32
	ConfirmLevel  procinfo.CriticalLevel
	Error         string
	ConfirmInput  string
}

type collectResultMsg struct {
	Rows []procinfo.Row
	Err  error
}

type tickMsg struct{}

type killResultMsg struct {
	PID uint32
	Err error
}

func NewModel() Model {
	return Model{
		Rows:  []procinfo.Row{},
		State: stateBrowse,
	}
}

func (m Model) Init() tea.Cmd {
	return m.collectCmd()
}

func (m Model) collectCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := portscan.Collect()
		return collectResultMsg{Rows: rows, Err: err}
	}
}
