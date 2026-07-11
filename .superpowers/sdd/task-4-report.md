# Task 4 Report: Join, Collect Orchestrator & Integration Test

**Status:** DONE

## Summary

Created three new files and fixed a bug in `ports.go`:

### New Files
- `internal/portscan/join.go` — `join()` function that pairs `PortEntry` records with their `ProcessInfo`, falling back to "Unknown" when a process is not found.
- `internal/portscan/collect.go` — `Collect()` function, the only public API of the portscan package. Runs `getListeningPorts()` and `getProcessInfo()` concurrently, joins results, enriches with username and critical classification.
- `internal/portscan/collect_test.go` — Two integration tests: one verifies a dynamically created listener is found with correct process info, the other ensures `Collect()` runs without error.

### Bug Fix
- `internal/portscan/ports.go:70` — Fixed port byte-order conversion. The Windows `dwLocalPort` field stores the port in network byte order (big-endian), but `uint16(row.LocalPort)` reads the raw little-endian bytes. Added byte swap (`raw>>8 | raw<<8`) to correctly convert to host byte order.

## Test Results
```
=== RUN   TestCollect_FindsListeningPort
--- PASS: TestCollect_FindsListeningPort (7.63s)
=== RUN   TestCollect_RunsWithoutError
    collect_test.go:42: collected 52 rows
--- PASS: TestCollect_RunsWithoutError (9.80s)
PASS
ok  	process_handler/internal/portscan	18.807s
```
