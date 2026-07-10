package procinfo

var blockedExeNames = map[string]bool{
	"System": true, "Idle": true, "Registry": true,
	"smss.exe": true, "csrss.exe": true, "wininit.exe": true,
	"services.exe": true, "lsass.exe": true, "winlogon.exe": true,
}

var warnExeNames = map[string]bool{
	"svchost.exe": true, "dwm.exe": true, "explorer.exe": true,
}

var systemUsernames = map[string]bool{
	"SYSTEM": true, "NETWORK SERVICE": true, "LOCAL SERVICE": true,
}

var blockedPIDs = map[uint32]bool{
	0: true, 4: true,
}

func Classify(pid uint32, name string, user string) CriticalLevel {
	if blockedPIDs[pid] || blockedExeNames[name] {
		return CritBlocked
	}
	if systemUsernames[user] {
		if warnExeNames[name] {
			return CritBlocked
		}
		return CritWarn
	}
	if warnExeNames[name] {
		return CritWarn
	}
	return CritNone
}
