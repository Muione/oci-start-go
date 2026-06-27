// Package db provides the SQLite read/write store, boot-time migrations, and a
// context-aware transaction helper. Pure-Go (modernc.org/sqlite, no cgo).
// See SPEC §5.1, §16.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file" // register "file" source
	_ "modernc.org/sqlite"                               // register "sqlite" driver
)

// Store holds isolated read/write pools. Write is serialized (MaxOpenConns=1)
// to avoid SQLITE_BUSY; Read is unlimited — WAL allows concurrent readers that
// never block the single writer.
type Store struct {
	Write *sql.DB
	Read  *sql.DB
}

// Open creates the write/read pools. Pragmas (WAL, busy_timeout, foreign_keys,
// synchronous) are applied per-connection via the DSN's _pragma query params.
func Open(dsn string, maxOpenConns, readMaxOpenConns int) (*Store, error) {
	writeDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	writeDB.SetMaxOpenConns(maxOpenConns)
	if err := writeDB.Ping(); err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("ping write pool: %w", err)
	}

	readDSN := dsn
	if strings.Contains(readDSN, "?") {
		readDSN += "&_pragma=query_only(1)"
	} else {
		readDSN += "?_pragma=query_only(1)"
	}
	readDB, err := sql.Open("sqlite", readDSN)
	if err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	readDB.SetMaxOpenConns(readMaxOpenConns)
	if err := readDB.Ping(); err != nil {
		_ = readDB.Close()
		_ = writeDB.Close()
		return nil, fmt.Errorf("ping read pool: %w", err)
	}

	return &Store{Write: writeDB, Read: readDB}, nil
}

// Migrate applies pending migrations from migrationsPath, reusing the write
// pool via golang-migrate's modernc-backed sqlite driver. ErrNoChange (already
// up to date) is not an error.
func (s *Store) Migrate(migrationsPath string) error {
	drv, err := sqlite.WithInstance(s.Write, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "sqlite", drv)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}
	// NOTE: deliberately do NOT call m.Close(). The sqlite driver's Close()
	// closes the underlying *sql.DB, which here is our shared write pool
	// (passed via WithInstance). Closing it would break all subsequent app
	// queries. The file source holds no persistent handles, so skipping
	// Close leaks nothing meaningful (the migrate struct is GC'd).
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Close closes both pools.
func (s *Store) Close() error {
	werr := s.Write.Close()
	rerr := s.Read.Close()
	if werr != nil {
		return werr
	}
	return rerr
}
