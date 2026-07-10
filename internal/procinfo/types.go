package procinfo

type PortEntry struct {
	LocalAddr string // "127.0.0.1"
	LocalPort uint16
	PID       uint32
}

type ProcessInfo struct {
	PID        uint32
	Name       string
	ExePath    string
	CmdLine    string
	ParentPID  uint32
	ParentName string
	Username   string // "DOMAIN\\user"
}

type CriticalLevel int

const (
	CritNone    CriticalLevel = iota
	CritWarn
	CritBlocked
)

type Row struct {
	PortEntry
	Proc     ProcessInfo
	Critical CriticalLevel
}
