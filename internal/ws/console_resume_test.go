// Package ws — console_resume_test.go: tests handleResumeConnection's decision
// logic via the getConsoleConnectionInfo + deps seams (no live ComputeClient,
// no ssh exec). The happy path (ACTIVE -> startTunnelAndNotify) is shared with
// create and covered by build; here we lock the error paths that decide
// whether a saved connection is resumable.
package ws

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// stubGetConsoleConn swaps the package-level get seam. Restored on cleanup.
func stubGetConsoleConn(t *testing.T, fn func(context.Context, oci.Clients, string) (*oci.ConsoleConnectionInfo, error)) {
	t.Helper()
	prev := getConsoleConnectionInfo
	getConsoleConnectionInfo = fn
	t.Cleanup(func() { getConsoleConnectionInfo = prev })
}

func resumeDeps(load func(context.Context, string) (string, string, error)) *ConsoleDeps {
	return &ConsoleDeps{
		Logger: zerolog.Nop(),
		InstanceLookup: func(instanceID string) (*ConsoleInstanceInfo, error) {
			return &ConsoleInstanceInfo{InstanceID: instanceID, TenantID: 1, DisplayName: "n", Shape: "s"}, nil
		},
		BuildClients:          func(context.Context, int64) (oci.Clients, error) { return oci.Clients{}, nil },
		LoadConsoleConnection: load,
	}
}

// readUntilType reads messages (discarding progress "output" frames) until it
// finds one of the wanted type, or the deadline. Single reader — no competing
// drain goroutine (which would race for the same frames).
func readUntilType(t *testing.T, c *websocket.Conn, wantType string, deadline time.Duration) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(deadline))
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read waiting for %q: %v", wantType, err)
		}
		var m map[string]any
		if err := json.Unmarshal(msg, &m); err != nil {
			continue
		}
		if m["type"] == wantType {
			return m
		}
	}
}

// TestHandleResumeConnection_NoSavedConnection: Load returns ErrNoRows ->
// the frontend is told there's nothing to resume.
func TestHandleResumeConnection_NoSavedConnection(t *testing.T) {
	stubGetConsoleConn(t, func(context.Context, oci.Clients, string) (*oci.ConsoleConnectionInfo, error) {
		t.Fatal("get should not be called when Load fails")
		return nil, nil
	})
	client, server := newConnPair(t)
	sc := &safeConn{c: server}
	h := &ConsoleHandler{deps: resumeDeps(func(context.Context, string) (string, string, error) {
		return "", "", sql.ErrNoRows
	})}
	data, _ := json.Marshal(map[string]string{"instanceId": "i-1"})
	h.handleResumeConnection(sc, data)
	m := readUntilType(t, client, "error", 2*time.Second)
	if msg, _ := m["message"].(string); msg == "" || !strContains(msg, "无可恢复") {
		t.Errorf("message=%v want '无可恢复'", m["message"])
	}
}

// TestHandleResumeConnection_NotActive: the saved connection is no longer
// ACTIVE (e.g. DELETED) -> tell the user to delete + create new.
func TestHandleResumeConnection_NotActive(t *testing.T) {
	var calls int32
	stubGetConsoleConn(t, func(_ context.Context, _ oci.Clients, connID string) (*oci.ConsoleConnectionInfo, error) {
		atomic.AddInt32(&calls, 1)
		if connID != "conn-1" {
			t.Errorf("get connID=%q want conn-1", connID)
		}
		return &oci.ConsoleConnectionInfo{ID: "conn-1", LifecycleState: "DELETED"}, nil
	})
	client, server := newConnPair(t)
	sc := &safeConn{c: server}
	h := &ConsoleHandler{deps: resumeDeps(func(context.Context, string) (string, string, error) {
		return "conn-1", "PEM", nil
	})}
	data, _ := json.Marshal(map[string]string{"instanceId": "i-1"})
	h.handleResumeConnection(sc, data)
	m := readUntilType(t, client, "error", 2*time.Second)
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("get called %d times, want 1", calls)
	}
	if msg, _ := m["message"].(string); msg == "" || !strContains(msg, "DELETED") {
		t.Errorf("message=%v want 'DELETED'", m["message"])
	}
}

// TestHandleResumeConnection_GetError: Get fails -> surface a fetch error.
func TestHandleResumeConnection_GetError(t *testing.T) {
	stubGetConsoleConn(t, func(context.Context, oci.Clients, string) (*oci.ConsoleConnectionInfo, error) {
		return nil, errors.New("boom")
	})
	client, server := newConnPair(t)
	sc := &safeConn{c: server}
	h := &ConsoleHandler{deps: resumeDeps(func(context.Context, string) (string, string, error) {
		return "conn-1", "PEM", nil
	})}
	data, _ := json.Marshal(map[string]string{"instanceId": "i-1"})
	h.handleResumeConnection(sc, data)
	m := readUntilType(t, client, "error", 2*time.Second)
	if msg, _ := m["message"].(string); msg == "" || !strContains(msg, "fetch console connection failed") {
		t.Errorf("message=%v want fetch error", m["message"])
	}
}

// TestHandleResumeConnection_EmptyPEM: a row with no key -> can't resume.
func TestHandleResumeConnection_EmptyPEM(t *testing.T) {
	stubGetConsoleConn(t, func(context.Context, oci.Clients, string) (*oci.ConsoleConnectionInfo, error) {
		t.Fatal("get should not be called when key is empty")
		return nil, nil
	})
	client, server := newConnPair(t)
	sc := &safeConn{c: server}
	h := &ConsoleHandler{deps: resumeDeps(func(context.Context, string) (string, string, error) {
		return "conn-1", "", nil
	})}
	data, _ := json.Marshal(map[string]string{"instanceId": "i-1"})
	h.handleResumeConnection(sc, data)
	m := readUntilType(t, client, "error", 2*time.Second)
	if msg, _ := m["message"].(string); msg == "" || !strContains(msg, "key is missing") {
		t.Errorf("message=%v want 'key is missing'", m["message"])
	}
}

// strContains is a tiny helper (the test file doesn't otherwise import strings).
func strContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
