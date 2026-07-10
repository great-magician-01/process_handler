package portscan

import (
	"net"
	"testing"
)

func TestCollect_FindsListeningPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("failed to start listener:", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	rows, err := Collect()
	if err != nil {
		t.Fatal("Collect() error:", err)
	}

	found := false
	for _, r := range rows {
		if r.LocalPort == uint16(port) {
			found = true
			if r.Proc.Name == "" || r.Proc.Name == "Unknown" {
				t.Error("expected process name to be filled for our own process")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected to find listening port %d in collect results, got %d rows", port, len(rows))
	}
}

func TestCollect_RunsWithoutError(t *testing.T) {
	rows, err := Collect()
	if err != nil {
		t.Fatal("Collect() error:", err)
	}
	t.Logf("collected %d rows", len(rows))
}
