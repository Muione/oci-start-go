// Package ws — log_test.go: tests for log broadcast head-of-line blocking (C7).
package ws

import (
	"sync"
	"testing"
	"time"
)

// TestLog_Broadcast_DeadConnNoBlock verifies a dead conn does not block
// broadcast from delivering to other conns (no head-of-line blocking), and
// that the dead conn is dropped.
func TestLog_Broadcast_DeadConnNoBlock(t *testing.T) {
	h := NewLogHandler()
	h.mu.Lock()
	h.sessions = make(map[*safeConn]struct{})
	h.mu.Unlock()

	liveClient, liveServer := newConnPair(t)
	live := &safeConn{c: liveServer}

	deadClient, deadServer := newConnPair(t)
	_ = deadClient.Close()
	_ = deadServer.Close() // dead: writes fail immediately
	dead := &safeConn{c: deadServer}

	h.mu.Lock()
	h.sessions[live] = struct{}{}
	h.sessions[dead] = struct{}{}
	h.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.broadcast([]byte("hello"))
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on dead conn (head-of-line)")
	}

	// Live conn must have received the line.
	_ = liveClient.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := liveClient.ReadMessage(); err != nil {
		t.Fatalf("live conn did not receive broadcast: %v", err)
	}

	// Dead conn should have been removed.
	h.mu.Lock()
	_, stillDead := h.sessions[dead]
	h.mu.Unlock()
	if stillDead {
		t.Fatal("dead conn was not removed after failed write")
	}
}

// TestLog_Broadcast_ConcurrentAddRemove exercises -race: broadcast running
// concurrently with add/remove must not panic or race the session map.
func TestLog_Broadcast_ConcurrentAddRemove(t *testing.T) {
	h := NewLogHandler()
	h.mu.Lock()
	h.sessions = make(map[*safeConn]struct{})
	h.mu.Unlock()

	var conns []*safeConn
	for i := 0; i < 4; i++ {
		c, s := newConnPair(t)
		drainConn(t, c)
		conns = append(conns, &safeConn{c: s})
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			h.broadcast([]byte("x"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			h.addSession(conns[i%len(conns)])
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			h.removeSession(conns[i%len(conns)])
		}
	}()
	wg.Wait()
}
