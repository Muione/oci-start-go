package auth

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	_ "modernc.org/sqlite"
)

// newAuthStore returns a *db.Store backed by a temp-file SQLite with the
// login_session, ban_record, and tenant tables created (matching the prod
// schema the auth package queries). Both pools share the single *sql.DB;
// tests are sequential so no WAL/locking concerns.
func newAuthStore(t *testing.T) *db.Store {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "auth.db")
	w, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, ddl := range []string{
		`CREATE TABLE login_session (
			token          TEXT PRIMARY KEY,
			username       TEXT NOT NULL,
			ip             TEXT,
			user_agent     TEXT,
			created_at     TEXT NOT NULL,
			expires_at     TEXT NOT NULL,
			last_active_at TEXT NOT NULL
		)`,
		`CREATE TABLE ban_record (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip_address TEXT NOT NULL,
			source TEXT,
			operator_name TEXT,
			reason TEXT,
			status INTEGER NOT NULL,
			create_time TEXT NOT NULL,
			unban_time TEXT,
			remark TEXT
		)`,
		`CREATE TABLE tenant (
			id INTEGER PRIMARY KEY,
			tenant_id TEXT,
			user_name TEXT,
			fingerprint TEXT,
			tenancy TEXT,
			region TEXT,
			key_file TEXT,
			created_at TEXT,
			api_synced INTEGER,
			enable_icmp INTEGER,
			enable_all_protocol INTEGER,
			is_home_region INTEGER,
			paren_id BIGINT,
			tenancy_name TEXT,
			tenancy_des TEXT,
			account_type TEXT,
			cloud_type INTEGER DEFAULT 1,
			region_en TEXT,
			id_str TEXT,
			email_address TEXT,
			email_enable INTEGER,
			transfer_status INTEGER DEFAULT 0,
			transfer_amount TEXT,
			is_active INTEGER DEFAULT 1,
			key_file_blob TEXT
		)`,
	} {
		if _, err := w.Exec(ddl); err != nil {
			_ = w.Close()
			t.Fatalf("create table: %v", err)
		}
	}
	t.Cleanup(func() { _ = w.Close() })
	return &db.Store{Write: w, Read: w}
}

// insertSessionRow inserts a session with explicit times for Validate tests.
func insertSessionRow(t *testing.T, store *db.Store, token, username, expiresAt, lastActiveAt string) {
	t.Helper()
	if err := repo.New(store.Write).InsertSession(context.Background(), repo.InsertSessionParams{
		Token:        token,
		Username:     username,
		CreatedAt:    lastActiveAt,
		ExpiresAt:    expiresAt,
		LastActiveAt: lastActiveAt,
	}); err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

// nowStr/futureStr/pastStr produce UTC wall-clocks matching the session
// storage format (Create/Touch write UTC). Parsing them back as UTC (in
// Validate) is zone-independent, so these tests don't depend on the machine TZ.
func nowStr() string                     { return time.Now().UTC().Format(timeFmt) }
func futureStr(d time.Duration) string   { return time.Now().UTC().Add(d).Format(timeFmt) }
func pastStr(d time.Duration) string     { return time.Now().UTC().Add(-d).Format(timeFmt) }

// B5: Create deletes prior sessions for the username (single session) and
// inserts a new row.
func TestCreate_DeletesPriorSessions(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()

	tok1, err := svc.Create(ctx, "alice", "1.2.3.4", "ua")
	if err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	// tok1 is validatable
	if _, _, ok := svc.Validate(ctx, tok1); !ok {
		t.Fatal("tok1 should be valid before second Create")
	}
	// second Create for same username must invalidate tok1 (single session)
	tok2, err := svc.Create(ctx, "alice", "1.2.3.4", "ua")
	if err != nil {
		t.Fatalf("Create #2: %v", err)
	}
	if tok1 == tok2 {
		t.Fatal("Create returned the same token twice; expected a new UUID")
	}
	if _, _, ok := svc.Validate(ctx, tok1); ok {
		t.Error("tok1 still valid after a new Create for the same username; single-session invariant broken")
	}
	if _, _, ok := svc.Validate(ctx, tok2); !ok {
		t.Error("tok2 should be valid")
	}
	// exactly one row for alice
	var n int
	row := store.Read.QueryRowContext(ctx, "SELECT COUNT(*) FROM login_session WHERE username = ?", "alice")
	if err := row.Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("login_session rows for alice = %d, want 1 (single session)", n)
	}
}

// B5: Validate returns false for an unknown token.
func TestValidate_UnknownToken_False(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	if _, _, ok := svc.Validate(context.Background(), "no-such-token"); ok {
		t.Error("unknown token should not validate")
	}
}

// B5: Validate returns false when the absolute expiry has passed.
func TestValidate_ExpiredAbsolute_False(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	insertSessionRow(t, store, "tok-abs", "bob", pastStr(time.Hour), nowStr()) // expired 1h ago, active now
	if _, _, ok := svc.Validate(context.Background(), "tok-abs"); ok {
		t.Error("absolutely-expired session should not validate")
	}
}

// B5: Validate returns false when last_active_at is older than the 2h active
// window (even if absolute expiry is still in the future).
func TestValidate_ExpiredActive_False(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	insertSessionRow(t, store, "tok-act", "bob", futureStr(30*24*time.Hour), pastStr(3*time.Hour)) // active 3h ago
	if _, _, ok := svc.Validate(context.Background(), "tok-act"); ok {
		t.Error("active-expired session (>2h) should not validate")
	}
}

// B5: Validate returns false when active window is exactly the boundary-ish.
func TestValidate_WithinActiveWindow_True(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	// 1h ago: within 2h active window, absolute 30d future
	insertSessionRow(t, store, "tok-ok", "carol", futureStr(30*24*time.Hour), pastStr(time.Hour))
	name, _, ok := svc.Validate(context.Background(), "tok-ok")
	if !ok {
		t.Error("session within active window should validate")
	}
	if name != "carol" {
		t.Errorf("username = %q, want carol", name)
	}
}

// B5: a freshly-created session validates and last_active ≈ now.
func TestValidate_Fresh_LastActiveNearNow(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()
	tok, err := svc.Create(ctx, "dave", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, last, ok := svc.Validate(ctx, tok)
	if !ok {
		t.Fatal("fresh session should validate")
	}
	if d := time.Since(last); d > 5*time.Second || d < -5*time.Second {
		t.Errorf("fresh session last_active off by %v", d)
	}
}

// B5 (TZ): Validate must not depend on the server's local timezone. The old
// code parsed ExpiresAt/LastActiveAt with time.ParseInLocation(timeFmt, …,
// time.Local), so flipping the process TZ shifted the recovered instant by
// the offset — a fresh session could be misjudged expired/valid. This test
// flips time.Local and asserts the same token still validates with last ≈ now.
//
// Red before the UTC fix (the flip shifts last by ~15h); green after.
func TestValidate_TZFlip_NoCoupling(t *testing.T) {
	orig := time.Local
	defer func() { time.Local = orig }()
	locA := time.FixedZone("TZ-A", 9*3600)  // +09:00
	locB := time.FixedZone("TZ-B", -6*3600) // -06:00

	time.Local = locA
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()
	tok, err := svc.Create(ctx, "erin", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Baseline: same TZ — must be valid, last ≈ now.
	if _, last, ok := svc.Validate(ctx, tok); !ok {
		t.Fatal("fresh session should validate in same TZ")
	} else if d := time.Since(last); d > 5*time.Second || d < -5*time.Second {
		t.Errorf("same-TZ last off by %v", d)
	}

	// Flip the process TZ and re-validate the SAME token.
	time.Local = locB
	_, last2, ok2 := svc.Validate(ctx, tok)
	if !ok2 {
		t.Error("TZ flip invalidated a fresh session — ParseInLocation(time.Local) coupling")
	}
	if d := time.Since(last2); d > 5*time.Second || d < -5*time.Second {
		t.Errorf("TZ flip shifted last by %v — stored times must be zone-independent (UTC)", d)
	}
}

// B5: Touch updates last_active_at to ~now.
func TestTouch_UpdatesLastActive(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()
	// session idle for 10 minutes (within 2h window)
	insertSessionRow(t, store, "tok-touch", "frank", futureStr(30*24*time.Hour), pastStr(10*time.Minute))
	if err := svc.Touch(ctx, "tok-touch"); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	_, last, ok := svc.Validate(ctx, "tok-touch")
	if !ok {
		t.Fatal("should still validate after touch")
	}
	if d := time.Since(last); d > 5*time.Second {
		t.Errorf("after Touch, last_active off by %v (should be ~now)", d)
	}
}

// B5: Touch on an unknown token is a no-error no-op (UPDATE matches 0 rows).
func TestTouch_UnknownToken_NoError(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	if err := svc.Touch(context.Background(), "no-such-token"); err != nil {
		t.Errorf("Touch on unknown token returned error: %v", err)
	}
}

// B5: Delete removes the session row.
func TestDelete_RemovesSession(t *testing.T) {
	store := newAuthStore(t)
	svc := NewSessionService(store)
	ctx := context.Background()
	tok, err := svc.Create(ctx, "gina", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(ctx, tok); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, _, ok := svc.Validate(ctx, tok); ok {
		t.Error("deleted token should not validate")
	}
}
