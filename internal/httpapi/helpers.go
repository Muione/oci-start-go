// Package httpapi — helpers.go: small shared helpers for handlers.
package httpapi

import "database/sql"

// ns unwraps a nullable string; empty when NULL.
func ns(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// nullStr wraps a plain string into a nullable; NULL when empty.
func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullInt64 wraps an int64 into a nullable; NULL when zero.
func nullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v != 0}
}
