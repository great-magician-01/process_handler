package procinfo

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		pid    uint32
		name   string
		user   string
		expect CriticalLevel
	}{
		{4, "System", "SYSTEM", CritBlocked},
		{0, "Idle", "SYSTEM", CritBlocked},
		{900, "csrss.exe", "SYSTEM", CritBlocked},
		{800, "lsass.exe", "SYSTEM", CritBlocked},
		{400, "winlogon.exe", "SYSTEM", CritBlocked},
		{1000, "svchost.exe", "NETWORK SERVICE", CritBlocked},
		{2000, "dwm.exe", "DOMAIN\\admin", CritWarn},
		{3000, "explorer.exe", "DOMAIN\\admin", CritWarn},
		{4000, "some-service.exe", "NETWORK SERVICE", CritWarn},
		{5000, "some-service.exe", "LOCAL SERVICE", CritWarn},
		{6000, "node.exe", "DOMAIN\\admin", CritNone},
		{7000, "python.exe", "DOMAIN\\admin", CritNone},
		{8000, "java.exe", "SYSTEM", CritWarn},
	}

	for _, tt := range tests {
		got := Classify(tt.pid, tt.name, tt.user)
		if got != tt.expect {
			t.Errorf("Classify(%d, %q, %q) = %d, want %d", tt.pid, tt.name, tt.user, got, tt.expect)
		}
	}
}
