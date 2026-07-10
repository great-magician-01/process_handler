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
