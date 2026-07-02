// Package ws — console_test.go: tests for the console control-WS race (C2).
package ws

import (
	"sync"
	"testing"
)

// TestConsole_ControlWS_NoRace verifies the control WebSocket — written by the
// SSH-tunnel-wait goroutine (tunnel_closed) and the main read loop (pong /
// vnc_ready / errors) — is serialized so concurrent writes cannot race.
func TestConsole_ControlWS_NoRace(t *testing.T) {
	client, server := newConnPair(t)
	// Drain so writes never block on a full send buffer.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()

	sc := &safeConn{c: server}
	session := &consoleSession{instanceID: "i-1", controlWS: sc}

	var wg sync.WaitGroup
	// "tunnel_closed" notifier goroutine (mirrors console.go tunnel-wait goroutine).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = session.controlWS.writeJSON(map[string]string{"type": "tunnel_closed"})
		}
	}()
	// Main-loop writers (pong / vnc_ready / error).
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = sc.writeJSON(map[string]string{"type": "pong"})
			}
		}()
	}
	wg.Wait()
}
