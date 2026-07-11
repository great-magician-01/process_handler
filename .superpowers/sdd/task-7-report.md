# Task 7: TUI Table & Detail Rendering

**Status: DONE**

## Files Created

- `internal/tui/table.go` - Scrolling table with columns PID | NAME | PORT | PARENT(PID+Name) | USER, windowed cursor navigation, truncation, and critical-level styling
- `internal/tui/detail.go` - Detail pane showing full process info for the selected row, with critical-level labels

## Dependencies

- Added `github.com/charmbracelet/bubbles` (and upgraded several indirect dependencies: colorprofile, x/ansi, x/cellbuf, x/term, lucasb-eyer/go-colorful, mattn/go-runewidth)

## Verification

- `go build ./...` compiles successfully with zero errors.
