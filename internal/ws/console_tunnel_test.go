// Package ws — console_tunnel_test.go: tests for the SSH-tunnel error
// formatter. When the local SSH tunnel port fails to come up, the real cause
// is in the ssh process's stderr (e.g. "Permission denied (publickey)", "Load
// key: invalid format", "Could not resolve host") — but it was being discarded
// to os.Stderr, never reaching the frontend. formatTunnelError appends a
// bounded tail of that output so the user sees why.
package ws

import (
	"fmt"
	"strings"
	"testing"
)

func TestFormatTunnelError(t *testing.T) {
	portErr := fmt.Errorf("port 43495 not ready after 15s")

	// No ssh output -> just the port error, no "ssh:" suffix.
	got := formatTunnelError(portErr, "")
	if !strings.Contains(got, "port 43495") {
		t.Errorf("missing port error: %q", got)
	}
	if strings.Contains(got, "ssh:") {
		t.Errorf("empty sshOut should not add ssh: suffix: %q", got)
	}

	// With ssh output -> append it.
	got = formatTunnelError(portErr, "Permission denied (publickey).")
	if !strings.Contains(got, "port 43495") || !strings.Contains(got, "ssh:") || !strings.Contains(got, "Permission denied") {
		t.Errorf("missing context: %q", got)
	}

	// Long output is truncated to a bounded tail.
	long := strings.Repeat("x", 2000)
	got = formatTunnelError(portErr, long)
	if len(got) > 700 {
		t.Errorf("not truncated: len=%d", len(got))
	}
	if !strings.Contains(got, "...") {
		t.Errorf("truncated output should mark with ...: %q", got)
	}
}
