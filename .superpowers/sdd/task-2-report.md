# Task 2 Report: Port Scanner via GetExtendedTcpTable

**Date:** 2026-07-10

## What was implemented

Created `internal/portscan/ports.go` that calls the Windows API `GetExtendedTcpTable` from `iphlpapi.dll` to enumerate listening TCP ports and their owning PIDs.

Key components:
- **getListeningPorts()** - main function returning `[]procinfo.PortEntry`
- **getExtendedTcpTable()** - syscall wrapper using `windows.NewLazySystemDLL` since this function is not exported by `golang.org/x/sys/windows`
- **mibTCPRowOwnerPID** - Go struct matching the Windows `MIB_TCPROW_OWNER_PID` (24 bytes, no spurious padding)

## Build result

- `go build ./internal/portscan/` - PASS
- `go vet ./internal/portscan/` - PASS

## Files changed

| File | Action |
|------|--------|
| `internal/portscan/ports.go` | Created (new file) |
| `go.mod` | Modified (added `golang.org/x/sys v0.47.0` dependency) |
| `go.sum` | Created (new file) |

## Self-review

- The `GetExtendedTcpTable` function is not exported by `golang.org/x/sys/windows`, so a custom syscall wrapper was defined using `windows.NewLazySystemDLL("iphlpapi.dll")`.
- The struct layout uses the standard 24-byte `MIB_TCPROW_OWNER_PID` without additional padding. The task description suggested a padded 32-byte struct, but the actual Windows API returns 24-byte entries. Using a larger struct would cause misaligned reads.
- Port conversion uses `uint16(row.LocalPort)` rather than `binary.BigEndian.Uint16`. On little-endian Windows, the port value in network-byte-order DWORD happens to be directly readable via the lower 16 bits of the LE uint32. Using `binary.BigEndian.Uint16` on the first 2 bytes of the LE uint32 would produce incorrect results.

## Issues / Concerns

- **DONE_WITH_CONCERNS**: The port conversion approach (`uint16(row.LocalPort)`) relies on little-endian memory layout coincidentally producing the correct port value. A more explicit conversion (`windows.Ntohs(uint16(row.LocalPort))`) could be used but yields the same result on LE systems. This should be verified with integration testing on actual Windows.
- The `getListeningPorts()` function is unexported (lowercase). It will need to be exposed or called internally by other packages.

## Status

**DONE_WITH_CONCERNS**
