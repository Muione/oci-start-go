package migration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite" // register "sqlite" driver, pure-Go (no cgo)
)

// --- test helpers ---

// setupTestDB opens a single-connection in-memory SQLite (modernc, no cgo) and
// creates the minimal subset of the oci-start schema that the importer touches.
// A shared-cache in-memory DSN keeps the one connection consistent.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:testimport?mode=memory&cache=shared&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(tenantSchema + loginUserSchema + ociSshConnSchema + tenantEmailConfigSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

const tenantSchema = `
CREATE TABLE tenant (
	id INTEGER PRIMARY KEY,
	tenant_id TEXT, user_name TEXT, fingerprint TEXT, tenancy TEXT, region TEXT,
	key_file TEXT, created_at TEXT, api_synced INTEGER, enable_icmp INTEGER,
	enable_all_protocol INTEGER, is_home_region INTEGER, paren_id BIGINT,
	tenancy_name TEXT, tenancy_des TEXT, account_type TEXT, cloud_type INTEGER DEFAULT 1,
	region_en TEXT, id_str TEXT, email_address TEXT, email_enable INTEGER,
	transfer_status INTEGER DEFAULT 0, transfer_amount TEXT, is_active INTEGER DEFAULT 1
);`

const loginUserSchema = `
CREATE TABLE login_user (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	is_first_user INTEGER,
	login_type TEXT NOT NULL,
	external_id TEXT,
	last_login_at TEXT
);`

const ociSshConnSchema = `
CREATE TABLE oci_ssh_conn (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	instance_id TEXT, name TEXT NOT NULL, remark TEXT NOT NULL, username TEXT NOT NULL,
	host TEXT, port INTEGER, password TEXT, cloud_type INTEGER DEFAULT 1, folder_id BIGINT
);`

const tenantEmailConfigSchema = `
CREATE TABLE tenant_email_config (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	tenant_id BIGINT, domain_id TEXT, domain_name TEXT, sender_id TEXT,
	credential_id TEXT, smtp_username TEXT, smtp_password TEXT, smtp_host TEXT,
	smtp_port TEXT, sender_email TEXT, dkim_id TEXT, cname_record_value TEXT,
	active INTEGER, created_time TEXT, daily_email_limit BIGINT,
	today_sent_count BIGINT, last_reset_date TEXT, dbs_record_ids_str TEXT
);`

func newTestImporter(db *sql.DB) *Importer {
	log := zerolog.Nop()
	return NewImporter(db, log, NewSQLSplitter(log))
}

// newTestImporterCaptureLog wires the importer's zerolog to a buffer so tests
// can assert on the per-row skip/error messages that ImportSQLText logs.
func newTestImporterCaptureLog(t *testing.T, db *sql.DB) (*Importer, *strings.Builder) {
	t.Helper()
	var buf strings.Builder
	log := zerolog.New(&buf)
	return NewImporter(db, log, NewSQLSplitter(log)), &buf
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- B6: splitValues table-driven coverage (previously zero tests) ---

func TestSplitValues(t *testing.T) {
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	// A multi-line value containing both an escaped quote ('') and a comma,
	// exercising the single-quote toggle's handling of '' inside a string.
	multiEscaped := "line1\nO''Brien, Jr\nline3"
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "1, 'a', 'b'", []string{"1", "'a'", "'b'"}},
		{"null_number_text", "NULL, 42, 3.14, 'text'", []string{"NULL", "42", "3.14", "'text'"}},
		{"empty_string", "'', 'x'", []string{"''", "'x'"}},
		{"escaped_quote", "'O''Brien', 2", []string{"'O''Brien'", "2"}},
		{"escaped_quote_and_comma_in_string", "'a''b,c', 2", []string{"'a''b,c'", "2"}},
		{"value_ends_with_escaped_quote", "'abc''', 2", []string{"'abc'''", "2"}},
		{"multiline_pem", "1, '" + pem + "'", []string{"1", "'" + pem + "'"}},
		{"multiline_pem_with_comma_and_escape", "1, '" + multiEscaped + "'", []string{"1", "'" + multiEscaped + "'"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitValues(tc.in)
			if !sliceEq(got, tc.want) {
				t.Fatalf("splitValues(%q)\n got: %v\nwant: %v", tc.in, got, tc.want)
			}
		})
	}
}

// --- B6: parseInsertSQL table-driven coverage (previously zero tests) ---

func TestParseInsertSQL(t *testing.T) {
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	cases := []struct {
		name    string
		sql     string
		wantErr bool
		table   string
		cols    []string
		values  []string
	}{
		{
			name:   "simple",
			sql:    "INSERT INTO foo (a, b) VALUES (1, 'x');",
			table:  "foo",
			cols:   []string{"a", "b"},
			values: []string{"1", "'x'"},
		},
		{
			name:   "null_and_numbers",
			sql:    "INSERT INTO foo (id, name, score) VALUES (1, NULL, 3.14);",
			table:  "foo",
			cols:   []string{"id", "name", "score"},
			values: []string{"1", "NULL", "3.14"},
		},
		{
			name:   "multiline_pem_value",
			sql:    "INSERT INTO tenant (id, key_file) VALUES (1, '" + pem + "');",
			table:  "tenant",
			cols:   []string{"id", "key_file"},
			values: []string{"1", "'" + pem + "'"},
		},
		{
			name:    "count_mismatch",
			sql:     "INSERT INTO foo (a, b) VALUES (1, 'x', 2);",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := parseInsertSQL(tc.sql)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got row: %+v", row)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if row.Table != tc.table {
				t.Errorf("table: got %q, want %q", row.Table, tc.table)
			}
			if !sliceEq(row.Cols, tc.cols) {
				t.Errorf("cols: got %v, want %v", row.Cols, tc.cols)
			}
			if !sliceEq(row.Values, tc.values) {
				t.Errorf("values: got %v, want %v", row.Values, tc.values)
			}
		})
	}
}

// --- B6: ImportSQLText integration over in-memory SQLite ---

// TestImportSQLText_InsertsValidRow is the B6 red: validateColumns uses
// `PRAGMA table_info(?)` with a bound placeholder, which modernc/sqlite rejects
// with a syntax error, so EVERY valid row is silently skipped. After the fix
// (whitelist-validated literal PRAGMA) the row inserts.
func TestImportSQLText_InsertsValidRow(t *testing.T) {
	db := setupTestDB(t)
	imp := newTestImporter(db)
	ctx := context.Background()

	sqlText := "INSERT INTO oci_ssh_conn (id, name, remark, username) VALUES (1, 'n', 'r', 'u');"
	stats, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err != nil {
		t.Fatalf("ImportSQLText: %v", err)
	}
	if stats.Inserted != 1 {
		t.Fatalf("expected Inserted=1, got Inserted=%d Skipped=%d Errors=%d",
			stats.Inserted, stats.Skipped, stats.Errors)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM oci_ssh_conn").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row in oci_ssh_conn, got %d", count)
	}
}

// TestImportSQLText_OciSshConnDefaults verifies the OCI_SSH_CONN default-filling
// parity (HOST→127.0.0.1, NAME→Unknown, REMARK→'', FOLDER_ID→-100). Depends on
// the B6 validateColumns fix so the row actually reaches INSERT.
func TestImportSQLText_OciSshConnDefaults(t *testing.T) {
	db := setupTestDB(t)
	imp := newTestImporter(db)
	ctx := context.Background()

	sqlText := "INSERT INTO oci_ssh_conn (id, name, remark, username, host, folder_id) VALUES (1, NULL, NULL, 'u', NULL, NULL);"
	stats, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err != nil {
		t.Fatalf("ImportSQLText: %v", err)
	}
	if stats.Inserted != 1 {
		t.Fatalf("expected Inserted=1, got Inserted=%d Skipped=%d Errors=%d",
			stats.Inserted, stats.Skipped, stats.Errors)
	}
	var host, name, remark string
	var folder int64
	if err := db.QueryRow("SELECT host, name, remark, folder_id FROM oci_ssh_conn WHERE id=1").
		Scan(&host, &name, &remark, &folder); err != nil {
		t.Fatalf("select: %v", err)
	}
	if host != "127.0.0.1" {
		t.Errorf("host default: got %q, want 127.0.0.1", host)
	}
	if name != "Unknown" {
		t.Errorf("name default: got %q, want Unknown", name)
	}
	if remark != "" {
		t.Errorf("remark default: got %q, want empty", remark)
	}
	if folder != -100 {
		t.Errorf("folder_id default: got %d, want -100", folder)
	}
}

// TestImportSQLText_InsertOrIgnoreDuplicate verifies INSERT OR IGNORE skips a
// duplicate PK without erroring. Depends on the B6 fix so the row reaches INSERT.
func TestImportSQLText_InsertOrIgnoreDuplicate(t *testing.T) {
	db := setupTestDB(t)
	imp := newTestImporter(db)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO oci_ssh_conn (id, name, remark, username) VALUES (1, 'n', 'r', 'u')"); err != nil {
		t.Fatalf("pre-insert: %v", err)
	}
	sqlText := "INSERT INTO oci_ssh_conn (id, name, remark, username) VALUES (1, 'n2', 'r2', 'u2');"
	stats, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err != nil {
		t.Fatalf("ImportSQLText: %v", err)
	}
	if stats.SkippedDups != 1 {
		t.Fatalf("expected SkippedDups=1, got %d (Inserted=%d Skipped=%d Errors=%d)",
			stats.SkippedDups, stats.Inserted, stats.Skipped, stats.Errors)
	}
}

// TestImportSQLText_TenantDedup verifies the two-pass TENANT duplicate guard:
// a pre-existing tenant ID aborts the import with "data already imported".
// This path does not touch validateColumns, so it passes independent of B6.
func TestImportSQLText_TenantDedup(t *testing.T) {
	db := setupTestDB(t)
	imp := newTestImporter(db)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO tenant (id) VALUES (5)"); err != nil {
		t.Fatalf("pre-insert: %v", err)
	}
	sqlText := "INSERT INTO tenant (id, tenant_id) VALUES (5, 'ocid1');"
	_, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err == nil {
		t.Fatal("expected dedup error, got nil")
	}
	if !strings.Contains(err.Error(), "already imported") {
		t.Fatalf("expected 'already imported' error, got: %v", err)
	}
}

// TestImportSQLText_LoginUserSkip verifies the LOGIN_USER handler skips a row
// whose username already exists. The skip happens before validateColumns, so it
// passes independent of B6.
func TestImportSQLText_LoginUserSkip(t *testing.T) {
	db := setupTestDB(t)
	imp := newTestImporter(db)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO login_user (id, username, password, login_type) VALUES (1, 'alice', 'x', 'LOCAL')"); err != nil {
		t.Fatalf("pre-insert: %v", err)
	}
	sqlText := "INSERT INTO login_user (id, username, password, login_type) VALUES (2, 'alice', 'y', 'LOCAL');"
	stats, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err != nil {
		t.Fatalf("ImportSQLText: %v", err)
	}
	if stats.SkippedUser != 1 {
		t.Fatalf("expected SkippedUser=1, got %d", stats.SkippedUser)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM login_user WHERE username='alice'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 alice, got %d (duplicate inserted)", count)
	}
}

// TestImportSQLText_MultilinePEM is the B6 "PEM 边界" red: the line-buffering
// loop `continue`s after buffering any line containing "PRIVATE KEY", so the
// closing `-----END ... PRIVATE KEY-----');` line never triggers
// isCompleteInsert and a trailing PEM-bearing INSERT is silently dropped.
func TestImportSQLText_MultilinePEM(t *testing.T) {
	db := setupTestDB(t)
	imp := newTestImporter(db)
	ctx := context.Background()

	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	sqlText := "INSERT INTO tenant (id, key_file) VALUES (1, '" + pem + "');"
	stats, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err != nil {
		t.Fatalf("ImportSQLText: %v", err)
	}
	if stats.Inserted != 1 {
		t.Fatalf("expected Inserted=1 (multiline PEM insert), got Inserted=%d InsertLines=%d Skipped=%d Errors=%d",
			stats.Inserted, stats.InsertLines, stats.Skipped, stats.Errors)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM tenant WHERE id=1").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected tenant id=1 inserted, got count=%d", count)
	}
}

// --- S7: SQL injection via untrusted table/column identifiers ---

// TestImportSQLText_RejectsSQLInjectionTableName asserts that a malicious table
// name (e.g. `foo; DROP TABLE victim`) is rejected with an explicit "invalid"
// identifier error rather than being interpolated into PRAGMA/INSERT SQL. The
// log is captured because ImportSQLText logs per-row errors and continues.
func TestImportSQLText_RejectsSQLInjectionTableName(t *testing.T) {
	db := setupTestDB(t)
	if _, err := db.Exec("CREATE TABLE victim (id INTEGER)"); err != nil {
		t.Fatalf("create victim: %v", err)
	}
	imp, buf := newTestImporterCaptureLog(t, db)
	ctx := context.Background()

	// Malicious table name that would drop `victim` if interpolated raw.
	sqlText := "INSERT INTO foo; DROP TABLE victim (x) VALUES (1);"
	stats, err := imp.ImportSQLText(ctx, sqlText, "/tmp")
	if err != nil {
		t.Fatalf("ImportSQLText: %v", err)
	}
	if stats.Skipped == 0 && stats.Errors == 0 {
		t.Fatal("expected the malicious row to be skipped/errored")
	}
	// victim must survive — no DROP TABLE executed.
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='victim'").Scan(&n); err != nil {
		t.Fatalf("check victim: %v", err)
	}
	if n != 1 {
		t.Fatal("victim table was dropped — SQL injection succeeded")
	}
	// Rejection must come from explicit identifier validation, not an opaque
	// PRAGMA syntax error.
	if !strings.Contains(buf.String(), "invalid") {
		t.Fatalf("expected log to mention 'invalid' identifier, got log: %s", buf.String())
	}
}

// TestValidateIdentifiers directly enumerates the S7 injection vectors the spec
// calls out (`; DROP TABLE`, quotes, spaces) plus a valid baseline.
func TestValidateIdentifiers(t *testing.T) {
	validRow := InsertRow{Table: "tenant", Cols: []string{"id", "key_file"}}
	malicious := []string{
		"foo; DROP TABLE victim",
		"foo'",
		"foo bar",
		`foo"bar`,
		"foo;--",
		"foo(bar",
		"",
		"1foo", // must start with letter/underscore
	}
	if err := validateIdentifiers(validRow); err != nil {
		t.Fatalf("valid row rejected: %v", err)
	}
	for _, tc := range malicious {
		t.Run(tc, func(t *testing.T) {
			err := validateIdentifiers(InsertRow{Table: tc, Cols: []string{"id"}})
			if err == nil {
				t.Fatalf("expected rejection for table name %q", tc)
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Fatalf("expected 'invalid' in error for %q, got: %v", tc, err)
			}
		})
	}
	// malicious column name must also be rejected
	if err := validateIdentifiers(InsertRow{Table: "tenant", Cols: []string{"id; DROP TABLE x"}}); err == nil {
		t.Fatal("expected rejection for malicious column name")
	}
}
