// Package grabber -- success_test.go: S4 regression. saveTemInstance must
// AES-256-GCM encrypt root_password before writing it to tem_instance, so the
// DB never holds the plaintext root password.
package grabber

import (
	"context"
	"database/sql"
	"testing"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/util/crypto"
	_ "modernc.org/sqlite" // register "sqlite" driver
)

// grabberTestSchema carries only the tables saveTemInstance touches (tenant for
// FindTenantByID, tem_instance for InsertTemInstance). boot_instance is included
// because FindBootInstanceByID-style scans list every column, in case future
// tests exercise the full success chain.
const grabberTestSchema = `
CREATE TABLE tenant (
    id INTEGER PRIMARY KEY, tenant_id TEXT, user_name TEXT, fingerprint TEXT, tenancy TEXT, region TEXT,
    key_file TEXT, created_at TEXT, api_synced INTEGER, enable_icmp INTEGER, enable_all_protocol INTEGER,
    is_home_region INTEGER, paren_id BIGINT, tenancy_name TEXT, tenancy_des TEXT, account_type TEXT,
    cloud_type INTEGER DEFAULT 1, region_en TEXT, id_str TEXT, email_address TEXT, email_enable INTEGER,
    transfer_status INTEGER DEFAULT 0, transfer_amount TEXT, is_active INTEGER DEFAULT 1, key_file_blob TEXT
);
CREATE TABLE tem_instance (
    id INTEGER PRIMARY KEY AUTOINCREMENT, tenancy TEXT, instance_id TEXT, public_ip TEXT, region TEXT,
    architecture TEXT, root_password TEXT, clone_boot_volume_id TEXT, cloud_type INTEGER DEFAULT 1
);
`

func newGrabberStore(t *testing.T) *db.Store {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Exec(grabberTestSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &db.Store{Write: d, Read: d}
}

func mustExecGrabber(t *testing.T, store *db.Store, query string, args ...any) {
	t.Helper()
	if _, err := store.Write.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func grabberMasterKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

// S4: saveTemInstance stores root_password as ciphertext, not plaintext. The
// stored value must decrypt back to the original under the engine's master key.
func TestSaveTemInstance_EncryptsRootPassword(t *testing.T) {
	store := newGrabberStore(t)
	ctx := context.Background()
	masterKey := grabberMasterKey()

	mustExecGrabber(t, store, `INSERT INTO tenant (id, tenant_id, user_name, tenancy, region, created_at, is_active)
		VALUES (1, 'ocid1.tenancy.x', 'user1', 'ocid1.tenancy.x', 'us-phoenix-1', '2026-01-01', 1)`)

	// saveTemInstance only touches e.deps, so a minimal Engine suffices.
	e := &Engine{deps: EngineDeps{Store: store, MasterKey: masterKey, Logger: zerolog.Nop()}}

	plain := "RootP@ss-1234"
	task := repo.BootInstance{
		ID:           1,
		BootID:       sql.NullString{String: "boot-1", Valid: true},
		TenantID:     sql.NullInt64{Int64: 1, Valid: true},
		Architecture: sql.NullString{String: "ARM", Valid: true},
		RootPassword: sql.NullString{String: plain, Valid: true},
		CloudType:    sql.NullInt64{Int64: 1, Valid: true},
	}
	result := &GrabResult{InstanceID: "ocid1.instance.x", PublicIP: "1.2.3.4"}

	if err := e.saveTemInstance(ctx, task, result); err != nil {
		t.Fatalf("saveTemInstance: %v", err)
	}

	var stored string
	err := store.Read.QueryRowContext(ctx,
		`SELECT root_password FROM tem_instance WHERE instance_id = ?`, "ocid1.instance.x").Scan(&stored)
	if err != nil {
		t.Fatalf("query tem_instance: %v", err)
	}
	if stored == "" {
		t.Fatal("root_password not stored")
	}
	if stored == plain {
		t.Fatalf("root_password stored as plaintext %q, want AES-256-GCM ciphertext", stored)
	}
	dec, err := crypto.DecryptString(stored, masterKey)
	if err != nil {
		t.Fatalf("stored value did not decrypt (stored=%q): %v", stored, err)
	}
	if dec != plain {
		t.Errorf("decrypted root_password = %q, want %q", dec, plain)
	}
}

// S4: without a master key, saveTemInstance stores the plaintext verbatim
// (unwired-bootstrap path — no silent corruption).
func TestSaveTemInstance_NilMasterKeyStoresPlaintext(t *testing.T) {
	store := newGrabberStore(t)
	ctx := context.Background()

	mustExecGrabber(t, store, `INSERT INTO tenant (id, tenant_id, user_name, tenancy, region, created_at, is_active)
		VALUES (1, 'ocid1.tenancy.x', 'user1', 'ocid1.tenancy.x', 'us-phoenix-1', '2026-01-01', 1)`)

	e := &Engine{deps: EngineDeps{Store: store, Logger: zerolog.Nop()}} // MasterKey nil

	plain := "plain-when-unwired"
	task := repo.BootInstance{
		ID:           1,
		TenantID:     sql.NullInt64{Int64: 1, Valid: true},
		Architecture: sql.NullString{String: "ARM", Valid: true},
		RootPassword: sql.NullString{String: plain, Valid: true},
		CloudType:    sql.NullInt64{Int64: 1, Valid: true},
	}
	result := &GrabResult{InstanceID: "ocid1.instance.y", PublicIP: "1.2.3.5"}

	if err := e.saveTemInstance(ctx, task, result); err != nil {
		t.Fatalf("saveTemInstance: %v", err)
	}

	var stored string
	if err := store.Read.QueryRowContext(ctx,
		`SELECT root_password FROM tem_instance WHERE instance_id = ?`, "ocid1.instance.y").Scan(&stored); err != nil {
		t.Fatalf("query tem_instance: %v", err)
	}
	if stored != plain {
		t.Errorf("nil master key: stored = %q, want plaintext %q (no encryption when unwired)", stored, plain)
	}
}
