// Package ws — console_serial_test.go: tests HandleSerialConsole's setup
// error paths. The interactive ssh bridge (WS↔stdin/stdout) is simple io
// piping + is exercised end-to-end only with a real OCI console; the setup
// logic (ensureSerialConn resume/create) reuses the tested resume + create
// patterns.
package ws

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func newSerialDeps(lookupFail bool) *ConsoleDeps {
	return &ConsoleDeps{
		Logger: zerolog.Nop(),
		InstanceLookup: func(instanceID string) (*ConsoleInstanceInfo, error) {
			if lookupFail {
				return nil, errors.New("not found")
			}
			return &ConsoleInstanceInfo{InstanceID: instanceID, TenantID: 1, CompartmentID: "comp-x"}, nil
		},
	}
}

// TestHandleSerialConsole_LookupFail: a failed instance lookup surfaces an
// error JSON to the WS client (not a panic / silent close).
func TestHandleSerialConsole_LookupFail(t *testing.T) {
	h := &ConsoleHandler{deps: newSerialDeps(true)}
	s := httptest.NewServer(http.HandlerFunc(h.HandleSerialConsole))
	t.Cleanup(s.Close)

	u := strings.Replace(s.URL, "http", "ws", 1) + "/?" + url.Values{"instanceId": {"i-1"}}.Encode()
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	// Skip progress "output" frames; read until the error frame.
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var m struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read waiting for error: %v", err)
		}
		if json.Unmarshal(msg, &m) != nil {
			continue
		}
		if m.Type == "error" {
			break
		}
	}
	if !strings.Contains(m.Message, "instance lookup failed") {
		t.Errorf("msg=%q, want 'instance lookup failed'", m.Message)
	}
}

// TestHandleSerialConsole_NoInstanceID: missing instanceId → 400 (no upgrade).
func TestHandleSerialConsole_NoInstanceID(t *testing.T) {
	h := &ConsoleHandler{deps: newSerialDeps(false)}
	s := httptest.NewServer(http.HandlerFunc(h.HandleSerialConsole))
	t.Cleanup(s.Close)

	// Dial without instanceId — the server returns 400 before upgrading, so
	// the WS dial fails (that's the assertion).
	_, _, err := websocket.DefaultDialer.Dial(s.URL+"/", nil)
	if err == nil {
		t.Fatal("dial without instanceId should fail (400, no WS upgrade)")
	}
}
