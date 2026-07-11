# Task 5 Report: Kill Package

**Status:** DONE

## Summary
Created `internal/kill/` package with `Terminate(pid uint32) error` using Windows `OpenProcess` + `TerminateProcess` APIs via `golang.org/x/sys/windows`.

## Files
- `internal/kill/kill.go` — single exported function `Terminate`
- `internal/kill/kill_test.go` — two tests

## Test Results
```
=== RUN   TestTerminate_DummyProcess
--- PASS: TestTerminate_DummyProcess (0.02s)
=== RUN   TestTerminate_SystemProcess
--- PASS: TestTerminate_SystemProcess (0.00s)
PASS
ok      process_handler/internal/kill    1.176s
```

Both tests pass:
- `TestTerminate_DummyProcess` — spawns `ping -n 60 127.0.0.1` and terminates it successfully
- `TestTerminate_SystemProcess` — attempts to terminate PID 4 (System), expects and receives an error
