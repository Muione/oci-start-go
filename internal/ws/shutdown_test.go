// Package ws — shutdown_test.go: tests for Hub.Shutdown covering Console
// (process kill + PEM cleanup) and Rescue (flow Cancel close + goroutine exit).
package ws

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newConnPair returns a client/server websocket conn pair from a test server.
// The server side is handed off immediately; it blocks until cleanup so the
// test can drive the server conn directly. Cleanup closes both conns and the
// server (LIFO: release before httptest.Server.Close so it doesn't hang).
func newConnPair(t *testing.T) (client, server *websocket.Conn) {
	t.Helper()
	srvCh := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		srvCh <- c
		<-release
		_ = c.Close()
	}))
	t.Cleanup(func() { close(release); s.Close() })
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	select {
	case server = <-srvCh:
	case <-time.After(time.Second):
		t.Fatal("server did not upgrade in time")
	}
	return c, server
}

// TestHubShutdown_Console verifies Hub.Shutdown kills the console SSH tunnel
// process and deletes the PEM key file for an active session.
func TestHubShutdown_Console(t *testing.T) {
	h := NewHub()
	h.Console.active = make(map[string]*consoleSession)

	tmpFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(tmpFile, []byte("PRIVATE KEY"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h.Console.active["i-1"] = &consoleSession{
		instanceID:  "i-1",
		sshCmd:      cmd,
		keyFilePath: tmpFile,
		cancel:      make(chan struct{}),
	}

	h.Shutdown()

	// Kill+Wait reaps the process; ProcessState is set once Wait returns.
	// A SIGKILL'd process is dead but ProcessState.Exited()==false (use Signaled),
	// so a non-nil ProcessState is the reliable "killed/reaped" signal here.
	if cmd.ProcessState == nil {
		t.Fatal("console SSH tunnel process was not killed on shutdown")
	}
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Fatal("console PEM key file was not removed on shutdown")
	}
}

// TestHubShutdown_Rescue verifies Hub.Shutdown closes each rescue flow's
// Cancel channel and the flow goroutine exits.
func TestHubShutdown_Rescue(t *testing.T) {
	h := NewHub()
	h.Rescue.active = make(map[string]*rescueFlow)

	flow := &rescueFlow{
		InstanceID: "i-1",
		Cancel:     make(chan struct{}),
		done:       make(chan struct{}),
	}
	h.Rescue.active["i-1"] = flow
	// Stand in for runRescueFlow: a goroutine that exits when Cancel closes.
	go func() {
		<-flow.Cancel
		close(flow.done)
	}()

	h.Shutdown()

	select {
	case <-flow.Cancel:
	default:
		t.Fatal("rescue flow Cancel channel not closed on shutdown")
	}
	select {
	case <-flow.done:
	case <-time.After(2 * time.Second):
		t.Fatal("rescue goroutine did not exit after Cancel was closed")
	}
}
