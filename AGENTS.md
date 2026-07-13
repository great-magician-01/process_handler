# AGENTS.md

Windows-only terminal TUI for finding and killing processes that hold listening TCP ports. Go 1.25 + Bubble Tea.

## Platform

- **Windows only.** Uses `golang.org/x/sys/windows` (iphlpapi.dll, OpenProcess, TerminateProcess) and WMI via `github.com/yusufpapurcu/wmi`. Will not build or test on Linux/macOS.
- Tests are integration-style and Windows-only: `portscan` opens a real `net.Listen` and asserts `Collect()` sees it; `kill` spawns `ping -n 60` and terminates it; `procinfo` is a pure table-driven test (safe, fast).

## Commands

```bash
go run .                       # launch the TUI (alt-screen, interactive)
go build ./...                 # build all packages
go vet ./...                   # baseline static check (no linter/CI configured)
go test ./...                  # all tests
go test ./internal/portscan    # one package
go test ./internal/procinfo -run TestClassify   # one test
```

No Makefile, no formatter, no golangci-lint, no CI workflows. `go vet` is the only static check.

## Architecture

Module path is the bare name `process_handler` (not a URL); imports look like `process_handler/internal/portscan`.

- `main.go` — entry; starts `tea.NewProgram` with alt screen.
- `internal/portscan/` — **data collection layer**. `Collect()` (`collect.go`) is the only external entry point; it runs the port scan and WMI query concurrently, joins by PID, fills usernames, and classifies. TUI only consumes `[]procinfo.Row`.
  - `ports.go` — `GetExtendedTcpTable` (TCP_TABLE_OWNER_PID_LISTENER) via syscall. **IPv4 TCP listeners only.** Port is network-byte-order and swapped manually (`raw>>8 | raw<<8`).
  - `process.go` — WQL `Win32_Process` for process details; username via `OpenProcess` + token + `LookupAccountSid` (pure API, avoids WMI method-call pitfalls).
  - `join.go` — ports × processes by PID; missing PID → `Name: "Unknown"`.
- `internal/procinfo/` — types (`PortEntry`, `ProcessInfo`, `Row`, `CriticalLevel`) and `Classify` (pure function).
- `internal/kill/` — `Terminate(pid)` via `OpenProcess(PROCESS_TERMINATE)` + `TerminateProcess`.
- `internal/tui/` — Bubble Tea Model/Update/View split across `model.go`, `update.go`, `view.go`, `table.go`, `detail.go`, `keys.go`, `styles.go`.

## Conventions and gotchas

- **`Row` embeds `PortEntry` and uses a named `Proc ProcessInfo` field** because both structs have a `PID` field — embedding both would collide. `Row.PID` comes from `PortEntry`; `Proc.PID` matches it by join.
- **Kill safety is enforced in the TUI, not in `kill.Terminate`:** `CritBlocked` rows are hard-rejected before reaching `confirm`; `CritWarn` rows require the user to type the full PID number before `y` confirms. `kill.Terminate` itself is a thin wrapper and relies on `ERROR_ACCESS_DENIED` as a natural guard.
- `Collect()` swallows the WMI error (degrades to port + "Unknown" process) but **surfaces** the port-scan error. Preserve this asymmetry when editing.
- **No code comments** in the codebase; match that style.
- Commits follow conventional commits (`feat:`, `fix:`, `chore:`, `docs:`).
- Design spec and implementation plan live in `docs/superpowers/specs/` and `docs/superpowers/plans/`. The spec lists `bubbles` as a dependency but it is **not** actually used (only `bubbletea` + `lipgloss`).

## Out of scope (per design spec)

IPv6/UDP/non-listening connections, cross-platform, batch kill, graceful termination, persistent config. Don't add these without checking the spec.
