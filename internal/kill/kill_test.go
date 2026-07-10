package kill

import (
	"os/exec"
	"testing"
)

func TestTerminate_DummyProcess(t *testing.T) {
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal("start ping:", err)
	}

	pid := uint32(cmd.Process.Pid)

	err := Terminate(pid)
	if err != nil {
		t.Fatal("Terminate:", err)
	}

	_ = cmd.Wait()
}

func TestTerminate_SystemProcess(t *testing.T) {
	err := Terminate(4)
	if err == nil {
		t.Error("expected error when terminating PID 4 (System)")
	}
}
