// Package ws — monitor_test.go: tests for monitor Broadcast race + dead-conn
// head-of-line blocking (C3).
package ws

import (
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func drainConn(t *testing.T, c *websocket.Conn) {
	t.Helper()
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// TestMonitor_Broadcast_NoRace exercises -race: Broadcast writes to a conn
// while the HandleMonitor read loop's pong path writes the same conn.
// Serialized via safeConn, this must not race.
func TestMonitor_Broadcast_NoRace(t *testing.T) {
	h := NewMonitorHandler()
	client, server := newConnPair(t)
	drainConn(t, client)
	sc := &safeConn{c: server}
	h.add(sc)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			h.Broadcast(MonitorReportDTO{ServerID: "s", CPUUsage: 1.5})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = sc.writeMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
		}
	}()
	wg.Wait()
}

// TestMonitor_DeadConnNoBlock verifies a dead conn does not block Broadcast
// from delivering to other conns (no head-of-line blocking).
func TestMonitor_DeadConnNoBlock(t *testing.T) {
	h := NewMonitorHandler()

	liveClient, liveServer := newConnPair(t)
	live := &safeConn{c: liveServer}
	h.add(live)

	deadClient, deadServer := newConnPair(t)
	_ = deadClient.Close()
	_ = deadServer.Close() // dead: writes fail immediately
	dead := &safeConn{c: deadServer}
	h.add(dead)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Broadcast(MonitorReportDTO{ServerID: "s"})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked on dead conn (head-of-line)")
	}

	// Live conn must have received the broadcast.
	_ = liveClient.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := liveClient.ReadMessage(); err != nil {
		t.Fatalf("live conn did not receive broadcast: %v", err)
	}
}
