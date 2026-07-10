package tui

var helpText = map[State]string{
	stateBrowse:  "r: refresh  R: auto-refresh  /: filter  enter: kill  q: quit  j/k or arrows: move",
	stateFilter:  "type to filter...  esc: back",
	stateConfirm: "y: confirm  n/esc: cancel",
	stateKilling: "killing...",
}
