// Package ws — rescue_test.go: tests for rescue deps race + conn-write race (C4)
// and init-overwrite-old-flow leak (C5).
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestRescue_SetDeps_NoRaceWithFlow verifies that swapping h.deps via SetDeps
// while a rescue flow goroutine is running does not trigger a race: the flow
// must read deps from the value captured at start, not from h.deps.
func TestRescue_SetDeps_NoRaceWithFlow(t *testing.T) {
	h := NewHub().Rescue
	h.active = make(map[string]*rescueFlow)

	client, server := newConnPair(t)
	drainConn(t, client)
	sc := &safeConn{c: server}

	release := make(chan struct{})
	deps := &RescueDeps{
		StopInstance: func(string, int64) error { return nil },
		GetInstance:  func(string, int64) (*RescueInstanceInfo, error) { return nil, errors.New("test-get-failure") },
		CheckAndEnableRule: func(ctx context.Context, tenantID int64) error {
			<-release // block the flow so it stays alive while we swap deps
			return nil
		},
	}
	h.SetDeps(deps)

	flow := &rescueFlow{
		InstanceID: "i-1",
		Cancel:     make(chan struct{}),
		done:       make(chan struct{}),
		sc:         sc,
	}
	h.mu.Lock()
	h.active["i-1"] = flow
	h.mu.Unlock()

	go h.runRescueFlow(sc, flow, deps, 1, 0, "")
	time.Sleep(50 * time.Millisecond) // let the goroutine enter CheckAndEnableRule

	// Concurrent SetDeps swaps must not race with the running flow.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.SetDeps(&RescueDeps{})
		}()
	}
	wg.Wait()

	close(release) // unblock → GetInstance errors → flow returns
	select {
	case <-flow.done:
	case <-time.After(2 * time.Second):
		t.Fatal("runRescueFlow did not exit after release")
	}
}

// TestRescue_ConcurrentConnWrites_NoPanic verifies the conn-write race between
// the rescue flow's send() and the main loop's handleStatus/handleCancel is
// eliminated: both write through the same safeConn, so concurrent writes are
// serialized and cannot panic gorilla or trip -race.
func TestRescue_ConcurrentConnWrites_NoPanic(t *testing.T) {
	h := NewHub().Rescue
	h.active = make(map[string]*rescueFlow)

	client, server := newConnPair(t)
	drainConn(t, client)
	sc := &safeConn{c: server}
	flow := &rescueFlow{
		InstanceID: "i-1",
		Cancel:     make(chan struct{}),
		done:       make(chan struct{}),
		sc:         sc,
	}
	h.mu.Lock()
	h.active["i-1"] = flow
	h.mu.Unlock()

	var wg sync.WaitGroup
	// send() path (runRescueFlow goroutine writes status).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = sc.writeJSON(RescueStatus{Step: "send", InstanceID: "i-1"})
		}
	}()
	// handleStatus/handleCancel path (main loop writes).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = sc.writeJSON(map[string]string{"type": "status"})
		}
	}()
	wg.Wait()

	_ = websocket.TextMessage // keep import in case of future edits
}

// TestRescue_InitReplacesOldFlow verifies a second init for the same instance
// closes the previous flow's Cancel and lets its goroutine exit (no leak),
// mirroring console.go's old-session cleanup.
func TestRescue_InitReplacesOldFlow(t *testing.T) {
	h := NewHub().Rescue
	h.active = make(map[string]*rescueFlow)

	client, server := newConnPair(t)
	drainConn(t, client)
	sc := &safeConn{c: server}

	release := make(chan struct{})
	deps := &RescueDeps{
		StopInstance: func(string, int64) error { return nil },
		GetInstance: func(string, int64) (*RescueInstanceInfo, error) {
			return &RescueInstanceInfo{
				ID: "i-1", State: "RUNNING", BootVolumeID: "bv",
				DisplayName: "x", PublicIP: "1.2.3.4",
				SSHUsername: "root", SSHPassword: "p",
			}, nil
		},
		CheckAndEnableRule: func(ctx context.Context, tenantID int64) error {
			<-release // keep the flow alive until we say go
			return nil
		},
	}
	h.SetDeps(deps)

	initData, _ := json.Marshal(map[string]any{"instanceId": "i-1", "tenantId": 1})

	// First init → flow1.
	h.handleInit(sc, initData)
	h.mu.Lock()
	flow1 := h.active["i-1"]
	h.mu.Unlock()
	if flow1 == nil {
		t.Fatal("flow1 not created")
	}
	time.Sleep(50 * time.Millisecond) // let flow1 enter CheckAndEnableRule

	// Second init for the same instance must close flow1.Cancel and replace it.
	h.handleInit(sc, initData)
	h.mu.Lock()
	flow2 := h.active["i-1"]
	h.mu.Unlock()
	if flow2 == nil || flow2 == flow1 {
		t.Fatal("second init did not replace flow1")
	}
	select {
	case <-flow1.Cancel:
	default:
		t.Fatal("flow1.Cancel not closed by second init")
	}

	// Stop flow2 too so it doesn't leak past the test, then release both.
	close(flow2.Cancel)
	close(release)

	select {
	case <-flow1.done:
	case <-time.After(2 * time.Second):
		t.Fatal("flow1 goroutine did not exit after Cancel was closed")
	}
	select {
	case <-flow2.done:
	case <-time.After(4 * time.Second): // flow2 may be mid poll-sleep
		t.Fatal("flow2 goroutine did not exit")
	}
}

// TestRescue_CompleteRescue_StopFailsAndTenantID verifies CompleteRescue (E7):
// a Stop failure sends an Error status and aborts (no Detach), and the real
// flow tenantID (non-zero) is passed to OCI calls instead of 0.
func TestRescue_CompleteRescue_StopFailsAndTenantID(t *testing.T) {
	h := NewHub().Rescue
	h.active = make(map[string]*rescueFlow)

	client, server := newConnPair(t)
	sc := &safeConn{c: server}

	var mu sync.Mutex
	detachCalled := false
	var stopTenantID int64
	deps := &RescueDeps{
		StopInstance: func(id string, tid int64) error {
			mu.Lock()
			stopTenantID = tid
			mu.Unlock()
			return errors.New("stop-boom")
		},
		DetachBootVolume: func(id string, tid int64) (string, error) {
			mu.Lock()
			detachCalled = true
			mu.Unlock()
			return "", nil
		},
		AttachBootVolume: func(id string, tid int64, bv string) error { return nil },
		StartInstance:    func(id string, tid int64) error { return nil },
		GetInstance:      func(id string, tid int64) (*RescueInstanceInfo, error) { return &RescueInstanceInfo{ID: id, State: "STOPPED"}, nil },
	}
	h.SetDeps(deps)

	flow := &rescueFlow{
		InstanceID:     "i-1",
		Cancel:         make(chan struct{}),
		done:           make(chan struct{}),
		sc:             sc,
		tenantID:       7,
		OriginalBootID: "bv-orig",
		RescueBootID:   "bv-rescue",
	}
	h.mu.Lock()
	h.active["i-1"] = flow
	h.mu.Unlock()

	h.CompleteRescue(server, "i-1")

	// Read buffered messages; expect an error status.
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	sawError := false
	for {
		_, msg, err := client.ReadMessage()
		if err != nil {
			break
		}
		var st RescueStatus
		if json.Unmarshal(msg, &st) == nil && st.Step == "error" && st.Error != "" {
			sawError = true
		}
	}
	if !sawError {
		t.Fatal("CompleteRescue did not send an Error status after Stop failure")
	}
	mu.Lock()
	didDetach := detachCalled
	tid := stopTenantID
	mu.Unlock()
	if didDetach {
		t.Fatal("DetachBootVolume was called after Stop failure (should have aborted)")
	}
	if tid != 7 {
		t.Fatalf("StopInstance tenantID = %d, want 7 (non-zero from flow)", tid)
	}
}
