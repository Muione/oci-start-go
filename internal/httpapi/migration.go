// Package httpapi — migration.go: data migration HTTP endpoints (Phase 8).
// Provides web-based import of Java H2 export files (plain SQL and encrypted .enc).
// Parity with Java MigrationController.
package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Muione/oci-start-go/internal/migration"
	"github.com/Muione/oci-start-go/internal/response"
)

// MigrationHandler handles data migration endpoints.
type MigrationHandler struct {
	importer   *migration.Importer
	splitter   *migration.SQLSplitter
	keyBaseDir string
	db         *sql.DB // for export
}

// NewMigrationHandler creates a migration handler.
func NewMigrationHandler(importer *migration.Importer, splitter *migration.SQLSplitter, keyBaseDir string, db *sql.DB) *MigrationHandler {
	return &MigrationHandler{
		importer:   importer,
		splitter:   splitter,
		keyBaseDir: keyBaseDir,
		db:         db,
	}
}

// ImportPlain imports a plain SQL file upload.
// POST /migration/import
func (h *MigrationHandler) ImportPlain(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "请选择一个SQL文件上传")
		return
	}

	f, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "无法读取上传文件")
		return
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "读取文件内容失败")
		return
	}

	stats, err := h.importer.ImportSQLText(context.Background(), string(content), h.keyBaseDir)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, fmt.Sprintf("导入失败: %v", err))
		return
	}

	response.OK(c, response.SuccessData(gin.H{
		"totalLines":     stats.TotalLines,
		"insertLines":    stats.InsertLines,
		"inserted":       stats.Inserted,
		"skipped":        stats.Skipped,
		"skippedDups":    stats.SkippedDups,
		"skippedUser":    stats.SkippedUser,
		"errors":         stats.Errors,
		"tablesFound":    stats.TablesFound,
		"message":        fmt.Sprintf("导入完成: 成功 %d, 跳过 %d, 错误 %d", stats.Inserted, stats.Skipped+stats.SkippedDups+stats.SkippedUser, stats.Errors),
	}))
}

// ImportEncrypted imports an encrypted .enc file upload.
// POST /migration/import-encrypted
func (h *MigrationHandler) ImportEncrypted(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "请选择一个备份文件上传")
		return
	}

	masterKey := strings.TrimSpace(c.PostForm("masterKey"))

	f, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "无法读取上传文件")
		return
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "读取文件内容失败")
		return
	}

	stats, err := h.importer.AutoImport(context.Background(), string(content), masterKey, h.keyBaseDir)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, fmt.Sprintf("导入失败: %v", err))
		return
	}

	// If masterKey was provided, return import stats
	_ = masterKey

	response.OK(c, response.SuccessData(gin.H{
		"totalLines":  stats.TotalLines,
		"insertLines": stats.InsertLines,
		"inserted":    stats.Inserted,
		"skipped":     stats.Skipped,
		"skippedDups": stats.SkippedDups,
		"skippedUser": stats.SkippedUser,
		"errors":      stats.Errors,
		"tablesFound": stats.TablesFound,
		"message":     fmt.Sprintf("导入完成: 成功 %d, 跳过 %d, 错误 %d", stats.Inserted, stats.Skipped+stats.SkippedDups+stats.SkippedUser, stats.Errors),
	}))
}

// ExportPlain exports the current SQLite database as plain SQL INSERT statements.
// GET /migration/export
func (h *MigrationHandler) ExportPlain(c *gin.Context) {
	if h.db == nil {
		response.Fail(c, http.StatusServiceUnavailable, "database not available for export")
		return
	}

	ctx := c.Request.Context()

	// Get all table names, excluding sqlite_* system tables.
	tables, err := h.getTableNames(ctx)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list tables: "+err.Error())
		return
	}

	// Tables to skip (grab engine state tables, same as Java IGNORE_TABLES).
	skipTables := map[string]bool{
		"BOOT_INSTANCE":     true,
		"OCI_COMPUTER_INFO": true,
		"LOGIN_USER":        true,
		"INSTANCE_TRAFFIC":  true,
		"INSTALL_APP":       true,
		"SQLITE_SEQUENCE":   true,
		"APP_VERSION":       true,
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var sb strings.Builder
	sb.WriteString("-- OCI-START-GO PLAIN BACKUP\n")
	sb.WriteString("-- GENERATED AT: " + now + "\n")
	sb.WriteString("--\n\n")

	for _, table := range tables {
		upper := strings.ToUpper(table)
		if skipTables[upper] {
			continue
		}

		sb.WriteString("-- ----------------------------\n")
		sb.WriteString("-- TABLE: " + table + "\n")
		sb.WriteString("-- ----------------------------\n")

		if err := h.exportTable(ctx, table, &sb); err != nil {
			sb.WriteString("-- ERROR exporting " + table + ": " + err.Error() + "\n")
		}
		sb.WriteString("\n")
	}

	fileName := fmt.Sprintf("oci-start-go_backup_%d.sql", time.Now().Unix())
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=\"" + fileName + "\"")
	c.String(http.StatusOK, sb.String())
}

func (h *MigrationHandler) getTableNames(ctx context.Context) ([]string, error) {
	rows, err := h.db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (h *MigrationHandler) exportTable(ctx context.Context, table string, sb *strings.Builder) error {
	// Get column names.
	colRows, err := h.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer colRows.Close()

	var cols []string
	for colRows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := colRows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		// Skip ID column for non-tenant tables (parity with Java export).
		if !strings.EqualFold(table, "tenant") && strings.EqualFold(name, "id") {
			continue
		}
		cols = append(cols, name)
	}

	if len(cols) == 0 {
		return nil
	}

	colList := strings.Join(cols, ", ")
	query := "SELECT " + colList + " FROM " + table

	dataRows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer dataRows.Close()

	vals := make([]interface{}, len(cols))
	valPtrs := make([]interface{}, len(cols))
	for i := range vals {
		valPtrs[i] = &vals[i]
	}

	for dataRows.Next() {
		if err := dataRows.Scan(valPtrs...); err != nil {
			continue
		}

		sb.WriteString("INSERT INTO ")
		sb.WriteString(table)
		sb.WriteString(" (")
		sb.WriteString(colList)
		sb.WriteString(") VALUES (")

		for i, v := range vals {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(formatSQLValue(v))
		}
		sb.WriteString(");\n")
	}

	return dataRows.Err()
}

func formatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case []byte:
		return "'" + strings.ReplaceAll(string(val), "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	}
}

// Ensure key directory exists
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
