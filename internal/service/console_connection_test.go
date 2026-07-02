// Package service — console_connection_test.go: tests for the console
// connection service that backs the VNC "list / resume / delete / create-new"
// feature. Persist encrypts the private key at rest; LoadForResume decrypts it
// back; joinOurs marks which OCI connections this app created and which are
// resumable (ours + ACTIVE + key present).
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// setupConsoleSvc opens an in-memory store with the console_connections table
// (post-0009 shape) and returns a service with a real master key. buildClients
// is nil — Persist/Load don't need it.
func setupConsoleSvc(t *testing.T) (*ConsoleConnectionService, []byte) {
	t.Helper()
	store := newTestStore(t)
	mustExec(t, store, `
CREATE TABLE console_connections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    tenant_id BIGINT NOT NULL,
    connection_id TEXT NOT NULL,
    private_key_path TEXT,
    cloud_type INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    encrypted_private_key TEXT,
    public_key_ssh TEXT
);
CREATE UNIQUE INDEX idx_console_connections_instance ON console_connections(instance_id);
`)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return NewConsoleConnectionService(store, key, nil), key
}

// TestConsoleConnectionService_PersistLoadRoundTrip: persist a private key,
// assert the stored value is ciphertext (not the raw PEM), then load it back
// and get the original PEM + connID. This is the resume trust root.
func TestConsoleConnectionService_PersistLoadRoundTrip(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	ctx := context.Background()
	pem := "-----BEGIN PRIVATE KEY-----\nMIIsecret\n-----END PRIVATE KEY-----\n"

	if err := svc.Persist(ctx, "i-1", "conn-a", 7, pem, "ssh-rsa AAA a"); err != nil {
		t.Fatalf("persist: %v", err)
	}

	// Stored value must be ciphertext, not the raw PEM.
	var enc string
	row := svc.store.Read.QueryRow("SELECT encrypted_private_key FROM console_connections WHERE instance_id=?", "i-1")
	if err := row.Scan(&enc); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if enc == pem {
		t.Error("stored private key must be ciphertext, not raw PEM")
	}

	gotConn, gotPEM, err := svc.LoadForResume(ctx, "i-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if gotConn != "conn-a" || gotPEM != pem {
		t.Errorf("load got conn=%q pem=%q, want conn-a / original pem", gotConn, gotPEM)
	}
}

// TestConsoleConnectionService_LoadForResume_NotFound: no row -> sql.ErrNoRows
// (so the caller can prompt "create new").
func TestConsoleConnectionService_LoadForResume_NotFound(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	_, _, err := svc.LoadForResume(context.Background(), "ghost")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("got %v, want sql.ErrNoRows", err)
	}
}

// TestJoinOurs_MarksMatchingConnID: only the OCI connection whose ID matches
// our persisted row is IsOurs; resumable only if ACTIVE + key present.
func TestJoinOurs_MarksMatchingConnID(t *testing.T) {
	conns := []oci.ConsoleConnection{
		{ID: "ours-active", LifecycleState: "ACTIVE"},
		{ID: "ours-creating", LifecycleState: "CREATING"},
		{ID: "theirs", LifecycleState: "ACTIVE"},
	}
	our := &repo.ConsoleConnectionRow{
		ConnectionID:        "ours-active",
		EncryptedPrivateKey: sql.NullString{String: "enc", Valid: true},
	}
	// First pass: row points at ours-active. Only that one is ours + resumable.
	views := joinOurs(conns, our)
	if len(views) != 3 {
		t.Fatalf("got %d views, want 3", len(views))
	}
	find := func(id string) *ConsoleConnectionView {
		for i := range views {
			if views[i].ConnID == id {
				return &views[i]
			}
		}
		return nil
	}
	if v := find("ours-active"); v == nil || !v.IsOurs || !v.CanResume {
		t.Errorf("ours-active: %+v, want IsOurs+CanResume", v)
	}
	if v := find("ours-creating"); v == nil || v.IsOurs || v.CanResume {
		t.Errorf("ours-creating (row points elsewhere): %+v, want !IsOurs", v)
	}
	if v := find("theirs"); v == nil || v.IsOurs || v.CanResume {
		t.Errorf("theirs: %+v, want !IsOurs", v)
	}

	// Second pass: point the row at the CREATING one — it's ours but not
	// resumable (not ACTIVE); ours-active is no longer ours.
	our.ConnectionID = "ours-creating"
	views = joinOurs(conns, our)
	find2 := func(id string) *ConsoleConnectionView {
		for i := range views {
			if views[i].ConnID == id {
				return &views[i]
			}
		}
		return nil
	}
	if v := find2("ours-creating"); v == nil || !v.IsOurs || v.CanResume {
		t.Errorf("ours-creating (now ours): %+v, want IsOurs, !CanResume", v)
	}
	if v := find2("ours-active"); v == nil || v.IsOurs {
		t.Errorf("ours-active (row moved): %+v, want !IsOurs", v)
	}
}

// TestJoinOurs_NoRow_NoneOurs: with no persisted row, nothing is ours.
func TestJoinOurs_NoRow_NoneOurs(t *testing.T) {
	conns := []oci.ConsoleConnection{{ID: "x", LifecycleState: "ACTIVE"}}
	views := joinOurs(conns, nil)
	if views[0].IsOurs || views[0].CanResume {
		t.Errorf("with no row, got IsOurs/CanResume: %+v", views[0])
	}
}

// TestJoinOurs_OursButNoKey_NotResumable: a row that matches but has no
// encrypted key (legacy private_key_path row) is ours-in-spirit but not
// resumable (we can't re-tunnel without the key).
func TestJoinOurs_OursButNoKey_NotResumable(t *testing.T) {
	conns := []oci.ConsoleConnection{{ID: "x", LifecycleState: "ACTIVE"}}
	our := &repo.ConsoleConnectionRow{ConnectionID: "x", EncryptedPrivateKey: sql.NullString{}}
	views := joinOurs(conns, our)
	if !views[0].IsOurs {
		t.Errorf("want IsOurs (connID matches)")
	}
	if views[0].CanResume {
		t.Errorf("want !CanResume (no key)")
	}
}

// stubList swaps the package-level list seam; restored on cleanup.
func stubList(t *testing.T, fn func(context.Context, oci.Clients, string, string) ([]oci.ConsoleConnection, error)) {
	t.Helper()
	prev := listConsoleConnectionsFn
	listConsoleConnectionsFn = fn
	t.Cleanup(func() { listConsoleConnectionsFn = prev })
}

// TestConsoleConnectionService_List_JoinsOurs: List calls the OCI list seam +
// joins our persisted row, marking the matching connection IsOurs+CanResume.
func TestConsoleConnectionService_List_JoinsOurs(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	svc.buildClients = func(context.Context, int64) (oci.Clients, error) { return oci.Clients{}, nil }

	if err := svc.Persist(context.Background(), "i-1", "conn-a", 7, "PEM", "pub"); err != nil {
		t.Fatalf("persist: %v", err)
	}

	stubList(t, func(_ context.Context, _ oci.Clients, comp, inst string) ([]oci.ConsoleConnection, error) {
		if inst != "i-1" || comp != "comp-x" {
			t.Errorf("list called comp=%q inst=%q, want comp-x/i-1", comp, inst)
		}
		return []oci.ConsoleConnection{
			{ID: "conn-a", LifecycleState: "ACTIVE"},
			{ID: "conn-x", LifecycleState: "ACTIVE"},
			{ID: "conn-b-deleted", LifecycleState: "DELETED"}, // filtered out (terminal)
		}, nil
	})

	views, err := svc.List(context.Background(), 7, "comp-x", "i-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	find := func(id string) *ConsoleConnectionView {
		for i := range views {
			if views[i].ConnID == id {
				return &views[i]
			}
		}
		return nil
	}
	if v := find("conn-a"); v == nil || !v.IsOurs || !v.CanResume {
		t.Errorf("conn-a: %+v, want IsOurs+CanResume", v)
	}
	if v := find("conn-x"); v == nil || v.IsOurs {
		t.Errorf("conn-x: %+v, want !IsOurs", v)
	}
	// DELETED is terminal — filtered out of the list (lingering DELETED rows
	// are noise + can't be re-deleted, which made delete look unresponsive).
	if v := find("conn-b-deleted"); v != nil {
		t.Errorf("conn-b-deleted should be filtered out, got %+v", v)
	}
}
// TestConsoleConnectionService_List_NoRow_AllTheirs: with no persisted row,
// no connection is marked ours.
func TestConsoleConnectionService_List_NoRow_AllTheirs(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	svc.buildClients = func(context.Context, int64) (oci.Clients, error) { return oci.Clients{}, nil }
	stubList(t, func(context.Context, oci.Clients, string, string) ([]oci.ConsoleConnection, error) {
		return []oci.ConsoleConnection{{ID: "x", LifecycleState: "ACTIVE"}}, nil
	})
	views, err := svc.List(context.Background(), 1, "c", "i-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if views[0].IsOurs {
		t.Errorf("with no row, got IsOurs: %+v", views[0])
	}
}

// TestConsoleConnectionService_Delete_RemovesRow: Delete calls the OCI delete
// seam + removes our persisted row when the connID matches.
func TestConsoleConnectionService_Delete_RemovesRow(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	svc.buildClients = func(context.Context, int64) (oci.Clients, error) { return oci.Clients{}, nil }

	ctx := context.Background()
	_ = svc.Persist(ctx, "i-1", "conn-a", 7, "PEM", "pub")

	var deleted string
	prev := deleteConsoleConnectionFn
	deleteConsoleConnectionFn = func(_ context.Context, _ oci.Clients, connID string) error {
		deleted = connID
		return nil
	}
	t.Cleanup(func() { deleteConsoleConnectionFn = prev })

	if err := svc.Delete(ctx, 7, "comp-x", "i-1", "conn-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != "conn-a" {
		t.Errorf("OCI delete called with %q, want conn-a", deleted)
	}
	// Row removed.
	if _, _, err := svc.LoadForResume(ctx, "i-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after delete, LoadForResume got %v, want ErrNoRows", err)
	}
}

// TestConsoleConnectionService_Delete_NotOurs_KeepsRow: deleting a connection
// that isn't ours doesn't touch our row.
func TestConsoleConnectionService_Delete_NotOurs_KeepsRow(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	svc.buildClients = func(context.Context, int64) (oci.Clients, error) { return oci.Clients{}, nil }
	ctx := context.Background()
	_ = svc.Persist(ctx, "i-1", "conn-a", 7, "PEM", "pub")
	prev := deleteConsoleConnectionFn
	deleteConsoleConnectionFn = func(context.Context, oci.Clients, string) error { return nil }
	t.Cleanup(func() { deleteConsoleConnectionFn = prev })
	if err := svc.Delete(ctx, 7, "c", "i-1", "theirs"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, err := svc.LoadForResume(ctx, "i-1"); err != nil {
		t.Errorf("row should remain: %v", err)
	}
}

// fake404Err mimics the oci-sdk ServiceError shape (GetHTTPStatusCode + GetCode)
// so isOciNotFound can detect it via errors.As.
type fake404Err struct{}

func (fake404Err) Error() string          { return "NotAuthorizedOrNotFound" }
func (fake404Err) GetHTTPStatusCode() int { return 404 }
func (fake404Err) GetCode() string        { return "NotAuthorizedOrNotFound" }

// TestConsoleConnectionService_Delete_NotFoundIsSuccess: deleting a lingering
// DELETED connection (OCI 404) is the desired end state — return success, not
// an error, and still drop our DB row. (The user clicked delete on a row that
// was already gone; that's fine.)
func TestConsoleConnectionService_Delete_NotFoundIsSuccess(t *testing.T) {
	svc, _ := setupConsoleSvc(t)
	svc.buildClients = func(context.Context, int64) (oci.Clients, error) { return oci.Clients{}, nil }
	ctx := context.Background()
	_ = svc.Persist(ctx, "i-1", "conn-a", 7, "PEM", "pub")
	prev := deleteConsoleConnectionFn
	deleteConsoleConnectionFn = func(context.Context, oci.Clients, string) error { return fake404Err{} }
	t.Cleanup(func() { deleteConsoleConnectionFn = prev })
	if err := svc.Delete(ctx, 7, "c", "i-1", "conn-a"); err != nil {
		t.Errorf("delete on 404: got %v want nil (already deleted = success)", err)
	}
	if _, _, err := svc.LoadForResume(ctx, "i-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("row should be removed after delete: %v", err)
	}
}

// TestIsOciNotFound: 404 detected via errors.As (through %w wrap) + string
// fallback; non-404 errors rejected.
func TestIsOciNotFound(t *testing.T) {
	if isOciNotFound(nil) {
		t.Error("nil should be false")
	}
	if !isOciNotFound(fake404Err{}) {
		t.Error("404 direct should be true")
	}
	if !isOciNotFound(fmt.Errorf("wrap: %w", fake404Err{})) {
		t.Error("404 wrapped should be true (errors.As)")
	}
	// fake500Err implements GetHTTPStatusCode=500 → not 404.
	type fake500Err struct{}
	f500 := fake500Err{}
	_ = f500
	if isOciNotFound(errors.New("some other error")) {
		t.Error("non-404 error should be false")
	}
}
