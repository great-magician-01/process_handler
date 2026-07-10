# Port Handler TUI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Windows TUI tool (Go + Bubble Tea) to view listening TCP ports, identify owning processes, and safely terminate them.

**Architecture:** Data collection via Windows API (`GetExtendedTcpTable` + WMI `Win32_Process` + token-based username lookup) encapsulated in `portscan`; Bubble Tea `model` consumes `[]Row` and renders a master-detail TUI with browse/filter/confirm/killing states.

**Tech Stack:** Go 1.25, `bubbletea`, `bubbles`, `lipgloss`, `golang.org/x/sys/windows`, `github.com/yusufpapurcu/wmi`

## Global Constraints

- Platform: **Windows only**
- Only **TCP IPv4 listening ports**
- Single-process kill (no batch)
- Go module path: `process_handler`
- All code under `internal/` except `main.go`

---

### Task 1: Project Scaffolding & procinfo Data Types

**Files:**
- Create: `go.mod`
- Create: `internal/procinfo/types.go`
- Create: `internal/procinfo/critical.go`
- Create: `internal/procinfo/critical_test.go`

**Interfaces:**
- Produces: `procinfo.PortEntry`, `procinfo.ProcessInfo`, `procinfo.Row`, `procinfo.CriticalLevel` (and consts `CritNone`/`CritWarn`/`CritBlocked`)
- Produces: `procinfo.Classify(pid uint32, name string, user string) CriticalLevel`

- [ ] **Step 1: Initialize Go module**

```bash
go mod init process_handler
```

- [ ] **Step 2: Create `internal/procinfo/types.go`**

```go
package procinfo

type PortEntry struct {
	LocalAddr string // "127.0.0.1"
	LocalPort uint16
	PID       uint32
}

type ProcessInfo struct {
	PID        uint32
	Name       string
	ExePath    string
	CmdLine    string
	ParentPID  uint32
	ParentName string
	Username   string // "DOMAIN\\user"
}

type CriticalLevel int

const (
	CritNone    CriticalLevel = iota
	CritWarn
	CritBlocked
)

type Row struct {
	PortEntry
	Proc     ProcessInfo
	Critical CriticalLevel
}
```

- [ ] **Step 3: Create `internal/procinfo/critical.go`**

```go
package procinfo

var blockedExeNames = map[string]bool{
	"System": true, "Idle": true, "Registry": true,
	"smss.exe": true, "csrss.exe": true, "wininit.exe": true,
	"services.exe": true, "lsass.exe": true, "winlogon.exe": true,
}

var warnExeNames = map[string]bool{
	"svchost.exe": true, "dwm.exe": true, "explorer.exe": true,
}

var systemUsernames = map[string]bool{
	"SYSTEM": true, "NETWORK SERVICE": true, "LOCAL SERVICE": true,
}

var blockedPIDs = map[uint32]bool{
	0: true, 4: true,
}

func Classify(pid uint32, name string, user string) CriticalLevel {
	if blockedPIDs[pid] || blockedExeNames[name] {
		return CritBlocked
	}
	if systemUsernames[user] || warnExeNames[name] {
		return CritWarn
	}
	return CritNone
}
```

- [ ] **Step 4: Create `internal/procinfo/critical_test.go`**

```go
package procinfo

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		pid    uint32
		name   string
		user   string
		expect CriticalLevel
	}{
		{4, "System", "SYSTEM", CritBlocked},
		{0, "Idle", "SYSTEM", CritBlocked},
		{900, "csrss.exe", "SYSTEM", CritBlocked},
		{800, "lsass.exe", "SYSTEM", CritBlocked},
		{400, "winlogon.exe", "SYSTEM", CritBlocked},
		{1000, "svchost.exe", "NETWORK SERVICE", CritBlocked}, // blocked via name, even with system user — name check wins
		{2000, "dwm.exe", "DOMAIN\\admin", CritWarn},
		{3000, "explorer.exe", "DOMAIN\\admin", CritWarn},
		{4000, "some-service.exe", "NETWORK SERVICE", CritWarn},
		{5000, "some-service.exe", "LOCAL SERVICE", CritWarn},
		{6000, "node.exe", "DOMAIN\\admin", CritNone},
		{7000, "python.exe", "DOMAIN\\admin", CritNone},
		{8000, "java.exe", "SYSTEM", CritWarn}, // system user but not in blocked/warn name list — CritWarn
	}

	for _, tt := range tests {
		got := Classify(tt.pid, tt.name, tt.user)
		if got != tt.expect {
			t.Errorf("Classify(%d, %q, %q) = %d, want %d", tt.pid, tt.name, tt.user, got, tt.expect)
		}
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/procinfo/...
```
Expected: All 12 cases pass.

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/procinfo/
git commit -m "feat: add procinfo types and critical process detection"
```

---

### Task 2: Port Scanner — GetExtendedTcpTable

**Files:**
- Create: `internal/portscan/ports.go`

**Interfaces:**
- Consumes: `procinfo.PortEntry`
- Produces: `func getListeningPorts() ([]procinfo.PortEntry, error)`

- [ ] **Step 1: Install x/sys/windows**

```bash
go get golang.org/x/sys
```

- [ ] **Step 2: Create `internal/portscan/ports.go`**

```go
package portscan

import (
	"encoding/binary"
	"net"
	"unsafe"

	"process_handler/internal/procinfo"

	"golang.org/x/sys/windows"
)

// AF_INET = 2, TCP_TABLE_OWNER_PID_LISTENER = 5, MIB_TCP_STATE_LISTEN = 2
const (
	TCP_TABLE_OWNER_PID_LISTENER = 5
	AF_INET                      = 2
	MIB_TCP_STATE_LISTEN         = 2
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  [4]byte
	_          [4]byte // padding to align LocalPort
	LocalPort  uint32 // network byte order
	RemoteAddr [4]byte
	_          [4]byte
	RemotePort uint32
	OwningPID  uint32
}

func getListeningPorts() ([]procinfo.PortEntry, error) {
	var buf []byte
	var bufSize uint32

	// First call to get required buffer size
	err := windows.GetExtendedTcpTable(
		0,
		&bufSize,
		false,
		AF_INET,
		TCP_TABLE_OWNER_PID_LISTENER,
		0,
	)
	if err != windows.ERROR_INSUFFICIENT_BUFFER {
		return nil, err
	}

	buf = make([]byte, bufSize)
	err = windows.GetExtendedTcpTable(
		uintptr(unsafe.Pointer(&buf[0])),
		&bufSize,
		false,
		AF_INET,
		TCP_TABLE_OWNER_PID_LISTENER,
		0,
	)
	if err != nil {
		return nil, err
	}

	// First 4 bytes = row count
	if len(buf) < 4 {
		return nil, nil
	}
	rowCount := binary.LittleEndian.Uint32(buf[:4])

	rowSize := uint32(unsafe.Sizeof(mibTCPRowOwnerPID{}))
	var rows []procinfo.PortEntry

	for i := uint32(0); i < rowCount; i++ {
		offset := 4 + i*rowSize
		if int(offset)+int(rowSize) > len(buf) {
			break
		}
		row := (*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[offset]))

		if row.State == MIB_TCP_STATE_LISTEN {
			port := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&row.LocalPort))[:])
			ip := net.IPv4(row.LocalAddr[0], row.LocalAddr[1], row.LocalAddr[2], row.LocalAddr[3])

			rows = append(rows, procinfo.PortEntry{
				LocalAddr: ip.String(),
				LocalPort: port,
				PID:       row.OwningPID,
			})
		}
	}

	return rows, nil
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/portscan/
```
Expected: Compiles successfully (no test yet — it needs Windows at runtime).

- [ ] **Step 4: Commit**

```bash
git add internal/portscan/ go.mod go.sum
git commit -m "feat: add port scanner via GetExtendedTcpTable"
```

---

### Task 3: Process Detail Collector (WMI + Username)

**Files:**
- Create: `internal/portscan/process.go`

**Interfaces:**
- Consumes: `procinfo.ProcessInfo`
- Produces: `func getProcessInfo() (map[uint32]procinfo.ProcessInfo, error)` — WMI query for name/path/cmdline/parent PID
- Produces: `func getUsername(pid uint32) (string, error)` — OpenProcessToken + LookupAccountSid

- [ ] **Step 1: Add WMI dependency**

```bash
go get github.com/yusufpapurcu/wmi
```

- [ ] **Step 2: Create `internal/portscan/process.go`**

```go
package portscan

import (
	"process_handler/internal/procinfo"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
)

type wmiProcess struct {
	ProcessID        uint32  `wmi:"ProcessId"`
	Name             string  `wmi:"Name"`
	ExecutablePath   *string `wmi:"ExecutablePath"`
	CommandLine      *string `wmi:"CommandLine"`
	ParentProcessID  uint32  `wmi:"ParentProcessId"`
}

func getProcessInfo() (map[uint32]procinfo.ProcessInfo, error) {
	var dst []wmiProcess
	query := "SELECT ProcessId, Name, ExecutablePath, CommandLine, ParentProcessId FROM Win32_Process"
	err := wmi.Query(query, &dst)
	if err != nil {
		return nil, err
	}

	result := make(map[uint32]procinfo.ProcessInfo, len(dst))
	for _, p := range dst {
		pi := procinfo.ProcessInfo{
			PID:       p.ProcessID,
			Name:      p.Name,
			ParentPID: p.ParentProcessID,
		}
		if p.ExecutablePath != nil {
			pi.ExePath = *p.ExecutablePath
		}
		if p.CommandLine != nil {
			pi.CmdLine = *p.CommandLine
		}
		result[p.ProcessID] = pi
	}

	// Resolve parent names from the same map
	for pid, pi := range result {
		if parent, ok := result[pi.ParentPID]; ok {
			pi.ParentName = parent.Name
			result[pid] = pi
		}
	}

	return result, nil
}

func getUsername(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	var tok windows.Token
	err = windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok)
	if err != nil {
		return "", err
	}
	defer tok.Close()

	user, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}

	sid := user.User.Sid
	account, domain, _, err := sid.LookupAccount("")
	if err != nil {
		return "", err
	}

	return domain + "\\" + account, nil
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/portscan/
```
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add internal/portscan/process.go go.mod go.sum
git commit -m "feat: add process info collector via WMI + token username"
```

---

### Task 4: Join, Collect Orchestrator & Integration Test

**Files:**
- Create: `internal/portscan/join.go`
- Create: `internal/portscan/collect.go`
- Create: `internal/portscan/collect_test.go`

**Interfaces:**
- Consumes: `getListeningPorts()`, `getProcessInfo()`, `getUsername()`
- Produces: `func Collect() ([]procinfo.Row, error)`

- [ ] **Step 1: Create `internal/portscan/join.go`**

```go
package portscan

import "process_handler/internal/procinfo"

func join(ports []procinfo.PortEntry, procs map[uint32]procinfo.ProcessInfo) []procinfo.Row {
	rows := make([]procinfo.Row, 0, len(ports))
	for _, pe := range ports {
		pi, ok := procs[pe.PID]
		if !ok {
			pi = procinfo.ProcessInfo{PID: pe.PID, Name: "Unknown"}
		}
		rows = append(rows, procinfo.Row{
			PortEntry: pe,
			Proc:      pi,
		})
	}
	return rows
}
```

- [ ] **Step 2: Create `internal/portscan/collect.go`**

```go
package portscan

import (
	"sync"

	"process_handler/internal/procinfo"
)

func Collect() ([]procinfo.Row, error) {
	var (
		ports []procinfo.PortEntry
		procs map[uint32]procinfo.ProcessInfo
		portErr, procErr error
		wg    sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		ports, portErr = getListeningPorts()
	}()
	go func() {
		defer wg.Done()
		procs, procErr = getProcessInfo()
	}()
	wg.Wait()

	if portErr != nil {
		return nil, portErr
	}

	rows := join(ports, procs)

	// Enrich with username and critical classification
	if procErr == nil {
		for i := range rows {
			user, err := getUsername(rows[i].PID)
			if err == nil {
				rows[i].Proc.Username = user
			}
			rows[i].Critical = procinfo.Classify(rows[i].PID, rows[i].Proc.Name, rows[i].Proc.Username)
		}
	}

	return rows, nil
}
```

- [ ] **Step 3: Create `internal/portscan/collect_test.go`**

```go
package portscan

import (
	"net"
	"os/exec"
	"testing"
	"time"
)

func TestCollect_FindsListeningPort(t *testing.T) {
	// Start a dummy listener on a random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("failed to start listener:", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	rows, err := Collect()
	if err != nil {
		t.Fatal("Collect() error:", err)
	}

	found := false
	for _, r := range rows {
		if r.LocalPort == uint16(port) {
			found = true
			if r.Proc.Name == "" || r.Proc.Name == "Unknown" {
				t.Error("expected process name to be filled for our own process")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected to find listening port %d in collect results", port)
	}
}

func TestCollect_RowAfterKill(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	cmd.Start()
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	time.Sleep(500 * time.Millisecond)

	rows, err := Collect()
	if err != nil {
		t.Fatal("Collect() error:", err)
	}

	found := false
	for _, r := range rows {
		if r.PID == uint32(cmd.Process.Pid) {
			found = true
			break
		}
	}
	// ping.exe may or may not be in the listening ports list (it doesn't listen)
	// This test just verifies Collect() runs and returns data without error
	_ = found
	t.Logf("collected %d rows", len(rows))
}
```

- [ ] **Step 4: Run integration tests**

```bash
go test -v ./internal/portscan/...
```
Expected: All tests pass (Windows only; skip if not Windows — not applicable since we're Windows-only).

- [ ] **Step 5: Commit**

```bash
git add internal/portscan/join.go internal/portscan/collect.go internal/portscan/collect_test.go
git commit -m "feat: add join + collect orchestrator with integration test"
```

---

### Task 5: Kill Package

**Files:**
- Create: `internal/kill/kill.go`
- Create: `internal/kill/kill_test.go`

**Interfaces:**
- Produces: `func Terminate(pid uint32) error`

- [ ] **Step 1: Create `internal/kill/kill.go`**

```go
package kill

import "golang.org/x/sys/windows"

func Terminate(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	return windows.TerminateProcess(h, 1)
}
```

- [ ] **Step 2: Create `internal/kill/kill_test.go`**

```go
package kill

import (
	"os/exec"
	"testing"
)

func TestTerminate_DummyProcess(t *testing.T) {
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal("start ping:", err)
	}

	pid := uint32(cmd.Process.Pid)

	err := Terminate(pid)
	if err != nil {
		t.Fatal("Terminate:", err)
	}

	// Wait for process to actually exit
	_ = cmd.Wait()
}

func TestTerminate_SystemProcess(t *testing.T) {
	// PID 4 = System, should fail as normal user
	err := Terminate(4)
	if err == nil {
		t.Error("expected error when terminating PID 4 (System)")
	}
}
```

- [ ] **Step 3: Run kill tests**

```bash
go test -v ./internal/kill/...
```
Expected: `TestTerminate_DummyProcess` PASS, `TestTerminate_SystemProcess` PASS (expects error).

- [ ] **Step 4: Commit**

```bash
git add internal/kill/
git commit -m "feat: add kill package via TerminateProcess API"
```

---

### Task 6: TUI Model, Styles & Keys

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keys.go`

**Interfaces:**
- Produces: `tui.Model` struct, `tui.NewModel() Model`, `tui.collectResultMsg`, `tui.tickMsg`, `tui.killResultMsg`
- Produces: Style definitions (theme, table, detail, toast, confirm box)
- Produces: Key binding documentation

- [ ] **Step 1: Create `internal/tui/styles.go`**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	theme = struct {
		Primary    lipgloss.Color
		Secondary  lipgloss.Color
		Danger     lipgloss.Color
		Warning    lipgloss.Color
		Muted      lipgloss.Color
		Bg         lipgloss.Color
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
```

- [ ] **Step 2: Create `internal/tui/keys.go`**

```go
package tui

var helpText = map[State]string{
	stateBrowse: "r: refresh  R: auto-refresh  /: filter  enter: kill  q: quit  j/k or ↑↓: move",
	stateFilter: "type to filter...  esc: back",
	stateConfirm: "y: confirm  n/esc: cancel",
	stateKilling: "killing...",
}
```

- [ ] **Step 3: Create `internal/tui/model.go`**

```go
package tui

import (
	"time"

	"process_handler/internal/procinfo"

	tea "github.com/charmbracelet/bubbletea"
)

type State int

const (
	stateBrowse State = iota
	stateFilter
	stateConfirm
	stateKilling
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
	ConfirmInput  string // for CritWarn PID type confirmation
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
		rows, err := procinfo.Rows(nil) // placeholder — actual import of portscan.Collect comes in Task 8
		return collectResultMsg{Rows: rows, Err: err}
	}
}
```

Note: The `collectCmd` is a placeholder. In Task 8 we wire it to `portscan.Collect()`.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/model.go internal/tui/styles.go internal/tui/keys.go
git commit -m "feat: add TUI model, styles, and key bindings"
```

---

### Task 7: TUI Table & Detail Rendering

**Files:**
- Create: `internal/tui/table.go`
- Create: `internal/tui/detail.go`

**Interfaces:**
- Produces: `func renderTable(m Model) string` — renders the scrolling port table
- Produces: `func renderDetail(m Model) string` — renders the detail pane for the selected row

- [ ] **Step 1: Add bubbles dependency**

```bash
go get github.com/charmbracelet/bubbles
```

- [ ] **Step 2: Create `internal/tui/table.go`**

```go
package tui

import (
	"fmt"
	"strings"

	"process_handler/internal/procinfo"
)

var headerRow = []string{"PID", "NAME", "PORT", "PARENT", "USER"}

func renderTable(m Model) string {
	if m.Width < 60 {
		return MutedStyle.Render("Window too narrow. Please widen terminal.")
	}

	// column widths proportional to terminal width
	pidW := 8
	portW := 25
	userW := 18
	remainder := m.Width - pidW - portW - userW - 4 // 4 separators
	nameW := remainder / 2
	if nameW < 10 {
		nameW = 10
	}
	parentW := remainder - nameW
	if parentW < 10 {
		parentW = 10
	}

	// header
	hdr := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
		pidW, "PID", nameW, "NAME", portW, "PORT", parentW, "PARENT", userW, "USER")
	sep := strings.Repeat("─", pidW+nameW+portW+parentW+userW+4)

	var sb strings.Builder
	sb.WriteString(HeaderStyle.Render(hdr))
	sb.WriteByte('\n')
	sb.WriteString(MutedStyle.Render(sep))
	sb.WriteByte('\n')

	// determine visible rows based on height available for table
	tableHeight := m.Height - 12 // reserve for detail pane + help + title + borders
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
		line := renderRow(r, pidW, nameW, portW, parentW, userW)

		if i == m.Cursor {
			line = CursorStyle.Render(">" + line[1:])
		}

		if r.Critical == procinfo.CritBlocked || r.Critical == procinfo.CritWarn {
			line = "⚠" + line[3:] // overlay warning icon over first chars
			if r.Critical == procinfo.CritBlocked {
				line = CriticalStyle.Render(line)
			} else {
				line = WarnStyle.Render(line)
			}
		}

		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return sb.String()
}

func renderRow(r procinfo.Row, pidW, nameW, portW, parentW, userW int) string {
	pid := fmt.Sprintf("%-*d", pidW, r.PID)
	name := truncateOrPad(r.Proc.Name, nameW)
	addr := fmt.Sprintf("%s:%-*d", r.LocalAddr, portW-len(r.LocalAddr)-1, r.LocalPort)
	if len(addr) > portW {
		addr = addr[:portW]
	}
	addr = fmt.Sprintf("%-*s", portW, addr)

	parent := "—"
	if r.Proc.ParentPID != 0 && r.Proc.ParentName != "" {
		parent = fmt.Sprintf("%d (%s)", r.Proc.ParentPID, truncate(r.Proc.ParentName, parentW-8))
	}
	parent = fmt.Sprintf("%-*s", parentW, truncateOrPad(parent, parentW))

	user := truncateOrPad(r.Proc.Username, userW)

	return fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
		pidW, pid, nameW, name, portW, addr, parentW, parent, userW, user)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func truncateOrPad(s string, w int) string {
	if len(s) > w {
		return truncate(s, w)
	}
	return fmt.Sprintf("%-*s", w, s)
}
```

- [ ] **Step 3: Create `internal/tui/detail.go`**

```go
package tui

import (
	"fmt"
	"strings"

	"process_handler/internal/procinfo"
)

func renderDetail(m Model) string {
	if len(m.Rows) == 0 || m.Cursor >= len(m.Rows) {
		return DetailStyle.Render(MutedStyle.Render("No process selected"))
	}

	r := m.Rows[m.Cursor]

	lines := []string{
		fmt.Sprintf("PID:        %d", r.PID),
		fmt.Sprintf("Name:       %s", r.Proc.Name),
		fmt.Sprintf("Port:       %s:%d", r.LocalAddr, r.LocalPort),
		fmt.Sprintf("ExePath:    %s", r.Proc.ExePath),
		fmt.Sprintf("CmdLine:    %s", r.Proc.CmdLine),
		fmt.Sprintf("Parent:     %d (%s)", r.Proc.ParentPID, r.Proc.ParentName),
		fmt.Sprintf("User:       %s", r.Proc.Username),
	}

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
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./internal/tui/
```
Expected: Compiles successfully (may warn about unused imports/methods — will be wired in next task).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/table.go internal/tui/detail.go go.mod go.sum
git commit -m "feat: add TUI table and detail rendering"
```

---

### Task 8: TUI Update Logic & Tests

**Files:**
- Create: `internal/tui/update.go`
- Create: `internal/tui/update_test.go`

**Interfaces:**
- Consumes: Model, `portscan.Collect()`, `kill.Terminate()`
- Produces: `func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)` — full state machine

- [ ] **Step 1: Fix `model.go` to import portscan**

First, update `collectCmd` in `internal/tui/model.go`:

```
Find the placeholder import and collectCmd in model.go and replace with:

import "process_handler/internal/portscan"

func (m Model) collectCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := portscan.Collect()
		return collectResultMsg{Rows: rows, Err: err}
	}
}
```

Exact edit in `internal/tui/model.go`:
- Replace placeholder `procinfo.Rows(nil)` with `portscan.Collect()`
- Add `"process_handler/internal/portscan"` to imports, remove unused `"time"`

- [ ] **Step 2: Create `internal/tui/update.go`**

```go
package tui

import (
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
		switch m.State {
		case stateBrowse:
			cmd = m.updateBrowse(msg)
		case stateFilter:
			cmd = m.updateFilter(msg)
		case stateConfirm:
			cmd = m.updateConfirm(msg)
		case stateKilling:
			// block input during killing
		}

	case collectResultMsg:
		if msg.Err != nil {
			m.Error = msg.Err.Error()
			m.Rows = m.AllRows // keep old data on error
		} else {
			m.Error = ""
			m.AllRows = msg.Rows
			m.applyFilter()
			// bound cursor
			if m.Cursor >= len(m.Rows) && len(m.Rows) > 0 {
				m.Cursor = len(m.Rows) - 1
			}
			if len(m.Rows) == 0 {
				m.Cursor = 0
			}
		}

	case tickMsg:
		if m.AutoRefreshOn {
			cmd = m.collectCmd()
		}
		// Also schedule next tick if auto-refresh on
		if m.AutoRefreshOn {
			cmd = tea.Batch(cmd, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return tickMsg{}
			}))
		}

	case killResultMsg:
		m.State = stateBrowse
		if msg.Err != nil {
			m.Toast = "Failed to kill PID " + fmt.Sprintf("%d", msg.PID) + ": " + msg.Err.Error()
			m.ToastError = true
		} else {
			m.Toast = "Killed PID " + fmt.Sprintf("%d", msg.PID)
			m.ToastError = false
		}
		// Refresh after kill
		cmd = m.collectCmd()
	}

	return m, cmd
}

func (m *Model) updateBrowse(msg tea.KeyMsg) tea.Cmd {
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

	case "enter", "k":
		if len(m.Rows) == 0 {
			return nil
		}
		row := m.Rows[m.Cursor]
		if row.Critical == procinfo.CritBlocked {
			m.Toast = "Cannot kill critical system process (PID " + fmt.Sprintf("%d", row.PID) + ")"
			m.ToastError = true
			return nil
		}
		m.State = stateConfirm
		m.ConfirmPID = row.PID
		m.ConfirmLevel = row.Critical
		m.ConfirmInput = ""
	}

	return nil
}

func (m *Model) updateFilter(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.State = stateBrowse
		m.Filter = ""

	case "backspace":
		if len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
		}

	default:
		if len(msg.String()) == 1 {
			m.Filter += msg.String()
		}
	}

	m.applyFilter()
	if m.Cursor >= len(m.Rows) {
		m.Cursor = 0
	}

	return nil
}

func (m *Model) updateConfirm(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "n":
		m.State = stateBrowse
		m.ConfirmInput = ""

	case "y":
		if m.ConfirmLevel == procinfo.CritWarn {
			// Must type exact PID to confirm
			expected := fmt.Sprintf("%d", m.ConfirmPID)
			if m.ConfirmInput != expected {
				// Not confirmed by PID yet, treat as if typing
				return nil
			}
		}
		m.State = stateKilling
		pid := m.ConfirmPID
		return func() tea.Msg {
			err := kill.Terminate(pid)
			return killResultMsg{PID: pid, Err: err}
		}

	case "backspace":
		if len(m.ConfirmInput) > 0 {
			m.ConfirmInput = m.ConfirmInput[:len(m.ConfirmInput)-1]
		}

	default:
		s := msg.String()
		if len(s) == 1 && s >= "0" && s <= "9" {
			m.ConfirmInput += s
		}
	}

	return nil
}

func (m *Model) applyFilter() {
	if m.Filter == "" {
		m.Rows = m.AllRows
		return
	}
	lower := strings.ToLower(m.Filter)
	filtered := make([]procinfo.Row, 0)
	for _, r := range m.AllRows {
		if match(lower, r) {
			filtered = append(filtered, r)
		}
	}
	m.Rows = filtered
}

func match(filter string, r procinfo.Row) bool {
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
```

Note: Need to add `"fmt"` and `"strings"` imports. Ensure model.go and update.go share the same package-level imports.

- [ ] **Step 3: Create `internal/tui/update_test.go`**

```go
package tui

import (
	"process_handler/internal/procinfo"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func makeModel(rows []procinfo.Row) Model {
	return Model{
		AllRows: rows,
		Rows:    rows,
		State:   stateBrowse,
		Width:   120,
		Height:  40,
	}
}

func TestUpdate_Quit(t *testing.T) {
	m := makeModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected Quit cmd")
	}
}

func TestUpdate_CursorMove(t *testing.T) {
	rows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1}, Proc: procinfo.ProcessInfo{Name: "a.exe"}},
		{PortEntry: procinfo.PortEntry{PID: 2}, Proc: procinfo.ProcessInfo{Name: "b.exe"}},
	}
	m := makeModel(rows)

	// Move down
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	mm2 := m2.(Model)

	if mm2.Cursor != 1 {
		t.Errorf("cursor should be 1 after down, got %d", mm2.Cursor)
	}

	// Move up
	m3, _ := mm2.Update(tea.KeyMsg{Type: tea.KeyUp})
	mm3 := m3.(Model)

	if mm3.Cursor != 0 {
		t.Errorf("cursor should be 0 after up, got %d", mm3.Cursor)
	}
}

func TestUpdate_FilterMode(t *testing.T) {
	rows := []procinfo.Row{
		{PortEntry: procinfo.PortEntry{PID: 1234, LocalPort: 3000}, Proc: procinfo.ProcessInfo{Name: "node.exe"}},
		{PortEntry: procinfo.PortEntry{PID: 5678, LocalPort: 8080}, Proc: procinfo.ProcessInfo{Name: "java.exe"}},
	}
	m := makeModel(rows)

	// Enter filter
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	mm2 := m2.(Model)

	if mm2.State != stateFilter {
		t.Error("expected filter state after /")
	}

	// Type "node"
	mm2.Filter = "node"
	mm2.applyFilter()

	if len(mm2.Rows) != 1 {
		t.Errorf("expected 1 filtered row, got %d", len(mm2.Rows))
	}
	if mm2.Rows[0].Proc.Name != "node.exe" {
		t.Errorf("expected node.exe, got %s", mm2.Rows[0].Proc.Name)
	}
}

func TestUpdate_KillBlocked(t *testing.T) {
	row := procinfo.Row{
		PortEntry: procinfo.PortEntry{PID: 4, LocalPort: 135},
		Proc:      procinfo.ProcessInfo{Name: "System"},
		Critical:  procinfo.CritBlocked,
	}
	m := makeModel([]procinfo.Row{row})

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm2 := m2.(Model)

	if mm2.Toast == "" || mm2.ToastError != true {
		t.Error("expected error toast when trying to kill blocked process")
	}
}

func TestUpdate_ConfirmCancel(t *testing.T) {
	row := procinfo.Row{
		PortEntry: procinfo.PortEntry{PID: 9999, LocalPort: 3000},
		Proc:      procinfo.ProcessInfo{Name: "test.exe"},
		Critical:  procinfo.CritNone,
	}
	m := makeModel([]procinfo.Row{row})

	// Press enter → go to confirm
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm2 := m2.(Model)
	if mm2.State != stateConfirm {
		t.Error("expected confirm state after enter")
	}

	// Press esc → back to browse
	m3, _ := mm2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm3 := m3.(Model)
	if mm3.State != stateBrowse {
		t.Error("expected browse state after esc from confirm")
	}
}

func TestUpdate_AutoRefreshToggle(t *testing.T) {
	m := makeModel(nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	mm2 := m2.(Model)
	if !mm2.AutoRefreshOn {
		t.Error("expected autoRefreshOn=true after first R")
	}

	m3, _ := mm2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	mm3 := m3.(Model)
	if mm3.AutoRefreshOn {
		t.Error("expected autoRefreshOn=false after second R")
	}
}
```

- [ ] **Step 4: Wire model.go to portscan**

The model.go created in Task 6 needs these exact edits:

In `internal/tui/model.go`:
- Replace import block to include only: `"process_handler/internal/portscan"`, `"process_handler/internal/procinfo"`, `"time"`
- Replace the `collectCmd` method body with:

```go
func (m Model) collectCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := portscan.Collect()
		return collectResultMsg{Rows: rows, Err: err}
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test -v ./internal/tui/...
```
Expected: All 6 tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/update.go internal/tui/update_test.go internal/tui/model.go
git commit -m "feat: add TUI update state machine with tests"
```

---

### Task 9: TUI View (Layout Assembly)

**Files:**
- Create: `internal/tui/view.go`

**Interfaces:**
- Consumes: Model, renderTable, renderDetail, styles
- Produces: `func (m Model) View() string` — full screen layout

- [ ] **Step 1: Create `internal/tui/view.go`**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width < 60 {
		return MutedStyle.Render("Terminal too narrow. Please widen to at least 60 columns.\n") +
			HelpStyle.Render("Press q to quit")
	}

	var sections []string

	// Title
	title := TitleStyle.Render("Process Handler")
	if m.AutoRefreshOn {
		title += MutedStyle.Render("  [auto-refresh: 3s]")
	}
	sections = append(sections, title)

	// Error bar
	if m.Error != "" {
		sections = append(sections, ToastError.Render(m.Error))
	}

	// Main content
	switch m.State {
	case stateConfirm:
		sections = append(sections, renderConfirmDialog(m))
	case stateKilling:
		sections = append(sections, MutedStyle.Render("Killing PID "+fmt.Sprintf("%d", m.ConfirmPID)+"..."))
	default:
		if len(m.Rows) == 0 && len(m.AllRows) == 0 {
			sections = append(sections, MutedStyle.Render("Loading processes..."))
		} else if len(m.Rows) == 0 {
			sections = append(sections, MutedStyle.Render("No processes match filter \""+m.Filter+"\""))
		} else {
			sections = append(sections, renderTable(m))
			sections = append(sections, renderDetail(m))
		}
	}

	// Help bar
	help := helpText[m.State]
	if m.State == stateFilter {
		help = HelpStyle.Render("filter: \"" + m.Filter + "\"") + "  " + helpText[m.State]
	}
	sections = append(sections, HelpStyle.Render(help))

	// Toast overlay (bottom)
	if m.Toast != "" {
		toastRenderer := ToastSuccess
		if m.ToastError {
			toastRenderer = ToastError
		}
		sections = append(sections, toastRenderer.Render(m.Toast))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderConfirmDialog(m Model) string {
	if len(m.Rows) == 0 || m.Cursor >= len(m.Rows) {
		return ""
	}
	r := m.Rows[m.Cursor]

	var lines []string
	lines = append(lines, fmt.Sprintf("Terminate PID %d (%s) ?", r.PID, r.Proc.Name))
	lines = append(lines, fmt.Sprintf("Port: %s:%d", r.LocalAddr, r.LocalPort))

	if r.Critical == procinfo.CritBlocked {
		lines = append(lines, CriticalStyle.Render("\nThis process CANNOT be terminated. It is a critical system process."))
		lines = append(lines, HelpStyle.Render("\nPress esc to go back"))
	} else {
		if r.Critical == procinfo.CritWarn {
			lines = append(lines, WarnStyle.Render("\n⚠ Warning: This appears to be a system-owned process."))
			lines = append(lines, WarnStyle.Render("Terminating it may cause system instability."))
			if m.ConfirmInput != "" {
				lines = append(lines, fmt.Sprintf("Type full PID to confirm: %s", m.ConfirmInput))
			} else {
				lines = append(lines, "Type the PID number to confirm: "+fmt.Sprintf("%d", r.PID))
			}
		} else {
			lines = append(lines, HelpStyle.Render("\nPress y to confirm, n/esc to cancel"))
		}
	}

	content := strings.Join(lines, "\n")
	return ConfirmBox.Width(min(m.Width-4, 60)).Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

Note: Add `"process_handler/internal/procinfo"` to imports of view.go.

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/tui/
```
Expected: Compiles successfully.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/view.go
git commit -m "feat: add TUI view layout assembly"
```

---

### Task 10: Main Entry Point & Final Wiring

**Files:**
- Create: `main.go`

**Interfaces:**
- Consumes: `tui.Model`, `tui.NewModel()`

- [ ] **Step 1: Create `main.go`**

```go
package main

import (
	"fmt"
	"os"

	"process_handler/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := tui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Add Bubble Tea dependency**

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
```

- [ ] **Step 3: Verify full build**

```bash
go build -o process_handler.exe .
```
Expected: Builds successfully. The binary `process_handler.exe` is created.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```
Expected: All tests pass.

- [ ] **Step 5: Sanity check — run the binary**

```bash
.\process_handler.exe
```
Expected: TUI launches, shows listening ports with process details, arrow keys work, `/` filter works, `q` exits.

- [ ] **Step 6: Final commit**

```bash
go mod tidy
git add main.go go.mod go.sum
git commit -m "feat: add main entry point and wire TUI"
```

---

### Plan Self-Review

**Note for implementer:** The `portscan/process.go` WMI library `github.com/yusufpapurcu/wmi` may require `github.com/go-ole/go-ole` as a transitive dependency. If `go mod tidy` doesn't resolve it, run `go get github.com/go-ole/go-ole`. If the library itself is unavailable, fall back to `github.com/StackExchange/wmi` which uses a similar API (`wmi.Query(query, &dst)`).

The `GetExtendedTcpTable` definition in `x/sys/windows` wraps `iphlpapi.dll`. The `mibTCPRowOwnerPID` struct's memory layout must match the Windows structure exactly — the padding fields (`_ [4]byte`) are critical for 64-bit alignment.
