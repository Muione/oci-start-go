package sysconf

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// newTestStore returns a *db.Store backed by a temp-file SQLite with the
// system_config table created. Both pools point at the same file. Tests own
// the cleanup via t.Cleanup.
func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "sysconf.db")
	w, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open write: %v", err)
	}
	r, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = w.Close()
		t.Fatalf("open read: %v", err)
	}
	_, err = w.Exec(`CREATE TABLE IF NOT EXISTS system_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		config_key TEXT UNIQUE,
		config_value TEXT,
		config_enabled INTEGER,
		last_modified TEXT
	)`)
	if err != nil {
		_ = w.Close()
		_ = r.Close()
		t.Fatalf("create system_config: %v", err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	return &db.Store{Write: w, Read: r}
}

// captureLogger returns a zerolog.Logger writing JSON lines to buf and a
// helper to decode them.
func captureLogger(buf *bytes.Buffer) zerolog.Logger {
	return zerolog.New(buf).Level(zerolog.WarnLevel)
}

func decodeLevels(buf *bytes.Buffer) []string {
	var levels []string
	dec := json.NewDecoder(buf)
	for dec.More() {
		var m map[string]interface{}
		if err := dec.Decode(&m); err != nil {
			break
		}
		if lv, ok := m["level"].(string); ok {
			levels = append(levels, lv)
		}
	}
	return levels
}

// E4: GetString/GetBool must log a warn when the underlying query fails (DB
// hiccup / disconnect), so a transient failure that makes turnstile/mfa read
// as "off" is not silently swallowed. Absent key (sql.ErrNoRows) is the
// documented "not configured" case and must stay silent.
func TestGetString_LogsWarnOnDBError(t *testing.T) {
	store := newTestStore(t)
	var buf bytes.Buffer
	svc := New(store)
	svc.SetLogger(captureLogger(&buf))

	// Break the read pool so FindConfigByKey returns a real query error
	// (not sql.ErrNoRows). Models a connection drop / DB hiccup.
	if err := store.Read.Close(); err != nil {
		t.Fatalf("close read pool: %v", err)
	}

	got := svc.GetString(context.Background(), "turnstile.enabled")
	if got != "" {
		t.Errorf("GetString on broken DB = %q, want \"\"", got)
	}
	levels := decodeLevels(&buf)
	if !contains(levels, "warn") {
		t.Errorf("expected a warn log for DB error, got levels=%v raw=%q", levels, buf.String())
	}
}

func TestGetBool_LogsWarnOnDBError(t *testing.T) {
	store := newTestStore(t)
	var buf bytes.Buffer
	svc := New(store)
	svc.SetLogger(captureLogger(&buf))

	if err := store.Read.Close(); err != nil {
		t.Fatalf("close read pool: %v", err)
	}

	if svc.GetBool(context.Background(), "mfa.enabled") {
		t.Error("GetBool on broken DB = true, want false")
	}
	levels := decodeLevels(&buf)
	if !contains(levels, "warn") {
		t.Errorf("expected a warn log for DB error, got levels=%v raw=%q", levels, buf.String())
	}
}

// E4 (cont.): an absent key is the normal "not configured" case — no warn,
// returns zero. Otherwise every missing config spams the log.
func TestGetString_AbsentKey_NoWarn(t *testing.T) {
	store := newTestStore(t)
	var buf bytes.Buffer
	svc := New(store)
	svc.SetLogger(captureLogger(&buf))

	got := svc.GetString(context.Background(), "never.set.key")
	if got != "" {
		t.Errorf("absent key = %q, want \"\"", got)
	}
	if buf.Len() != 0 {
		t.Errorf("absent key should not log, got %q", buf.String())
	}
}

// B3: SetString and SetEnabled must NOT clobber the other column on upsert.
// SetEnabled(k,true) then SetString(k,"v") → enabled must still be true.
func TestB3_SetStringPreservesEnabled(t *testing.T) {
	store := newTestStore(t)
	svc := New(store)
	ctx := context.Background()
	const k = "turnstile.enabled"

	if err := svc.SetEnabled(ctx, k, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if err := svc.SetString(ctx, k, "region=us-phoenix-1"); err != nil {
		t.Fatalf("SetString: %v", err)
	}

	if !svc.GetBool(ctx, k) {
		t.Error("enabled was cleared by SetString; expected preserved true")
	}
	if got := svc.GetString(ctx, k); !strings.Contains(got, "us-phoenix-1") {
		t.Errorf("value = %q, want to contain the set string", got)
	}
}

// B3 (cont.): SetString(k,"v") then SetEnabled(k,true) → value must still be "v".
func TestB3_SetEnabledPreservesValue(t *testing.T) {
	store := newTestStore(t)
	svc := New(store)
	ctx := context.Background()
	const k = "app.feature.url"

	if err := svc.SetString(ctx, k, "https://example.com/hook"); err != nil {
		t.Fatalf("SetString: %v", err)
	}
	if err := svc.SetEnabled(ctx, k, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}

	if got := svc.GetString(ctx, k); got != "https://example.com/hook" {
		t.Errorf("value was cleared by SetEnabled; got %q, want preserved", got)
	}
	if !svc.GetBool(ctx, k) {
		t.Error("enabled = false, want true")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
