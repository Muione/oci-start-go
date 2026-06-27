package db

import (
	"context"
	"database/sql"
)

// WithTx runs fn inside a transaction on the write pool. On error the tx is
// rolled back; on success it is committed (the deferred rollback is then a
// no-op). The ctx deadline/traceId propagate — SPEC §16 context-everywhere.
// Repository methods accept *sql.Tx (or *sql.DB) so the service composes the
// transaction while sqlc's WithTx lets a *Queries run inside it.
func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
