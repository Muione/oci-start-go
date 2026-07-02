// Package service -- service_test_helper_test.go: shared in-memory modernc
// sqlite helpers for service-layer tests. One connection (MaxOpenConns=1) so a
// private :memory: schema persists; both store pools point at the same *sql.DB.
//
// countConnector wraps modernc's driver to count queries (Exec/Query) and rows
// fetched (Next) per *sql.DB, used to prove N+1 fixes do not fan out with N.
package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync/atomic"
	"testing"

	"github.com/Muione/oci-start-go/internal/db"
	_ "modernc.org/sqlite" // register "sqlite" driver
)

// testSchema creates every table the service-layer tests touch, matching the
// migration definitions (minimal columns are not enough: the sqlc SELECTs list
// every column, so a missing column would scan-fail).
const testSchema = `
CREATE TABLE tenant (
    id INTEGER PRIMARY KEY,
    tenant_id TEXT, user_name TEXT, fingerprint TEXT, tenancy TEXT, region TEXT,
    key_file TEXT, created_at TEXT, api_synced INTEGER, enable_icmp INTEGER,
    enable_all_protocol INTEGER, is_home_region INTEGER, paren_id BIGINT,
    tenancy_name TEXT, tenancy_des TEXT, account_type TEXT, cloud_type INTEGER DEFAULT 1,
    region_en TEXT, id_str TEXT, email_address TEXT, email_enable INTEGER,
    transfer_status INTEGER DEFAULT 0, transfer_amount TEXT, is_active INTEGER DEFAULT 1,
    key_file_blob TEXT
);
CREATE TABLE register_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_prv_id BIGINT, tenant_id TEXT UNIQUE,
    account_type INTEGER, plan_type INTEGER, register_time TEXT, city TEXT, country TEXT,
    email_address TEXT, first_name TEXT, last_name TEXT, line1 TEXT, postal_code TEXT,
    subscription_plan_number TEXT, upgrade_state TEXT, created_time TEXT, updated_time TEXT,
    cloud_type INTEGER DEFAULT 1
);
CREATE TABLE boot_instance (
    id INTEGER PRIMARY KEY AUTOINCREMENT, version BIGINT NOT NULL DEFAULT 0, boot_id TEXT,
    tenant_id BIGINT, ocpu INTEGER, memory INTEGER, disk INTEGER, loop_time INTEGER,
    instance_count INTEGER, status INTEGER, architecture TEXT, root_password TEXT,
    public_ip TEXT, next_execution_time TEXT, add_count BIGINT DEFAULT 0,
    success_count INTEGER DEFAULT 0, remark TEXT, created_at TEXT, updated_at TEXT,
    cloud_type INTEGER DEFAULT 1, current_attempt_count INTEGER DEFAULT 0,
    yesterday_attempt_count INTEGER DEFAULT 0, reset_today_flag INTEGER DEFAULT 0,
    last_reset_date TEXT, fail_count INTEGER DEFAULT 0, total_count BIGINT, image_id TEXT,
    operating_system TEXT, operating_system_version TEXT, data_gap TEXT, notify_flag TEXT
);
CREATE TABLE instance_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id BIGINT, instance_id TEXT, display_name TEXT,
    shape TEXT, state TEXT, ocpus INTEGER, memory_in_gbs INTEGER, boot_volume_size_in_gbs BIGINT,
    public_ips TEXT, private_ips TEXT, availability_domain TEXT, compartment_id TEXT,
    boot_volume_id TEXT, remark TEXT, boot_volume_name TEXT, vpus_per_gb TEXT,
    ipv6_addresses TEXT, vnic_ids TEXT, username TEXT, port INTEGER, password TEXT,
    processor_description TEXT, architecture TEXT, cloud_type INTEGER DEFAULT 1,
    sys_image_backup INTEGER DEFAULT 0, conn_time BIGINT NOT NULL DEFAULT 0,
    enable_ping INTEGER NOT NULL DEFAULT 0, on_line_enable INTEGER NOT NULL DEFAULT 1,
    last_on_line_enable INTEGER NOT NULL DEFAULT 1, offline_notify INTEGER NOT NULL DEFAULT 0,
    resume_notify INTEGER NOT NULL DEFAULT 0, monitor_installed INTEGER, last_heartbeat TEXT,
    create_time TEXT
);
CREATE TABLE tenant_email_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id BIGINT, domain_id TEXT, domain_name TEXT,
    sender_id TEXT, credential_id TEXT, smtp_username TEXT, smtp_password TEXT,
    smtp_host TEXT, smtp_port TEXT, sender_email TEXT, dkim_id TEXT, cname_record_value TEXT,
    active INTEGER, created_time TEXT, daily_email_limit BIGINT, today_sent_count BIGINT,
    last_reset_date TEXT, dbs_record_ids_str TEXT
);
CREATE TABLE email_body (
    id INTEGER PRIMARY KEY AUTOINCREMENT, email_body_id TEXT NOT NULL UNIQUE,
    current_version BIGINT, tenant_name TEXT, tenant_email_config_id BIGINT, sender_email TEXT,
    title TEXT, content TEXT, receive_total BIGINT, receive_success_total BIGINT,
    receive_fail_total BIGINT, create_time TEXT
);
CREATE TABLE email_send_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT, email_send_record_id TEXT, email_body_id TEXT,
    email_send_address TEXT, current_version BIGINT, tenant_name TEXT, email_receive_id BIGINT,
    receive_email_address TEXT, send_state INTEGER, create_time TEXT
);
CREATE TABLE email_receive (
    id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL, name TEXT NOT NULL,
    create_time TEXT, update_time TEXT
);
`

// newTestStore opens a private in-memory SQLite with the test schema. Both
// store pools point at the same *sql.DB so reads see writes.
func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Exec(testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &db.Store{Write: d, Read: d}
}

// --- counting driver: proves N+1 fixes without fan-out ---

type countConn struct {
	inner driver.Conn
	exec  driver.ExecerContext
	query driver.QueryerContext
	begin driver.ConnBeginTx
	reset driver.SessionResetter
	valid driver.Validator
	ctrQ  *atomic.Int64
	ctrR  *atomic.Int64
}

func (c *countConn) Close() error { return c.inner.Close() }

// Prepare/Begin satisfy the legacy driver.Conn interface; *sql.DB prefers the
// context-aware ExecerContext/QueryerContext/ConnBeginTx paths, so these are
// rarely called — delegate to the inner conn.
func (c *countConn) Prepare(query string) (driver.Stmt, error) { return c.inner.Prepare(query) }
func (c *countConn) Begin() (driver.Tx, error)                 { return c.inner.Begin() }

func (c *countConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.ctrQ.Add(1)
	return c.exec.ExecContext(ctx, q, args)
}

func (c *countConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.ctrQ.Add(1)
	rs, err := c.query.QueryContext(ctx, q, args)
	if err != nil {
		return nil, err
	}
	return &countRows{Rows: rs, ctr: c.ctrR}, nil
}

func (c *countConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.begin != nil {
		return c.begin.BeginTx(ctx, opts)
	}
	return nil, driver.ErrSkip
}

func (c *countConn) ResetSession(ctx context.Context) error {
	if c.reset != nil {
		return c.reset.ResetSession(ctx)
	}
	return nil
}

func (c *countConn) IsValid() bool {
	if c.valid != nil {
		return c.valid.IsValid()
	}
	return true
}

type countRows struct {
	driver.Rows
	ctr *atomic.Int64
}

func (r *countRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err == nil {
		r.ctr.Add(1) // a row was actually fetched
	}
	return err
}

type countConnector struct {
	inner driver.Driver
	dsn   string
	ctrQ  *atomic.Int64
	ctrR  *atomic.Int64
}

func (c *countConnector) Connect(ctx context.Context) (driver.Conn, error) {
	raw, err := c.inner.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	cc := &countConn{inner: raw, ctrQ: c.ctrQ, ctrR: c.ctrR}
	if e, ok := raw.(driver.ExecerContext); ok {
		cc.exec = e
	}
	if q, ok := raw.(driver.QueryerContext); ok {
		cc.query = q
	}
	if b, ok := raw.(driver.ConnBeginTx); ok {
		cc.begin = b
	}
	if r, ok := raw.(driver.SessionResetter); ok {
		cc.reset = r
	}
	if v, ok := raw.(driver.Validator); ok {
		cc.valid = v
	}
	return cc, nil
}

func (c *countConnector) Driver() driver.Driver { return c.inner }

// moderncDriver returns modernc's registered driver without connecting.
func moderncDriver() driver.Driver {
	probe, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	d := probe.Driver()
	_ = probe.Close()
	return d
}

// mustExec runs a statement on the store's write pool, fatal on error.
func mustExec(t *testing.T, store *db.Store, query string, args ...any) {
	t.Helper()
	if _, err := store.Write.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// newCountingStore is newTestStore plus per-DB query/row counters. Reset the
// counters (Store(0)) after seeding and before the action under test.
func newCountingStore(t *testing.T) (store *db.Store, queries, rows *atomic.Int64) {
	t.Helper()
	qc, rc := &atomic.Int64{}, &atomic.Int64{}
	conn := &countConnector{inner: moderncDriver(), dsn: ":memory:", ctrQ: qc, ctrR: rc}
	d := sql.OpenDB(conn)
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Exec(testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &db.Store{Write: d, Read: d}, qc, rc
}
