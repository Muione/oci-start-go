// Command migrate is a standalone CLI tool for importing data from a Java
// oci-start H2 database export into the Go SQLite database.
//
// Usage:
//
//	migrate -db </path/to/oci-start.db> -file <export.sql|export.enc> [-key <master-key>] [-keydir <dir>]
//
// Supports plain SQL exports and AES-256-CBC encrypted .enc files.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/Muione/oci-start-go/internal/migration"
	"github.com/rs/zerolog"
)

func main() {
	dbPath := flag.String("db", "", "Path to target SQLite database file")
	filePath := flag.String("file", "", "Path to SQL export file (.sql or .enc)")
	masterKey := flag.String("key", "", "Master key for encrypted .enc files (required for .enc)")
	keyDir := flag.String("keydir", "/tmp/oci-start-keys", "Directory for extracted PEM key files")
	flag.Parse()

	if *dbPath == "" || *filePath == "" {
		fmt.Fprintf(os.Stderr, "Usage: migrate -db <db> -file <export> [-key <master-key>] [-keydir <dir>]\n")
		os.Exit(1)
	}

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()

	// Read the export file
	data, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatal().Err(err).Str("file", *filePath).Msg("cannot read input file")
	}

	content := string(data)

	// Auto-detect encrypted format
	if strings.HasPrefix(strings.TrimSpace(content), "-----BEGIN OCI-START MIGRATION-----") {
		if *masterKey == "" {
			log.Fatal().Msg("encrypted file detected — master key is required (-key flag)")
		}
		log.Info().Msg("detected encrypted .enc format, decrypting...")
	}

	// Ensure key directory exists
	if err := os.MkdirAll(*keyDir, 0755); err != nil {
		log.Fatal().Err(err).Str("dir", *keyDir).Msg("cannot create key directory")
	}

	// Open target database
	db, err := sql.Open("sqlite", *dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatal().Err(err).Str("db", *dbPath).Msg("cannot open database")
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

	// Run import
	splitter := migration.NewSQLSplitter(log)
	importer := migration.NewImporter(db, log, splitter)

	ctx := context.Background()
	stats, err := importer.AutoImport(ctx, content, *masterKey, *keyDir)
	if err != nil {
		log.Fatal().Err(err).Msg("import failed")
	}

	log.Info().
		Int64("total_lines", stats.TotalLines).
		Int64("insert_statements", stats.InsertLines).
		Int64("rows_inserted", stats.Inserted).
		Int64("skipped", stats.Skipped).
		Int64("skipped_dups", stats.SkippedDups).
		Int64("skipped_user", stats.SkippedUser).
		Int64("errors", stats.Errors).
		Msg("import completed successfully")
}
