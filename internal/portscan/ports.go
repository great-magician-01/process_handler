package portscan

import (
	"net"
	"syscall"
	"unsafe"

	"process_handler/internal/procinfo"

	"golang.org/x/sys/windows"
)

const (
	afInet                   = 2
	tcpTableOwnerPIDListener = 5
	mibTCPStateListen        = 2
)

var (
	modIphlpapi             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modIphlpapi.NewProc("GetExtendedTcpTable")
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  [4]byte
	LocalPort  uint32
	RemoteAddr [4]byte
	RemotePort uint32
	OwningPID  uint32
}

func getExtendedTcpTable(pTcpTable unsafe.Pointer, pdwSize *uint32, bOrder bool,
	ulAf uint32, tableClass uint32, reserved uint32) error {
	var order uintptr
	if bOrder {
		order = 1
	}
	r0, _, _ := syscall.SyscallN(procGetExtendedTcpTable.Addr(),
		uintptr(pTcpTable), uintptr(unsafe.Pointer(pdwSize)), order,
		uintptr(ulAf), uintptr(tableClass), uintptr(reserved))
	if r0 != 0 {
		return syscall.Errno(r0)
	}
	return nil
}

func getListeningPorts() ([]procinfo.PortEntry, error) {
	var bufSize uint32
	err := getExtendedTcpTable(nil, &bufSize, false, afInet, tcpTableOwnerPIDListener, 0)
	if err != windows.ERROR_INSUFFICIENT_BUFFER {
		return nil, err
	}

	buf := make([]byte, bufSize)
	err = getExtendedTcpTable(unsafe.Pointer(&buf[0]), &bufSize, false, afInet, tcpTableOwnerPIDListener, 0)
	if err != nil {
		return nil, err
	}

	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(mibTCPRowOwnerPID{})
	entries := make([]procinfo.PortEntry, 0, count)

	for i := uint32(0); i < count; i++ {
		row := (*mibTCPRowOwnerPID)(unsafe.Pointer(&buf[4+uintptr(i)*rowSize]))
		if row.State != mibTCPStateListen {
			continue
		}
		port := uint16(row.LocalPort)
		ip := net.IPv4(row.LocalAddr[0], row.LocalAddr[1], row.LocalAddr[2], row.LocalAddr[3]).String()

		entries = append(entries, procinfo.PortEntry{
			LocalAddr: ip,
			LocalPort: port,
			PID:       row.OwningPID,
		})
	}

	return entries, nil
}
