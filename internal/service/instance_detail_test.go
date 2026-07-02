// Package service -- instance_detail_test.go: P-2 regression. GetByID must
// resolve the single tenant name via FindTenantByID (one row), not by building
// a full id->name map over every tenant (full-table scan).
package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Muione/oci-start-go/internal/util/crypto"
)

// TestInstanceDetailGetByID_NoFullTenantScan seeds 100 tenants + 1 instance and
// asserts GetByID fetches only the one tenant it needs (not all 100), and that
// a renamed tenant is visible on the next call (no stale cache on the hot path).
func TestInstanceDetailGetByID_NoFullTenantScan(t *testing.T) {
	store, _, rc := newCountingStore(t)
	ctx := context.Background()

	for i := 1; i <= 100; i++ {
		mustExec(t, store, `INSERT INTO tenant (id, tenant_id, user_name, region, created_at, is_active)
			VALUES (?, ?, ?, 'us-phoenix-1', '2026-01-01 00:00:00', 1)`,
			i, fmt.Sprintf("ocid.tenancy.%d", i), fmt.Sprintf("tenant-%d", i))
	}
	mustExec(t, store, `INSERT INTO instance_detail (id, tenant_id, instance_id, display_name, state, conn_time, enable_ping, on_line_enable)
		VALUES (1, 1, 'ocid.instance.1', 'inst-1', 'RUNNING', 0, 0, 1)`)

	svc := NewInstanceDetailSvc(store)

	rc.Store(0) // count only GetByID's row fetches
	got, err := svc.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.TenantName != "tenant-1" {
		t.Errorf("TenantName = %q, want tenant-1", got.TenantName)
	}
	// Old code built a full id->name map via ListTenants (100 rows). The fix
	// resolves only the needed tenant via FindTenantByID (1 row) + the instance
	// row (1) = ~2 rows, independent of tenant count.
	if rows := rc.Load(); rows > 5 {
		t.Errorf("GetByID fetched %d rows; want <= 5 (no full-table tenant scan over 100 tenants)", rows)
	}

	// Rename tenant 1; GetByID must see the new name immediately (fresh query,
	// not a stale cached map).
	mustExec(t, store, `UPDATE tenant SET user_name = ? WHERE id = 1`, "tenant-1-renamed")
	rc.Store(0)
	got, err = svc.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID after rename: %v", err)
	}
	if got.TenantName != "tenant-1-renamed" {
		t.Errorf("TenantName after rename = %q, want tenant-1-renamed (stale cache?)", got.TenantName)
	}
}

// TestInstanceDetailList_TenantNameCacheHit proves List's tenant-name map is
// cached: the second List call does not re-scan the tenant table.
func TestInstanceDetailList_TenantNameCacheHit(t *testing.T) {
	store, qc, _ := newCountingStore(t)
	ctx := context.Background()

	for i := 1; i <= 50; i++ {
		mustExec(t, store, `INSERT INTO tenant (id, tenant_id, user_name, region, created_at, is_active)
			VALUES (?, ?, ?, 'us-phoenix-1', '2026-01-01 00:00:00', 1)`,
			i, fmt.Sprintf("ocid.tenancy.%d", i), fmt.Sprintf("t%d", i))
	}
	mustExec(t, store, `INSERT INTO instance_detail (id, tenant_id, instance_id, display_name, state, conn_time, enable_ping, on_line_enable)
		VALUES (1, 1, 'ocid.instance.1', 'd1', 'RUNNING', 0, 0, 1)`)

	oldTTL := tenantCacheTTL
	tenantCacheTTL = time.Hour // keep the cache fresh across the two calls
	t.Cleanup(func() { tenantCacheTTL = oldTTL })

	svc := NewInstanceDetailSvc(store)
	if _, _, err := svc.List(ctx, 10, 0); err != nil {
		t.Fatalf("List #1: %v", err)
	}
	qc.Store(0) // count only the second call
	if _, _, err := svc.List(ctx, 10, 0); err != nil {
		t.Fatalf("List #2: %v", err)
	}
	// Cache hit: the second call skips ListTenants, issuing only
	// CountInstanceDetails + ListAllInstanceDetails (2 queries). Without the
	// cache it would re-scan tenants (3 queries).
	if got := qc.Load(); got > 2 {
		t.Errorf("List #2 issued %d queries, want <= 2 (tenant-name map should be cached)", got)
	}
}

// testMasterKey returns a deterministic 32-byte AES key for S4 tests.
func testMasterKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

// S4: instance root password is AES-256-GCM encrypted at rest. GetRootPassword
// decrypts on read; legacy plaintext rows degrade-return verbatim (fallback).
func TestInstanceDetailGetRootPassword_DecryptAndFallback(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	masterKey := testMasterKey()

	plain := "s3cret-Root-P@ss"
	enc, err := crypto.EncryptString(plain, masterKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Row 1: ciphertext (new-style encrypted). Row 2: plaintext (legacy).
	mustExec(t, store, `INSERT INTO instance_detail (id, password, conn_time, enable_ping, on_line_enable) VALUES (1, ?, 0, 0, 1)`, enc)
	mustExec(t, store, `INSERT INTO instance_detail (id, password, conn_time, enable_ping, on_line_enable) VALUES (2, ?, 0, 0, 1)`, plain)

	svc := NewInstanceDetailSvc(store)
	svc.SetMasterKey(masterKey)

	got1, err := svc.GetRootPassword(ctx, 1)
	if err != nil {
		t.Fatalf("GetRootPassword(1) ciphertext row: %v", err)
	}
	if got1 != plain {
		t.Errorf("ciphertext row: GetRootPassword = %q, want %q (decrypt)", got1, plain)
	}
	got2, err := svc.GetRootPassword(ctx, 2)
	if err != nil {
		t.Fatalf("GetRootPassword(2) plaintext row: %v", err)
	}
	if got2 != plain {
		t.Errorf("plaintext row: GetRootPassword = %q, want %q (fallback)", got2, plain)
	}
}

// S4: with masterKey unwired (nil), GetRootPassword returns the raw stored
// value verbatim — keeps callers correct during bootstrap / before wiring.
func TestInstanceDetailGetRootPassword_NilMasterKeyReturnsRaw(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	masterKey := testMasterKey()
	enc, _ := crypto.EncryptString("whatever", masterKey)
	plain := "legacy-plain"

	mustExec(t, store, `INSERT INTO instance_detail (id, password, conn_time, enable_ping, on_line_enable) VALUES (1, ?, 0, 0, 1)`, enc)
	mustExec(t, store, `INSERT INTO instance_detail (id, password, conn_time, enable_ping, on_line_enable) VALUES (2, ?, 0, 0, 1)`, plain)

	svc := NewInstanceDetailSvc(store) // no SetMasterKey → masterKey nil

	got1, _ := svc.GetRootPassword(ctx, 1)
	if got1 != enc {
		t.Errorf("nil key, ciphertext row: = %q, want raw %q", got1, enc)
	}
	got2, _ := svc.GetRootPassword(ctx, 2)
	if got2 != plain {
		t.Errorf("nil key, plaintext row: = %q, want %q", got2, plain)
	}
}
