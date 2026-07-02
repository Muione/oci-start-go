package bootstrap

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/sysconf"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// newSysconfStore builds a temp-file SQLite with system_config so the
// turnstile BypassTokenHolder can read/write turnstile.enabled via sysconf.
// Returns a *db.Store sharing the single *sql.DB across both pools.
func newSysconfStore(t *testing.T) *db.Store {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "turnstile.db")
	w, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
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
		t.Fatalf("create system_config: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return &db.Store{Write: w, Read: w}
}

// S6: the bypass token must NOT appear in full in the log output (log
// leakage → anyone with log access can disable turnstile). Only a short
// prefix/hash is allowed for correlation.
func TestCheckAndLog_DoesNotLogFullToken(t *testing.T) {
	store := newSysconfStore(t)
	sc := sysconf.New(store)
	ctx := context.Background()
	if err := sc.SetEnabled(ctx, "turnstile.enabled", true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}

	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.WarnLevel)
	h := &BypassTokenHolder{}
	if err := h.CheckAndLog(ctx, sc, false, logger); err != nil {
		t.Fatalf("CheckAndLog: %v", err)
	}

	// The holder must have a token to test against.
	h.mu.Lock()
	tok := h.token
	h.mu.Unlock()
	if tok == "" {
		t.Fatal("no token generated; turnstile.enabled should be true")
	}

	out := buf.String()
	if strings.Contains(out, tok) {
		t.Errorf("full bypass token %q appears in log output — log leakage:\n%s", tok, out)
	}
	// Sanity: something about the bypass was logged.
	if !strings.Contains(out, "turnstile") && !strings.Contains(strings.ToLower(out), "bypass") {
		t.Errorf("expected a turnstile/bypass log line, got: %s", out)
	}
}

// S6: ConsumeAndRotate must compare the presented token against the holder
// token in constant time and be single-use (rotate on success).
func TestConsumeAndRotate_CorrectToken_ConsumesAndRotates(t *testing.T) {
	h := &BypassTokenHolder{}
	h.token = "abcdef0123456789"

	if !h.ConsumeAndRotate("abcdef0123456789") {
		t.Error("correct token should match")
	}
	// Single-use: a second consume with the same token must fail.
	if h.ConsumeAndRotate("abcdef0123456789") {
		t.Error("token must be single-use; second consume should fail")
	}
	h.mu.Lock()
	empty := h.token
	h.mu.Unlock()
	if empty != "" {
		t.Errorf("token not rotated after consume; still %q", empty)
	}
}

// S6: wrong token (including a prefix of the real token) must NOT match.
func TestConsumeAndRotate_WrongToken(t *testing.T) {
	h := &BypassTokenHolder{}
	h.token = "abcdef0123456789"

	for _, bad := range []string{"", "wrong", "abcdef", "abcdef012345678", "Xabcdef0123456789"} {
		if h.ConsumeAndRotate(bad) {
			t.Errorf("ConsumeAndRotate(%q) matched; must not", bad)
		}
	}
	// A wrong attempt must NOT consume/rotate the real token.
	h.mu.Lock()
	tok := h.token
	h.mu.Unlock()
	if tok != "abcdef0123456789" {
		t.Errorf("wrong-token attempt consumed the real token; now %q", tok)
	}
}

// S6: empty holder (turnstile disabled) never matches anything.
func TestConsumeAndRotate_EmptyHolder(t *testing.T) {
	h := &BypassTokenHolder{}
	if h.ConsumeAndRotate("anything") {
		t.Error("empty holder must not match")
	}
}
