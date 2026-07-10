package kill

import "golang.org/x/sys/windows"

func Terminate(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	return windows.TerminateProcess(h, 1)
}
