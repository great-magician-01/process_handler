package tui

import (
	"fmt"
	"testing"

	"process_handler/internal/procinfo"

	tea "github.com/charmbracelet/bubbletea"
)

func makeTestModel(rows []procinfo.Row) Model {
	return Model{
		AllRows: rows,
		Rows:    rows,
		State:   stateBrowse,
		Width:   120,
		Height:  40,
	}
}

func TestUpdate_Quit(t *testing.T) {
	m := makeTestModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected Quit cmd for 'q'")
	}
}

func TestUpdate_CursorMove(t *testing.T) {
	rows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1}, Proc: procinfo.ProcessInfo{Name: "a.exe"}},
		{PortEntry: procinfo.PortEntry{PID: 2}, Proc: procinfo.ProcessInfo{Name: "b.exe"}},
	}
	m := makeTestModel(rows)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(Model).Cursor != 1 {
		t.Errorf("cursor should be 1 after down, got %d", m2.(Model).Cursor)
	}

	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m3.(Model).Cursor != 0 {
		t.Errorf("cursor should be 0 after up, got %d", m3.(Model).Cursor)
	}
}

func TestUpdate_CursorClampUpper(t *testing.T) {
	rows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1}},
	}
	m := makeTestModel(rows)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(Model).Cursor != 0 {
		t.Error("cursor should not move below last row")
	}
}

func TestUpdate_CursorClampLower(t *testing.T) {
	rows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1}},
	}
	m := makeTestModel(rows)
	m.Cursor = 0

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m2.(Model).Cursor != 0 {
		t.Error("cursor should not move above 0")
	}
}

func TestUpdate_FilterMode(t *testing.T) {
	rows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1234, LocalPort: 3000}, Proc: procinfo.ProcessInfo{Name: "node.exe"}},
		{PortEntry: procinfo.PortEntry{PID: 5678, LocalPort: 8080}, Proc: procinfo.ProcessInfo{Name: "java.exe"}},
	}
	m := makeTestModel(rows)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if m2.(Model).State != stateFilter {
		t.Error("expected filter state after /")
	}

	mm := m2.(Model)
	mm.Filter = "node"
	mm.applyFilter()
	if len(mm.Rows) != 1 || mm.Rows[0].Proc.Name != "node.exe" {
		t.Error("filter should match node.exe")
	}

	m3, _ := mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m3.(Model).State != stateBrowse || m3.(Model).Filter != "" {
		t.Error("escape should clear filter and return to browse")
	}
}

func TestUpdate_KillBlocked(t *testing.T) {
	row := procinfo.Row{
		PortEntry: procinfo.PortEntry{PID: 4, LocalPort: 135},
		Proc:      procinfo.ProcessInfo{Name: "System"},
		Critical:  procinfo.CritBlocked,
	}
	m := makeTestModel([]procinfo.Row{row})

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(Model).Toast == "" || !m2.(Model).ToastError {
		t.Error("expected error toast when trying to kill blocked process")
	}
	if m2.(Model).State != stateBrowse {
		t.Error("should stay in browse when killing blocked process")
	}
}

func TestUpdate_ConfirmCancel(t *testing.T) {
	row := procinfo.Row{
		PortEntry: procinfo.PortEntry{PID: 9999, LocalPort: 3000},
		Proc:      procinfo.ProcessInfo{Name: "test.exe"},
	}
	m := makeTestModel([]procinfo.Row{row})

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(Model).State != stateConfirm {
		t.Error("expected confirm state after enter")
	}

	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m3.(Model).State != stateBrowse {
		t.Error("expected browse state after esc from confirm")
	}
}

func TestUpdate_AutoRefreshToggle(t *testing.T) {
	m := makeTestModel(nil)
	if m.AutoRefreshOn {
		t.Error("auto-refresh should start disabled")
	}

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if !m2.(Model).AutoRefreshOn {
		t.Error("autoRefreshOn should be true after first R")
	}
	if cmd == nil {
		t.Error("first R should return a tick command")
	}

	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if m3.(Model).AutoRefreshOn {
		t.Error("autoRefreshOn should be false after second R")
	}
}

func TestUpdate_CollectResult(t *testing.T) {
	m := makeTestModel(nil)
	newRows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1, LocalPort: 8080}, Proc: procinfo.ProcessInfo{Name: "web.exe"}},
	}
	m2, _ := m.Update(collectResultMsg{Rows: newRows, Err: nil})
	model := m2.(Model)
	if len(model.AllRows) != 1 {
		t.Error("AllRows should be updated")
	}
	if model.Error != "" {
		t.Error("error should be cleared on success")
	}
}

func TestUpdate_CollectError(t *testing.T) {
	m := makeTestModel(nil)
	m2, _ := m.Update(collectResultMsg{Rows: nil, Err: fmt.Errorf("test error")})
	if m2.(Model).Error == "" {
		t.Error("expected error message")
	}
}

func TestUpdate_KillResult(t *testing.T) {
	m := makeTestModel(nil)
	m2, _ := m.Update(killResultMsg{PID: 100, Err: nil})
	model := m2.(Model)
	if model.Toast == "" || model.ToastError {
		t.Error("expected success toast")
	}
	if model.State != stateBrowse {
		t.Error("should return to browse after kill")
	}
}

func TestUpdate_TickWhenDisabled(t *testing.T) {
	m := makeTestModel(nil)
	m.AutoRefreshOn = false
	m2, cmd := m.Update(tickMsg{})
	if cmd != nil {
		t.Error("tick should not produce cmd when auto-refresh disabled")
	}
	_ = m2
}

func TestUpdate_ManualRefresh(t *testing.T) {
	m := makeTestModel(nil)
	m.Toast = "old toast"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m2.(Model).Toast != "" {
		t.Error("refresh should clear toast")
	}
}
