// Package oci — console_ensure_test.go: tests for the console-connection
// lifecycle recovery that fixes the 409 IncorrectState ("already exists or has
// not been terminated") returned by CreateInstanceConsoleConnection when a
// previous connection lingers.
//
// OCI allows only ONE console connection per instance at a time and Delete is
// asynchronous (DELETING -> DELETED). Creating before the prior connection
// reaches DELETED yields 409. The fix: clear -> wait-cleared -> create, and on
// 409 retry the same once.
package oci

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeOCIError implements the oci-sdk ServiceError shape (GetHTTPStatusCode +
// GetCode) so isConsoleConflict can detect it via errors.As.
type fakeOCIError struct {
	status int
	code   string
}

func (e fakeOCIError) Error() string          { return e.code }
func (e fakeOCIError) GetHTTPStatusCode() int { return e.status }
func (e fakeOCIError) GetCode() string        { return e.code }

// wrap409 wraps a 409 fakeOCIError with %w, mirroring how GenerateConsoleConnection
// wraps the SDK error: fmt.Errorf("create console connection for %s: %w", ...).
func wrap409(e fakeOCIError) error { return fmt.Errorf("create console connection: %w", e) }

// --- isConsoleConflict ---

func TestIsConsoleConflict(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"409 IncorrectState direct", fakeOCIError{409, "IncorrectState"}, true},
		{"409 IncorrectState wrapped %w", wrap409(fakeOCIError{409, "IncorrectState"}), true},
		{"404 NotAuthorizedOrNotFound", fakeOCIError{404, "NotAuthorizedOrNotFound"}, false},
		{"409 LimitExceeded", fakeOCIError{409, "LimitExceeded"}, false},
		{"500 InternalError", fakeOCIError{500, "InternalError"}, false},
		{"fallback string match", fmt.Errorf("Http Status Code: 409. Error Code: IncorrectState."), true},
		{"unrelated error", errors.New("network timeout"), false},
	}
	for _, c := range cases {
		if got := isConsoleConflict(c.err); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// --- waitForCleared ---

func TestWaitForCleared_PollsUntilTerminal(t *testing.T) {
	var calls int32
	list := func(context.Context) ([]ConsoleConnection, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return []ConsoleConnection{{ID: "c1", LifecycleState: "DELETING"}}, nil // still terminating
		}
		return []ConsoleConnection{{ID: "c1", LifecycleState: "DELETED"}}, nil // terminal
	}
	if err := waitForCleared(context.Background(), list, 1*time.Millisecond, 2*time.Second); err != nil {
		t.Fatalf("got err %v want nil", err)
	}
	if calls < 2 {
		t.Errorf("expected >=2 list calls, got %d", calls)
	}
}

func TestWaitForCleared_EmptyImmediately(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, nil }
	if err := waitForCleared(context.Background(), list, 1*time.Millisecond, 1*time.Second); err != nil {
		t.Fatalf("got err %v want nil", err)
	}
}

func TestWaitForCleared_Timeout(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) {
		return []ConsoleConnection{{ID: "c1", LifecycleState: "DELETING"}}, nil // never clears
	}
	err := waitForCleared(context.Background(), list, 2*time.Millisecond, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("got %v want timeout error", err)
	}
}

func TestWaitForCleared_ListError(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, errors.New("boom") }
	err := waitForCleared(context.Background(), list, 1*time.Millisecond, 1*time.Second)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("got %v want list error", err)
	}
}

// --- clearAll ---

func TestClearAll_DeletesNonTerminal_SkipsTerminal(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) {
		return []ConsoleConnection{
			{ID: "active", LifecycleState: "ACTIVE"},
			{ID: "creating", LifecycleState: "CREATING"},
			{ID: "deleting", LifecycleState: "DELETING"},
			{ID: "deleted", LifecycleState: "DELETED"},
		}, nil
	}
	var deleted []string
	del := func(_ context.Context, id string) error { deleted = append(deleted, id); return nil }
	if err := clearAll(context.Background(), list, del); err != nil {
		t.Fatalf("got err %v want nil", err)
	}
	if len(deleted) != 2 || deleted[0] != "active" || deleted[1] != "creating" {
		t.Errorf("deleted = %v, want [active creating]", deleted)
	}
}

// --- waitForActive ---

func TestWaitForActive_ActiveImmediately(t *testing.T) {
	get := func(_ context.Context, _ string) (*ConsoleConnectionInfo, error) {
		return &ConsoleConnectionInfo{ID: "c1", LifecycleState: "ACTIVE"}, nil
	}
	info, err := waitForActive(context.Background(), get, "c1", 1*time.Millisecond, 1*time.Second)
	if err != nil || info == nil || info.ID != "c1" {
		t.Fatalf("got %v %v want c1 nil", info, err)
	}
}

func TestWaitForActive_TerminalFailed(t *testing.T) {
	get := func(_ context.Context, _ string) (*ConsoleConnectionInfo, error) {
		return &ConsoleConnectionInfo{ID: "c1", LifecycleState: "FAILED"}, nil
	}
	_, err := waitForActive(context.Background(), get, "c1", 1*time.Millisecond, 1*time.Second)
	if err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("got %v want FAILED terminal error", err)
	}
}

// --- ensureConsoleConnection ---

// TestEnsureConsoleConnection_ClearsBeforeCreate: a leftover ACTIVE connection
// must be deleted and the clear must settle BEFORE create is called. This is
// the exact bug — creating immediately after Delete races the async DELETING
// state and yields 409.
func TestEnsureConsoleConnection_ClearsBeforeCreate(t *testing.T) {
	var muSeq struct{ order []string }
	addSeq := func(s string) { muSeq.order = append(muSeq.order, s) }

	var listCalls int32
	list := func(context.Context) ([]ConsoleConnection, error) {
		n := atomic.AddInt32(&listCalls, 1)
		if n == 1 {
			return []ConsoleConnection{{ID: "old", LifecycleState: "ACTIVE"}}, nil
		}
		return []ConsoleConnection{{ID: "old", LifecycleState: "DELETED"}}, nil // cleared
	}
	del := func(_ context.Context, id string) error { addSeq("del:" + id); return nil }
	create := func(context.Context) (*ConsoleConnectionInfo, error) {
		addSeq("create")
		return &ConsoleConnectionInfo{ID: "new", LifecycleState: "ACTIVE"}, nil
	}
	get := func(_ context.Context, id string) (*ConsoleConnectionInfo, error) {
		t.Fatalf("get should not be called when create returns ACTIVE")
		return nil, nil
	}

	info, err := ensureConsoleConnection(context.Background(), consoleOps{list, create, del, get}, 1*time.Millisecond, 2*time.Second)
	if err != nil {
		t.Fatalf("got err %v want nil", err)
	}
	if info == nil || info.ID != "new" {
		t.Fatalf("got info %v want new", info)
	}
	// del must come before create.
	di, ci := -1, -1
	for i, s := range muSeq.order {
		if s == "del:old" {
			di = i
		}
		if s == "create" {
			ci = i
		}
	}
	if di < 0 || ci < 0 {
		t.Fatalf("missing del/create in order %v", muSeq.order)
	}
	if di > ci {
		t.Errorf("del (idx %d) must come before create (idx %d); order=%v", di, ci, muSeq.order)
	}
}

func TestEnsureConsoleConnection_NoLeftover(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, nil }
	var delCalled bool
	del := func(context.Context, string) error { delCalled = true; return nil }
	create := func(context.Context) (*ConsoleConnectionInfo, error) {
		return &ConsoleConnectionInfo{ID: "new", LifecycleState: "ACTIVE"}, nil
	}
	get := func(context.Context, string) (*ConsoleConnectionInfo, error) { return nil, nil }
	info, err := ensureConsoleConnection(context.Background(), consoleOps{list, create, del, get}, 1*time.Millisecond, 1*time.Second)
	if err != nil || info == nil || info.ID != "new" {
		t.Fatalf("got %v %v", info, err)
	}
	if delCalled {
		t.Error("del should not be called when no leftover")
	}
}

// TestEnsureConsoleConnection_409RetriesOnce: create returns 409 IncorrectState
// the first time (race / external leftover). The flow must clear+wait+retry
// once and succeed on the second create.
func TestEnsureConsoleConnection_409RetriesOnce(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, nil }
	var createCalls int32
	create := func(context.Context) (*ConsoleConnectionInfo, error) {
		if atomic.AddInt32(&createCalls, 1) == 1 {
			return nil, wrap409(fakeOCIError{409, "IncorrectState"}) // 409 race
		}
		return &ConsoleConnectionInfo{ID: "new", LifecycleState: "ACTIVE"}, nil
	}
	del := func(context.Context, string) error { return nil }
	get := func(context.Context, string) (*ConsoleConnectionInfo, error) { return nil, nil }

	info, err := ensureConsoleConnection(context.Background(), consoleOps{list, create, del, get}, 1*time.Millisecond, 2*time.Second)
	if err != nil {
		t.Fatalf("got err %v want nil", err)
	}
	if info == nil || info.ID != "new" {
		t.Fatalf("got info %v want new", info)
	}
	if createCalls != 2 {
		t.Errorf("create called %d times, want 2 (initial + 1 retry)", createCalls)
	}
}

// TestEnsureConsoleConnection_409TwiceGivesUp: if the retry also 409s, surface
// the error instead of looping forever.
func TestEnsureConsoleConnection_409TwiceGivesUp(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, nil }
	create := func(context.Context) (*ConsoleConnectionInfo, error) {
		return nil, wrap409(fakeOCIError{409, "IncorrectState"})
	}
	ops := consoleOps{list, create, func(context.Context, string) error { return nil }, func(context.Context, string) (*ConsoleConnectionInfo, error) { return nil, nil }}
	_, err := ensureConsoleConnection(context.Background(), ops, 1*time.Millisecond, 1*time.Second)
	if err == nil {
		t.Fatal("got nil err, want 409-after-retry error")
	}
	if !strings.Contains(err.Error(), "retry") && !strings.Contains(err.Error(), "409") {
		t.Errorf("err %q should mention retry/409", err)
	}
}

func TestEnsureConsoleConnection_Non409CreateError(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, nil }
	create := func(context.Context) (*ConsoleConnectionInfo, error) { return nil, errors.New("internal") }
	ops := consoleOps{list, create, func(context.Context, string) error { return nil }, func(context.Context, string) (*ConsoleConnectionInfo, error) { return nil, nil }}
	_, err := ensureConsoleConnection(context.Background(), ops, 1*time.Millisecond, 1*time.Second)
	if err == nil || !strings.Contains(err.Error(), "internal") {
		t.Fatalf("got %v want 'internal' error", err)
	}
}

// TestEnsureConsoleConnection_WaitsForActive: create returns CREATING; must
// poll get until ACTIVE.
func TestEnsureConsoleConnection_WaitsForActive(t *testing.T) {
	list := func(context.Context) ([]ConsoleConnection, error) { return nil, nil }
	create := func(context.Context) (*ConsoleConnectionInfo, error) {
		return &ConsoleConnectionInfo{ID: "new", LifecycleState: "CREATING"}, nil
	}
	var getCalls int32
	get := func(_ context.Context, _ string) (*ConsoleConnectionInfo, error) {
		n := atomic.AddInt32(&getCalls, 1)
		if n == 1 {
			return &ConsoleConnectionInfo{ID: "new", LifecycleState: "CREATING"}, nil
		}
		return &ConsoleConnectionInfo{ID: "new", LifecycleState: "ACTIVE"}, nil
	}
	info, err := ensureConsoleConnection(context.Background(), consoleOps{list, create, nil, get}, 1*time.Millisecond, 2*time.Second)
	if err != nil || info == nil || info.LifecycleState != "ACTIVE" {
		t.Fatalf("got %v %v", info, err)
	}
	if getCalls < 2 {
		t.Errorf("expected >=2 get calls, got %d", getCalls)
	}
}
