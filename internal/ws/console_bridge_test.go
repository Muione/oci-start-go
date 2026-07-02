// Package ws — console_bridge_test.go: regression for the
// "close of closed channel" panic in HandleVNCBridge's bidirectional copy.
// Both copy goroutines used to defer close(done); when one exited + the
// handler returned (closing the conns), the other's Read returned and its
// defer close(done) panicked. bridgeVNC guards the close with sync.Once.
package ws

import (
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestBridgeVNC_NoPanicOnClose: closing one side (then the other) must not
// panic. With the old double-close(done) it crashed the process. Also verifies
// binary bytes flow client -> TCP peer through the bridge.
func TestBridgeVNC_NoPanicOnClose(t *testing.T) {
	client, server := newConnPair(t)
	a, b := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = b.Close() }()
	defer func() { _ = server.Close() }()
	defer func() { _ = a.Close() }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		bridgeVNC(server, a)
	}()

	// Binary byte flow: client (WS) -> b (TCP) via the bridge.
	if err := client.WriteMessage(websocket.BinaryMessage, []byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 2)
	_ = b.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, err := b.Read(buf); err != nil || n != 2 || string(buf) != "hi" {
		t.Fatalf("b.Read n=%d err=%v buf=%q, want \"hi\"", n, err, buf)
	}

	// Close both peers so both copy goroutines exit. With the old code the
	// second goroutine's defer close(done) panicked here.
	_ = client.Close()
	_ = b.Close()

	select {
	case <-done:
		// bridgeVNC returned cleanly — no panic.
	case <-time.After(2 * time.Second):
		t.Fatal("bridgeVNC did not return within 2s (goroutine leak?)")
	}
}
