package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

// setupConsoleConnectionDB opens a private in-memory SQLite and creates the
// console_connections table with the post-0009 shape (encrypted_private_key +
// public_key_ssh + a unique index on instance_id so upsert ON CONFLICT works).
func setupConsoleConnectionDB(t *testing.T) *Queries {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
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
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return New(db)
}

func TestUpsertConsoleConnection_InsertThenReplace(t *testing.T) {
	q := setupConsoleConnectionDB(t)
	ctx := context.Background()

	if err := q.UpsertConsoleConnection(ctx, "i-1", 7, "conn-a", "ENC(a)", "ssh-rsa AAA a"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := q.GetConsoleConnectionByInstance(ctx, "i-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ConnectionID != "conn-a" || got.EncryptedPrivateKey.String != "ENC(a)" || got.PublicKeySSH.String != "ssh-rsa AAA a" {
		t.Errorf("got %+v", got)
	}

	// Upsert replaces (same instance -> new conn/key).
	if err := q.UpsertConsoleConnection(ctx, "i-1", 7, "conn-b", "ENC(b)", "ssh-rsa AAA b"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err = q.GetConsoleConnectionByInstance(ctx, "i-1")
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got.ConnectionID != "conn-b" || got.EncryptedPrivateKey.String != "ENC(b)" {
		t.Errorf("upsert did not replace: %+v", got)
	}
}

func TestGetConsoleConnection_NotFound(t *testing.T) {
	q := setupConsoleConnectionDB(t)
	_, err := q.GetConsoleConnectionByInstance(context.Background(), "nope")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("got %v, want sql.ErrNoRows", err)
	}
}

func TestDeleteConsoleConnectionByInstance(t *testing.T) {
	q := setupConsoleConnectionDB(t)
	ctx := context.Background()
	_ = q.UpsertConsoleConnection(ctx, "i-1", 1, "conn-a", "ENC", "pub")
	if err := q.DeleteConsoleConnectionByInstance(ctx, "i-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := q.GetConsoleConnectionByInstance(ctx, "i-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after delete, got %v, want ErrNoRows", err)
	}
	// Delete missing is a no-op (no error).
	if err := q.DeleteConsoleConnectionByInstance(ctx, "ghost"); err != nil {
		t.Errorf("delete missing: %v", err)
	}
}
