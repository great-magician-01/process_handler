package tui

var helpText = map[State]string{
	stateBrowse:  "r: refresh  R: auto-refresh  /: filter  enter/space: kill  ?: help  q: quit  j/k or arrows: move",
	stateFilter:  "type to filter...  esc: back",
	stateConfirm: "y: confirm  n/esc: cancel",
	stateKilling: "killing...",
	stateHelp:    "esc: back",
}

var helpOverlay = ` Key Bindings
 ───────────
  j/↓        move cursor down
  k/↑        move cursor up
  r          manual refresh
  R          toggle auto-refresh (3s)
  /          enter filter mode
  enter/space kill selected process
  ?          show this help
  q          quit
  ctrl+c     quit from any state

 In Filter Mode:
  type to filter by PID, name, port, or path
  esc to exit filter

 In Confirm Dialog:
  y to confirm kill
  n/esc to cancel
  For system-owned processes: type PID to confirm`
