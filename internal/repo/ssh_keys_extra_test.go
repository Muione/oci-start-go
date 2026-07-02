package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func setupSSHKeysDB(t *testing.T) *Queries {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE ssh_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT NOT NULL,
    encrypted_key TEXT NOT NULL,
    encrypted_passphrase TEXT,
    fingerprint TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	return New(db)
}

func TestCreateSSHKey_ReturnsID(t *testing.T) {
	q := setupSSHKeysDB(t)
	ctx := context.Background()
	id, err := q.CreateSSHKey(ctx, "k1", "ENC(key)", "ENC(pass)", "fp")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id <= 0 {
		t.Errorf("id=%d, want > 0", id)
	}
	got, err := q.GetSSHKey(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Label != "k1" || got.EncryptedKey.String != "ENC(key)" || got.EncryptedPassphrase.String != "ENC(pass)" || got.Fingerprint.String != "fp" {
		t.Errorf("got %+v", got)
	}
}

func TestListSSHKeys(t *testing.T) {
	q := setupSSHKeysDB(t)
	ctx := context.Background()
	_, _ = q.CreateSSHKey(ctx, "a", "EK1", "", "fp1")
	_, _ = q.CreateSSHKey(ctx, "b", "EK2", "EP2", "fp2")
	rows, err := q.ListSSHKeys(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2", len(rows))
	}
	if rows[0].Label != "a" || rows[1].Label != "b" {
		t.Errorf("order: %+v %+v", rows[0], rows[1])
	}
}

func TestGetSSHKey_NotFound(t *testing.T) {
	q := setupSSHKeysDB(t)
	_, err := q.GetSSHKey(context.Background(), 999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("got %v, want ErrNoRows", err)
	}
}

func TestDeleteSSHKey(t *testing.T) {
	q := setupSSHKeysDB(t)
	ctx := context.Background()
	id, _ := q.CreateSSHKey(ctx, "k", "EK", "", "fp")
	if err := q.DeleteSSHKey(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := q.GetSSHKey(ctx, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after delete: %v, want ErrNoRows", err)
	}
	// Delete missing is a no-op.
	if err := q.DeleteSSHKey(ctx, 999); err != nil {
		t.Errorf("delete missing: %v", err)
	}
}
