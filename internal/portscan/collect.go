package portscan

import (
	"sync"

	"process_handler/internal/procinfo"
)

func Collect() ([]procinfo.Row, error) {
	var (
		ports            []procinfo.PortEntry
		procs            map[uint32]procinfo.ProcessInfo
		portErr, _       error
		wg               sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		ports, portErr = getListeningPorts()
	}()
	go func() {
		defer wg.Done()
		procs, _ = getProcessInfo()
	}()
	wg.Wait()

	if portErr != nil {
		return nil, portErr
	}

	rows := join(ports, procs)

	for i := range rows {
		user, err := getUsername(rows[i].PID)
		if err == nil {
			rows[i].Proc.Username = user
		}
		rows[i].Critical = procinfo.Classify(rows[i].PID, rows[i].Proc.Name, rows[i].Proc.Username)
	}

	return rows, nil
}
