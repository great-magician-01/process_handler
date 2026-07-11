# Task 3 Report: Process Detail Collector (WMI + Username)

- **Status**: DONE
- **Module**: `process_handler/internal/portscan/process.go`

## Summary

Created `internal/portscan/process.go` with two exported functions:

### `getProcessInfo() (map[uint32]procinfo.ProcessInfo, error)`
- Queries WMI `Win32_Process` for ProcessId, Name, ExecutablePath, CommandLine, ParentProcessId
- Handles nil pointer fields (ExecutablePath, CommandLine) safely
- Resolves parent process names from the same map in a second pass
- Dependency: `github.com/yusufpapurcu/wmi` v1.2.4

### `getUsername(pid uint32) (string, error)`
- Opens process handle with `PROCESS_QUERY_INFORMATION`
- Opens process token with `TOKEN_QUERY`
- Retrieves token user SID, looks up account name
- Returns `"DOMAIN\user"` format
- Returns error on any failure (permission denied, process exited, etc.)
- Dependency: `golang.org/x/sys/windows` (already present)

## Verification
- `go build ./internal/portscan/` passes cleanly

## Commit
- `116c01d`: feat: add process info collector via WMI + token username

## Dependencies Added
- `github.com/yusufpapurcu/wmi v1.2.4`
- `github.com/go-ole/go-ole v1.2.6` (transitive)
