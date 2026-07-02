package repo

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTenantAggDB opens a private in-memory SQLite (modernc, driver "sqlite")
// and creates the tenant / register_detail / boot_instance tables with exactly
// the columns the aggregate and the three legacy queries touch. A single
// connection (MaxOpenConns=1) is used so the per-connection :memory: schema
// persists across statements within the test; the repo methods are agnostic to
// pool config. Returns the repo handle.
func setupTenantAggDB(t *testing.T) *Queries {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
DROP TABLE IF EXISTS tenant;
DROP TABLE IF EXISTS register_detail;
DROP TABLE IF EXISTS boot_instance;
CREATE TABLE tenant (
    id INTEGER PRIMARY KEY,
    tenant_id TEXT,
    user_name TEXT,
    fingerprint TEXT,
    tenancy TEXT,
    region TEXT,
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
    is_active INTEGER DEFAULT 1
);
CREATE TABLE register_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_prv_id BIGINT,
    tenant_id TEXT,
    account_type INTEGER,
    plan_type INTEGER,
    register_time TEXT,
    city TEXT,
    country TEXT,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
    line1 TEXT,
    postal_code TEXT,
    subscription_plan_number TEXT,
    upgrade_state TEXT,
    created_time TEXT,
    updated_time TEXT,
    cloud_type INTEGER DEFAULT 1
);
CREATE TABLE boot_instance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT,
    status INTEGER
);
`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return New(db)
}

// TestListTenantsWithCounts_Parity is the P-1 contract: the single aggregate
// query must return, per tenant, the same register_time / boot count / child
// count that the three legacy queries (FindRegisterDetailByTenantId,
// CountBootInstancesByTenantId, CountTenantChildren) would return individually.
func TestListTenantsWithCounts_Parity(t *testing.T) {
	q := setupTenantAggDB(t)
	ctx := context.Background()

	// Tenant 1: parent, has register_detail, 2 active + 1 inactive boot, 1 child.
	if _, err := q.db.ExecContext(ctx, `
INSERT INTO tenant (id, tenant_id, user_name, tenancy, region, created_at, is_active)
VALUES (1, 'ocid1.tenancy.oc1..aaa', 'u1', 'tenancy-aaa', 'us-phoenix-1', '2026-01-01 00:00:00', 1),
       (2, 'ocid1.tenancy.oc1..bbb', 'u2', 'tenancy-bbb', 'us-ashburn-1', '2026-02-01 00:00:00', 1);
`); err != nil {
		t.Fatalf("seed tenants: %v", err)
	}
	// Make tenant 2 a child of tenant 1.
	if _, err := q.db.ExecContext(ctx, `UPDATE tenant SET paren_id = 1 WHERE id = 2`); err != nil {
		t.Fatalf("set paren: %v", err)
	}
	if _, err := q.db.ExecContext(ctx, `
INSERT INTO register_detail (tenant_id, register_time) VALUES
    ('ocid1.tenancy.oc1..aaa', '2026-01-15 10:00:00'),
    ('ocid1.tenancy.oc1..aaa', '2026-02-20 12:00:00');
`); err != nil {
		t.Fatalf("seed register_detail: %v", err)
	}
	if _, err := q.db.ExecContext(ctx, `
INSERT INTO boot_instance (tenant_id, status) VALUES
    (1, 1), (1, 1), (1, 0), (2, 1);
`); err != nil {
		t.Fatalf("seed boot_instance: %v", err)
	}

	rows, err := q.ListTenantsWithCounts(ctx)
	if err != nil {
		t.Fatalf("ListTenantsWithCounts: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// A duplicate register_detail row for tenant 1 is seeded above; this row
	// count staying 2 is the regression guard that the aggregate does not
	// multiply tenant rows via a LEFT JOIN on register_detail (which has no
	// unique constraint on tenant_id).
	// Ordered by id.
	if rows[0].ID != 1 || rows[1].ID != 2 {
		t.Fatalf("unexpected order: %+v", []int64{rows[0].ID, rows[1].ID})
	}

	// Per-row parity against the three legacy queries.
	for _, r := range rows {
		// register_time parity.
		rd, rdErr := q.FindRegisterDetailByTenantId(ctx, ns(r.TenantID))
		switch {
		case rdErr == nil:
			if !r.RegisterTime.Valid || r.RegisterTime.String != ns(rd.RegisterTime) {
				t.Errorf("tenant %d: aggregate register_time %v != legacy %v",
					r.ID, r.RegisterTime, rd.RegisterTime)
			}
		case rdErr == sql.ErrNoRows:
			if r.RegisterTime.Valid {
				t.Errorf("tenant %d: aggregate register_time %v but legacy found none", r.ID, r.RegisterTime)
			}
		default:
			t.Fatalf("tenant %d: FindRegisterDetailByTenantId: %v", r.ID, rdErr)
		}

		// boot count parity (status=1 only).
		bootN, err := q.CountBootInstancesByTenantId(ctx, nullInt(r.ID))
		if err != nil {
			t.Fatalf("tenant %d: CountBootInstancesByTenantId: %v", r.ID, err)
		}
		if r.BootCount != bootN {
			t.Errorf("tenant %d: aggregate boot_count %d != legacy %d", r.ID, r.BootCount, bootN)
		}

		// child count parity.
		childN, err := q.CountTenantChildren(ctx, nullInt(r.ID))
		if err != nil {
			t.Fatalf("tenant %d: CountTenantChildren: %v", r.ID, err)
		}
		if r.ChildCount != childN {
			t.Errorf("tenant %d: aggregate child_count %d != legacy %d", r.ID, r.ChildCount, childN)
		}
	}

	// Spot-check concrete values so the parity loop can't be vacuous.
	// register_time is one of the two seeded duplicates (LIMIT 1 picks the
	// first; the parity loop already proves it matches the legacy query).
	if !rows[0].RegisterTime.Valid ||
		(rows[0].RegisterTime.String != "2026-01-15 10:00:00" && rows[0].RegisterTime.String != "2026-02-20 12:00:00") {
		t.Errorf("tenant 1 register_time = %v, want one of the two seeded values", rows[0].RegisterTime)
	}
	if rows[0].BootCount != 2 {
		t.Errorf("tenant 1 boot_count = %d, want 2", rows[0].BootCount)
	}
	if rows[0].ChildCount != 1 {
		t.Errorf("tenant 1 child_count = %d, want 1", rows[0].ChildCount)
	}
	if rows[1].RegisterTime.Valid {
		t.Errorf("tenant 2 register_time = %v, want invalid", rows[1].RegisterTime)
	}
	if rows[1].BootCount != 1 {
		t.Errorf("tenant 2 boot_count = %d, want 1", rows[1].BootCount)
	}
	if rows[1].ChildCount != 0 {
		t.Errorf("tenant 2 child_count = %d, want 0", rows[1].ChildCount)
	}
}

// ns mirrors service.ns but is local so the test stays self-contained.
func ns(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func nullInt(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
