// Package ws — safeconn_test.go: tests for safeConn write serialization.
package ws

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialTestWS dials a test websocket server whose handler runs srv(conn).
// Cleanup closes the server and the returned client conn.
func dialTestWS(t *testing.T, srv func(*websocket.Conn)) *websocket.Conn {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		srv(conn)
	}))
	t.Cleanup(s.Close)
	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func isTimeoutErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, os.ErrDeadlineExceeded)
}

// TestSafeConn_ConcurrentWrites exercises -race: many goroutines mixing
// writeMessage and writeJSON on the same safeConn must not panic or race.
func TestSafeConn_ConcurrentWrites(t *testing.T) {
	raw := dialTestWS(t, func(conn *websocket.Conn) {
		// Drain so the OS buffer never fills and writes never block on I/O.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	sc := &safeConn{c: raw}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = sc.writeMessage(websocket.TextMessage, []byte("hi"))
				_ = sc.writeJSON(map[string]string{"k": "v"})
			}
		}()
	}
	wg.Wait()
}

// TestSafeConn_WriteDeadlineExceeded verifies a write to a non-reading peer
// returns an error once the per-write deadline elapses.
func TestSafeConn_WriteDeadlineExceeded(t *testing.T) {
	// Server upgrades then never reads → client send buffer fills → write blocks.
	// It blocks on a release channel so the test can unblock it (and let
	// httptest.Server.Close() complete) in cleanup.
	release := make(chan struct{})
	raw := dialTestWS(t, func(conn *websocket.Conn) {
		<-release
	})
	t.Cleanup(func() { close(release) }) // LIFO: runs before s.Close() in dialTestWS
	sc := &safeConn{c: raw}

	old := writeTimeout
	writeTimeout = 10 * time.Millisecond
	t.Cleanup(func() { writeTimeout = old })

	big := bytes.Repeat([]byte("x"), 1<<20) // 1MB; exceeds typical socket send buffer
	var err error
	for i := 0; i < 50; i++ {
		if err = sc.writeMessage(websocket.TextMessage, big); err != nil {
			break
		}
	}
	if err == nil {
		t.Fatalf("expected deadline error, got nil")
	}
	if !isTimeoutErr(err) {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	// writeJSON must honor the same deadline behavior.
	var errj error
	for i := 0; i < 50; i++ {
		if errj = sc.writeJSON(bytes.Repeat([]byte("x"), 1<<20)); errj != nil {
			break
		}
	}
	if errj == nil || !isTimeoutErr(errj) {
		t.Fatalf("expected timeout error from writeJSON, got: %v", errj)
	}
}

// TestSafeConn_WriteJSON_Marshals checks the happy path: a JSON write is
// received intact by the peer and a closed conn surfaces an error.
func TestSafeConn_WriteJSON_HappyPath(t *testing.T) {
	got := make(chan []byte, 1)
	raw := dialTestWS(t, func(conn *websocket.Conn) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		got <- msg
	})
	sc := &safeConn{c: raw}
	if err := sc.writeJSON(map[string]string{"type": "ok"}); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	select {
	case msg := <-got:
		if !strings.Contains(string(msg), `"ok"`) || !strings.Contains(string(msg), `"type"`) {
			t.Fatalf("unexpected payload: %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("peer did not receive message")
	}
	// Closing the underlying conn makes subsequent writes error.
	_ = raw.Close()
	// Give the closed conn a moment to propagate; then a write must error.
	// Use a short deadline so we don't hang if the close hasn't propagated.
	old := writeTimeout
	writeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { writeTimeout = old })
	if err := sc.writeMessage(websocket.TextMessage, []byte("x")); err == nil {
		t.Fatal("expected error on closed conn, got nil")
	}
	// silence unused io import if future edits drop a use.
	_ = io.Discard
}
