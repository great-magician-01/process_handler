package portscan

import (
	"github.com/great-magician-01/process_handler/internal/procinfo"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
)

type wmiProcess struct {
	ProcessID       uint32  `wmi:"ProcessId"`
	Name            string  `wmi:"Name"`
	ExecutablePath  *string `wmi:"ExecutablePath"`
	CommandLine     *string `wmi:"CommandLine"`
	ParentProcessID uint32  `wmi:"ParentProcessId"`
}

func getProcessInfo() (map[uint32]procinfo.ProcessInfo, error) {
	var dst []wmiProcess
	q := "SELECT ProcessId, Name, ExecutablePath, CommandLine, ParentProcessId FROM Win32_Process"
	if err := wmi.Query(q, &dst); err != nil {
		return nil, err
	}

	result := make(map[uint32]procinfo.ProcessInfo, len(dst))
	for _, p := range dst {
		pi := procinfo.ProcessInfo{
			PID:       p.ProcessID,
			Name:      p.Name,
			ParentPID: p.ParentProcessID,
		}
		if p.ExecutablePath != nil {
			pi.ExePath = *p.ExecutablePath
		}
		if p.CommandLine != nil {
			pi.CmdLine = *p.CommandLine
		}
		result[p.ProcessID] = pi
	}

	for _, pi := range result {
		if parent, ok := result[pi.ParentPID]; ok {
			pi.ParentName = parent.Name
		}
		result[pi.PID] = pi
	}

	return result, nil
}

func getUsername(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	var tok windows.Token
	err = windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok)
	if err != nil {
		return "", err
	}
	defer tok.Close()

	user, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}

	sid := user.User.Sid
	account, domain, _, err := sid.LookupAccount("")
	if err != nil {
		return "", err
	}

	return domain + "\\" + account, nil
}
