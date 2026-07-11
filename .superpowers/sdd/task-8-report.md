# Task 8: TUI Update Logic & Tests

**Status:** DONE

## Summary

Created the bubbletea `Update` state machine for the TUI and comprehensive unit tests.

## Files Created

- `internal/tui/update.go` — Full Update function with state machine (browse/filter/confirm/killing)
- `internal/tui/update_test.go` — 13 unit tests covering all states and edge cases

## Files Modified

- `internal/tui/model.go` — Added `View()` stub method (required for `tea.Model` interface compliance)

## Test Results

All 13 tests pass:
- `TestUpdate_Quit`
- `TestUpdate_CursorMove`
- `TestUpdate_CursorClampUpper`
- `TestUpdate_CursorClampLower`
- `TestUpdate_FilterMode`
- `TestUpdate_KillBlocked`
- `TestUpdate_ConfirmCancel`
- `TestUpdate_AutoRefreshToggle`
- `TestUpdate_CollectResult`
- `TestUpdate_CollectError`
- `TestUpdate_KillResult`
- `TestUpdate_TickWhenDisabled`
- `TestUpdate_ManualRefresh`

Full build (`go build ./...`) succeeds with no errors.
