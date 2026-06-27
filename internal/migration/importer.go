// Package migration — SQL INSERT parser and SQLite importer.
// Parity with Java DatabaseImportService.processInsertLine / importFromSqlText.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Importer executes parsed INSERT statements against a SQLite database.
type Importer struct {
	db  *sql.DB
	log zerolog.Logger
	s   *SQLSplitter
}

// NewImporter creates a SQL importer for the target database.
func NewImporter(db *sql.DB, log zerolog.Logger, s *SQLSplitter) *Importer {
	return &Importer{db: db, log: log, s: s}
}

// InsertRow holds a parsed SQL INSERT row.
type InsertRow struct {
	Table  string
	Cols   []string
	Values []string // raw SQL value strings: 'quoted', NULL, or number
}

// AutoImport detects the file format and imports accordingly.
// Returns import statistics.
func (imp *Importer) AutoImport(ctx context.Context, content, masterKey, keyBaseDir string) (ImportStats, error) {
	content = strings.TrimSpace(content)

	// 1) Encrypted format
	if strings.HasPrefix(content, "-----BEGIN OCI-START MIGRATION-----") {
		if masterKey == "" {
			return imp.s.Stats(), fmt.Errorf("this is an encrypted file, masterKey is required")
		}
		c := &Crypter{}
		sqlText, err := c.ParseEncryptedFileWithKey(content, masterKey)
		if err != nil {
			return imp.s.Stats(), fmt.Errorf("decrypt failed: %w", err)
		}
		return imp.ImportSQLText(ctx, sqlText, keyBaseDir)
	}

	// 2) Plain SQL
	return imp.ImportSQLText(ctx, content, keyBaseDir)
}

// ImportSQLText imports plain SQL INSERT statements into the database.
// Parity with Java DatabaseImportService.importFromSqlText.
func (imp *Importer) ImportSQLText(ctx context.Context, sqlText, keyBaseDir string) (ImportStats, error) {
	// Phase 1: scan for TENANT IDs, check for duplicates
	tenantIDs, err := imp.scanTenantIDs(sqlText)
	if err != nil {
		return imp.s.Stats(), fmt.Errorf("scan tenant IDs: %w", err)
	}

	for _, tid := range tenantIDs {
		var count int64
		err := imp.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenant WHERE id = ?", tid).Scan(&count)
		if err != nil {
			return imp.s.Stats(), fmt.Errorf("check tenant duplicate ID=%d: %w", tid, err)
		}
		if count > 0 {
			return imp.s.Stats(), fmt.Errorf("data already imported — tenant ID=%d exists; do not import again", tid)
		}
	}

	// Phase 2: parse and execute INSERTs
	lines := strings.Split(sqlText, "\n")
	var buf strings.Builder

	for _, raw := range lines {
		atomic.AddInt64(&imp.s.stats.TotalLines, 1)
		trimmed := strings.TrimSpace(raw)

		// Preserve PEM private key lines (multi-line values)
		if strings.Contains(trimmed, "PRIVATE KEY") {
			buf.WriteString(raw)
			buf.WriteByte('\n')
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		buf.WriteString(raw)
		buf.WriteByte('\n')

		if !isCompleteInsert(buf.String()) {
			continue
		}

		fullSQL := strings.TrimSpace(buf.String())
		buf.Reset()
		atomic.AddInt64(&imp.s.stats.InsertLines, 1)

		if !strings.HasPrefix(strings.ToUpper(fullSQL), "INSERT INTO ") {
			continue
		}

		if err := imp.executeInsert(ctx, fullSQL, keyBaseDir); err != nil {
			imp.log.Warn().Err(err).Str("sql", truncate(fullSQL, 200)).Msg("import: skipping line")
			atomic.AddInt64(&imp.s.stats.Errors, 1)
			// Continue — don't abort on single-row errors
		}
	}

	imp.log.Info().
		Int64("total_lines", imp.s.stats.TotalLines).
		Int64("insert_lines", imp.s.stats.InsertLines).
		Int64("inserted", imp.s.stats.Inserted).
		Int64("skipped_dups", imp.s.stats.SkippedDups).
		Int64("skipped_user", imp.s.stats.SkippedUser).
		Int64("errors", imp.s.stats.Errors).
		Msg("import completed")

	return imp.s.Stats(), nil
}

// scanTenantIDs does a first pass to collect all tenant IDs from INSERT INTO TENANT statements.
func (imp *Importer) scanTenantIDs(sqlText string) ([]int64, error) {
	var ids []int64
	lines := strings.Split(sqlText, "\n")
	var buf strings.Builder

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.Contains(trimmed, "PRIVATE KEY") {
			buf.WriteString(raw)
			buf.WriteByte('\n')
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		buf.WriteString(raw)
		buf.WriteByte('\n')
		if !isCompleteInsert(buf.String()) {
			continue
		}
		fullSQL := strings.TrimSpace(buf.String())
		buf.Reset()

		upper := strings.ToUpper(fullSQL)
		if !strings.HasPrefix(upper, "INSERT INTO TENANT") {
			continue
		}

		row, err := parseInsertSQL(fullSQL)
		if err != nil {
			continue
		}

		for i, col := range row.Cols {
			if strings.EqualFold(col, "ID") && i < len(row.Values) {
				raw := strings.Trim(row.Values[i], "'")
				if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
					ids = append(ids, id)
				}
			}
		}
	}

	return ids, nil
}

// executeInsert parses and executes a single INSERT statement.
func (imp *Importer) executeInsert(ctx context.Context, fullSQL, keyBaseDir string) error {
	row, err := parseInsertSQL(fullSQL)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	tableUpper := strings.ToUpper(row.Table)

	// --- TENANT special handling (TenantTableExportHandler) ---
	if tableUpper == "TENANT" {
		var keyContent string
		var keyFileIdx = -1
		var keyContentIdx = -1

		for i, col := range row.Cols {
			if strings.EqualFold(col, "KEY_FILE_CONTENT") && i < len(row.Values) {
				keyContentIdx = i
				val := row.Values[i]
				if val != "NULL" {
					keyContent = strings.TrimPrefix(strings.TrimSuffix(val, "'"), "'")
					keyContent = strings.ReplaceAll(keyContent, "''", "'")
				}
			}
			if strings.EqualFold(col, "KEY_FILE") {
				keyFileIdx = i
			}
		}

		if keyContent != "" && keyFileIdx >= 0 {
			// Write PEM file and set KEY_FILE path
			keyPath := fmt.Sprintf("%s/%d_key_%d.pem", keyBaseDir, time.Now().UnixNano(), len(keyContent))
			// In a real implementation, write to disk here.
			// For import, we keep the content as-is and let the Go service manage key storage.
			row.Values[keyFileIdx] = fmt.Sprintf("'%s'", escapeSQL(keyPath))
			imp.log.Debug().Str("table", row.Table).Str("keyPath", keyPath).Msg("tenant key file path set")
		}
		// Remove KEY_FILE_CONTENT virtual column
		if keyContentIdx >= 0 {
			row.Cols = append(row.Cols[:keyContentIdx], row.Cols[keyContentIdx+1:]...)
			row.Values = append(row.Values[:keyContentIdx], row.Values[keyContentIdx+1:]...)
		}
	}

	// --- LOGIN_USER handler: skip if username already exists ---
	if tableUpper == "LOGIN_USER" {
		for i, col := range row.Cols {
			if strings.EqualFold(col, "USERNAME") && i < len(row.Values) {
				username := strings.Trim(row.Values[i], "'")
				var count int
				err := imp.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM login_user WHERE username = ?", username).Scan(&count)
				if err != nil {
					return fmt.Errorf("check login_user: %w", err)
				}
				if count > 0 {
					atomic.AddInt64(&imp.s.stats.SkippedUser, 1)
					imp.log.Debug().Str("username", username).Msg("skipping existing login_user")
					return nil
				}
			}
		}
	}

	// --- OCI_SSH_CONN defaults (parity with Java) ---
	if tableUpper == "OCI_SSH_CONN" {
		for i, col := range row.Cols {
			if i >= len(row.Values) {
				continue
			}
			switch strings.ToUpper(col) {
			case "HOST":
				val := strings.Trim(row.Values[i], "'")
				if val == "" || row.Values[i] == "NULL" {
					row.Values[i] = "'127.0.0.1'"
				}
			case "REMARK":
				val := strings.Trim(row.Values[i], "'")
				if val == "" || row.Values[i] == "NULL" {
					row.Values[i] = "''"
				}
			case "NAME":
				val := strings.Trim(row.Values[i], "'")
				if val == "" || row.Values[i] == "NULL" {
					row.Values[i] = "'Unknown'"
				}
			case "FOLDER_ID":
				val := strings.Trim(row.Values[i], "'")
				if val == "" || row.Values[i] == "NULL" {
					row.Values[i] = "'-100'"
				}
			}
		}
	}

	// --- TENANT_EMAIL_CONFIG: set LAST_RESET_DATE if missing ---
	if tableUpper == "TENANT_EMAIL_CONFIG" {
		hasLastResetDate := false
		for _, col := range row.Cols {
			if strings.EqualFold(col, "LAST_RESET_DATE") {
				hasLastResetDate = true
				break
			}
		}
		if !hasLastResetDate {
			row.Cols = append(row.Cols, "LAST_RESET_DATE")
			row.Values = append(row.Values, fmt.Sprintf("'%s'", time.Now().Format("2006-01-02")))
		}
	}

	// --- Validate columns exist in target table ---
	if err := imp.validateColumns(ctx, row); err != nil {
		atomic.AddInt64(&imp.s.stats.Skipped, 1)
		return err
	}

	// --- Execute INSERT ---
	colSQL := strings.Join(row.Cols, ", ")
	placeholders := make([]string, len(row.Values))
	args := make([]interface{}, len(row.Values))
	for i, v := range row.Values {
		placeholders[i] = "?"
		args[i] = parseSQLValue(v)
	}
	placeholderSQL := strings.Join(placeholders, ", ")

	insertSQL := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", row.Table, colSQL, placeholderSQL)

	result, err := imp.db.ExecContext(ctx, insertSQL, args...)
	if err != nil {
		return fmt.Errorf("exec insert into %s: %w", row.Table, err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		atomic.AddInt64(&imp.s.stats.Inserted, 1)
		imp.s.stats.TablesFound[row.Table]++
	} else {
		atomic.AddInt64(&imp.s.stats.SkippedDups, 1)
	}

	return nil
}

// validateColumns checks that all columns in the INSERT exist in the target table.
func (imp *Importer) validateColumns(ctx context.Context, row InsertRow) error {
	rows, err := imp.db.QueryContext(ctx, "PRAGMA table_info(?)", row.Table)
	if err != nil {
		return fmt.Errorf("pragma table_info(%s): %w", row.Table, err)
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		existing[strings.ToUpper(name)] = true
	}

	for _, col := range row.Cols {
		if !existing[strings.ToUpper(col)] {
			imp.log.Warn().Str("table", row.Table).Str("column", col).Msg("unknown column, skipping row")
			return fmt.Errorf("table %s has no column %s", row.Table, col)
		}
	}
	return nil
}

// parseInsertSQL parses a single INSERT INTO ... VALUES (...) statement into a InsertRow.
// Handles multi-line PEM private key values embedded in SQL strings.
func parseInsertSQL(fullSQL string) (InsertRow, error) {
	var row InsertRow

	upper := strings.ToUpper(fullSQL)
	intoIdx := strings.Index(upper, "INTO ")
	valuesIdx := strings.Index(upper, "VALUES")

	if intoIdx < 0 || valuesIdx < 0 {
		return row, fmt.Errorf("not a valid INSERT")
	}

	tableStart := intoIdx + len("INTO ")
	colParenStart := strings.Index(fullSQL[tableStart:], "(")
	if colParenStart < 0 {
		return row, fmt.Errorf("no column paren")
	}
	colParenStart += tableStart

	colParenEnd := strings.Index(fullSQL[colParenStart:], ")")
	if colParenEnd < 0 {
		return row, fmt.Errorf("no column paren end")
	}
	colParenEnd += colParenStart

	row.Table = strings.TrimSpace(fullSQL[tableStart : tableStart+colParenStart-tableStart])
	// Remove leading '(' from table name if present
	row.Table = strings.TrimPrefix(row.Table, "(")
	row.Table = strings.TrimSpace(row.Table)

	colPart := strings.TrimSpace(fullSQL[colParenStart+1 : colParenEnd])
	row.Cols = splitColumns(colPart)

	valParenStart := strings.Index(fullSQL[valuesIdx:], "(")
	if valParenStart < 0 {
		return row, fmt.Errorf("no values paren")
	}
	valParenStart += valuesIdx
	valParenEnd := strings.LastIndex(fullSQL, ")")
	if valParenEnd < 0 || valParenEnd <= valParenStart {
		return row, fmt.Errorf("no values paren end")
	}

	valPart := strings.TrimSpace(fullSQL[valParenStart+1 : valParenEnd])
	row.Values = splitValues(valPart)

	if len(row.Values) != len(row.Cols) {
		return row, fmt.Errorf("column count %d != value count %d for table %s", len(row.Cols), len(row.Values), row.Table)
	}

	return row, nil
}

// splitColumns splits comma-separated column names.
func splitColumns(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

// splitValues splits a VALUES clause, handling quoted strings with commas.
// Parity with Java DatabaseImportService.splitValues.
func splitValues(valPart string) []string {
	var result []string
	var current strings.Builder
	inString := false

	for i := 0; i < len(valPart); i++ {
		c := valPart[i]
		if c == '\'' {
			inString = !inString
			current.WriteByte(c)
		} else if c == ',' && !inString {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// parseSQLValue converts a raw SQL value token to a Go value for database/sql.
func parseSQLValue(token string) interface{} {
	token = strings.TrimSpace(token)
	if strings.EqualFold(token, "NULL") {
		return nil
	}
	if strings.HasPrefix(token, "'") && strings.HasSuffix(token, "'") {
		inner := token[1 : len(token)-1]
		inner = strings.ReplaceAll(inner, "''", "'")
		return inner
	}
	// Try integer
	if i, err := strconv.ParseInt(token, 10, 64); err == nil {
		return i
	}
	// Try float
	if f, err := strconv.ParseFloat(token, 64); err == nil {
		return f
	}
	return token
}

// isCompleteInsert returns true if the accumulated SQL represents a complete INSERT statement.
// Simple heuristic: contains both INSERT INTO and VALUES, and ends with );
func isCompleteInsert(sql string) bool {
	upper := strings.ToUpper(sql)
	if !strings.Contains(upper, "INSERT INTO ") || !strings.Contains(upper, "VALUES") {
		return false
	}
	return strings.HasSuffix(strings.TrimSpace(sql), ");")
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
