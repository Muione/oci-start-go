// Package service — ssh_key_test.go: tests for the SSHKeyService that stores
// SSH private keys AES-256-GCM encrypted at rest (master key) + resolves them
// by id for the WS SSH handler. The key content never leaves the backend; the
// frontend only sees id/label/fingerprint.
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Muione/oci-start-go/internal/util/sshkeygen"
)

func setupSSHKeySvc(t *testing.T) *SSHKeyService {
	t.Helper()
	store := newTestStore(t)
	mustExec(t, store, `CREATE TABLE ssh_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT NOT NULL,
    encrypted_key TEXT NOT NULL,
    encrypted_passphrase TEXT,
    fingerprint TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return NewSSHKeyService(store, key)
}

// TestSSHKeyService_CreateResolve_RoundTrip: create a key, assert the stored
// value is ciphertext (not the raw PEM), resolve it back, get the original PEM.
func TestSSHKeyService_CreateResolve_RoundTrip(t *testing.T) {
	svc := setupSSHKeySvc(t)
	ctx := context.Background()
	kp, err := sshkeygen.GenerateRSA2048()
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}

	id, err := svc.Create(ctx, "my-key", kp.PrivateKeyPEM, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Stored value must be ciphertext, not the raw PEM.
	var enc string
	row := svc.store.Read.QueryRow("SELECT encrypted_key FROM ssh_keys WHERE id=?", id)
	if err := row.Scan(&enc); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if enc == kp.PrivateKeyPEM {
		t.Error("stored key must be ciphertext, not raw PEM")
	}

	content, _, err := svc.Resolve(ctx, id)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if content != kp.PrivateKeyPEM {
		t.Errorf("resolve got a different PEM than stored")
	}
}

// TestSSHKeyService_List_NoKeyContent: the list view must NOT include the key
// content (only id/label/fingerprint) — the frontend never sees the material.
func TestSSHKeyService_List_NoKeyContent(t *testing.T) {
	svc := setupSSHKeySvc(t)
	ctx := context.Background()
	kp, _ := sshkeygen.GenerateRSA2048()
	id, _ := svc.Create(ctx, "k1", kp.PrivateKeyPEM, "")

	views, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 1 || views[0].ID != id || views[0].Label != "k1" {
		t.Errorf("view %+v", views)
	}
	if views[0].Fingerprint == "" || !strings.HasPrefix(views[0].Fingerprint, "SHA256:") {
		t.Errorf("fingerprint=%q want SHA256:...", views[0].Fingerprint)
	}
	// SSHKeyView is defined without a Content/EncryptedKey field, so the
	// material cannot leak via List — enforced by the type itself.
}

func TestSSHKeyService_Create_InvalidKey(t *testing.T) {
	svc := setupSSHKeySvc(t)
	_, err := svc.Create(context.Background(), "bad", "not a key", "")
	if err == nil {
		t.Fatal("invalid key: want error")
	}
}

func TestSSHKeyService_Resolve_NotFound(t *testing.T) {
	svc := setupSSHKeySvc(t)
	_, _, err := svc.Resolve(context.Background(), 999)
	if err == nil {
		t.Error("missing key: want error")
	}
}

func TestSSHKeyService_Delete(t *testing.T) {
	svc := setupSSHKeySvc(t)
	ctx := context.Background()
	kp, _ := sshkeygen.GenerateRSA2048()
	id, _ := svc.Create(ctx, "k", kp.PrivateKeyPEM, "")
	if err := svc.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, err := svc.Resolve(ctx, id); err == nil {
		t.Error("after delete: want error on resolve")
	}
}

// TestFingerprint_ValidKey: fingerprint of a valid key is SHA256:...; bad key
// errors. Pure function — testable without DB.
func TestFingerprint_ValidKey(t *testing.T) {
	kp, _ := sshkeygen.GenerateRSA2048()
	fp, err := fingerprint(kp.PrivateKeyPEM, "")
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Errorf("fp=%q want SHA256:...", fp)
	}
	if _, err := fingerprint("bad", ""); err == nil {
		t.Error("bad key: want error")
	}
}
