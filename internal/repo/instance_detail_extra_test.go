package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

// setupInstanceDetailDB opens a private in-memory SQLite (modernc, driver
// "sqlite") and creates an instance_detail table with exactly the columns the
// three extra queries touch. Single connection (MaxOpenConns=1) so the
// per-connection :memory: schema persists across statements.
func setupInstanceDetailDB(t *testing.T) *Queries {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
DROP TABLE IF EXISTS instance_detail;
CREATE TABLE instance_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT,
    display_name TEXT,
    public_ips TEXT,
    private_ips TEXT,
    shape TEXT,
    state TEXT,
    boot_volume_id TEXT,
    availability_domain TEXT,
    compartment_id TEXT,
    tenant_id BIGINT,
    username TEXT,
    port INTEGER,
    password TEXT
);
`)
	if err != nil {
		t.Fatalf("create instance_detail: %v", err)
	}
	return New(db)
}

// TestFindConsoleInstanceInfo_RoundTrip is the Q3 console contract: the repo
// method returns the same columns main.go currently scans inline
// (instance_id, display_name, public_ips, private_ips, shape, username, port,
// password, tenant_id, compartment_id, availability_domain).
func TestFindConsoleInstanceInfo_RoundTrip(t *testing.T) {
	q := setupInstanceDetailDB(t)
	ctx := context.Background()

	const instID = "ocid1.instance.oc1.phx.aaa"
	if _, err := q.db.ExecContext(ctx, `
INSERT INTO instance_detail (
    instance_id, display_name, public_ips, private_ips, shape,
    username, port, password, tenant_id, compartment_id, availability_domain
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, instID, "my-vm", "203.0.113.1", "10.0.0.1", "VM.Standard2.1",
		"opc", 22, "s3cr3t", 1, "ocid1.compartment.oc1..ccc", "PHX-AD-1"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := q.FindConsoleInstanceInfo(ctx, sql.NullString{String: instID, Valid: true})
	if err != nil {
		t.Fatalf("FindConsoleInstanceInfo: %v", err)
	}
	wantNull := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	checkStr(t, "instance_id", got.InstanceID, wantNull(instID))
	checkStr(t, "display_name", got.DisplayName, wantNull("my-vm"))
	checkStr(t, "public_ips", got.PublicIps, wantNull("203.0.113.1"))
	checkStr(t, "private_ips", got.PrivateIps, wantNull("10.0.0.1"))
	checkStr(t, "shape", got.Shape, wantNull("VM.Standard2.1"))
	checkStr(t, "username", got.Username, wantNull("opc"))
	checkStr(t, "password", got.Password, wantNull("s3cr3t"))
	checkStr(t, "compartment_id", got.CompartmentID, wantNull("ocid1.compartment.oc1..ccc"))
	checkStr(t, "availability_domain", got.AvailabilityDomain, wantNull("PHX-AD-1"))
	if !got.Port.Valid || got.Port.Int64 != 22 {
		t.Errorf("port = %v, want 22", got.Port)
	}
	if !got.TenantID.Valid || got.TenantID.Int64 != 1 {
		t.Errorf("tenant_id = %v, want 1", got.TenantID)
	}

	// Missing instance -> sql.ErrNoRows (matches the current inline QueryRow behaviour).
	if _, err := q.FindConsoleInstanceInfo(ctx, sql.NullString{String: "does-not-exist", Valid: true}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing instance err = %v, want sql.ErrNoRows", err)
	}
}

// TestFindRescueInstanceInfo_RoundTrip is the Q3 rescue contract: returns
// instance_id, display_name, state, boot_volume_id, shape, availability_domain,
// compartment_id, public_ips, username, password.
func TestFindRescueInstanceInfo_RoundTrip(t *testing.T) {
	q := setupInstanceDetailDB(t)
	ctx := context.Background()

	const instID = "ocid1.instance.oc1.phx.bbb"
	if _, err := q.db.ExecContext(ctx, `
INSERT INTO instance_detail (
    instance_id, display_name, state, boot_volume_id, shape,
    availability_domain, compartment_id, public_ips, username, password
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, instID, "rescue-vm", "STOPPED", "ocid1.volume.oc1..vvv", "VM.Standard.E3.Flex",
		"PHX-AD-2", "ocid1.compartment.oc1..ccc", "203.0.113.2", "root", "r00t"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := q.FindRescueInstanceInfo(ctx, sql.NullString{String: instID, Valid: true})
	if err != nil {
		t.Fatalf("FindRescueInstanceInfo: %v", err)
	}
	wantNull := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	checkStr(t, "instance_id", got.InstanceID, wantNull(instID))
	checkStr(t, "display_name", got.DisplayName, wantNull("rescue-vm"))
	checkStr(t, "state", got.State, wantNull("STOPPED"))
	checkStr(t, "boot_volume_id", got.BootVolumeID, wantNull("ocid1.volume.oc1..vvv"))
	checkStr(t, "shape", got.Shape, wantNull("VM.Standard.E3.Flex"))
	checkStr(t, "availability_domain", got.AvailabilityDomain, wantNull("PHX-AD-2"))
	checkStr(t, "compartment_id", got.CompartmentID, wantNull("ocid1.compartment.oc1..ccc"))
	checkStr(t, "public_ips", got.PublicIps, wantNull("203.0.113.2"))
	checkStr(t, "username", got.Username, wantNull("root"))
	checkStr(t, "password", got.Password, wantNull("r00t"))

	if _, err := q.FindRescueInstanceInfo(ctx, sql.NullString{String: "nope", Valid: true}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing instance err = %v, want sql.ErrNoRows", err)
	}
}

// TestFindCompartmentID_RoundTrip is the Q3 compartment contract: returns the
// compartment_id column (nullable) for an instance.
func TestFindCompartmentID_RoundTrip(t *testing.T) {
	q := setupInstanceDetailDB(t)
	ctx := context.Background()

	const instID = "ocid1.instance.oc1.phx.ccc"
	if _, err := q.db.ExecContext(ctx, `
INSERT INTO instance_detail (instance_id, compartment_id) VALUES (?, ?)
`, instID, "ocid1.compartment.oc1..zzz"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := q.FindCompartmentID(ctx, sql.NullString{String: instID, Valid: true})
	if err != nil {
		t.Fatalf("FindCompartmentID: %v", err)
	}
	if !got.Valid || got.String != "ocid1.compartment.oc1..zzz" {
		t.Errorf("compartment_id = %v, want ocid1.compartment.oc1..zzz", got)
	}

	// Null compartment when the column is empty.
	const instID2 = "ocid1.instance.oc1.phx.ddd"
	if _, err := q.db.ExecContext(ctx, `INSERT INTO instance_detail (instance_id) VALUES (?)`, instID2); err != nil {
		t.Fatalf("seed null: %v", err)
	}
	got2, err := q.FindCompartmentID(ctx, sql.NullString{String: instID2, Valid: true})
	if err != nil {
		t.Fatalf("FindCompartmentID null row: %v", err)
	}
	if got2.Valid {
		t.Errorf("compartment_id = %v, want invalid", got2)
	}

	// Missing instance -> sql.ErrNoRows.
	if _, err := q.FindCompartmentID(ctx, sql.NullString{String: "nope", Valid: true}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("missing instance err = %v, want sql.ErrNoRows", err)
	}
}

func checkStr(t *testing.T, name string, got, want sql.NullString) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
