// hand-written; regenerate via sqlc generate after adding queries to
// queries/ssh_keys.sql (sqlc unavailable in current env). Matches the style
// of console_connection_extra.sql.go.
// source: ssh_keys.sql
package repo

import (
	"context"
	"database/sql"
)

// SSHKeyRow is the at-rest shape: the private key + passphrase are AES-256-GCM
// ciphertext (encrypted by service.SSHKeyService with the master key). The
// fingerprint lets the UI identify a key without exposing the material.
type SSHKeyRow struct {
	ID                  int64
	Label               string
	EncryptedKey        sql.NullString
	EncryptedPassphrase sql.NullString
	Fingerprint         sql.NullString
	CreatedAt           sql.NullTime
}

const createSSHKey = `-- name: CreateSSHKey :one
INSERT INTO ssh_keys (label, encrypted_key, encrypted_passphrase, fingerprint)
VALUES (?, ?, ?, ?)
RETURNING id
`

// CreateSSHKey inserts an encrypted SSH private key + its (encrypted) passphrase
// + fingerprint, returning the new row id.
func (q *Queries) CreateSSHKey(ctx context.Context, label, encryptedKey, encryptedPassphrase, fingerprint string) (int64, error) {
	row := q.db.QueryRowContext(ctx, createSSHKey, label, encryptedKey, encryptedPassphrase, fingerprint)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

const listSSHKeys = `-- name: ListSSHKeys :many
SELECT id, label, encrypted_key, encrypted_passphrase, fingerprint, created_at
FROM ssh_keys ORDER BY id
`

// ListSSHKeys returns all stored SSH keys (oldest first).
func (q *Queries) ListSSHKeys(ctx context.Context) ([]SSHKeyRow, error) {
	rows, err := q.db.QueryContext(ctx, listSSHKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SSHKeyRow
	for rows.Next() {
		var k SSHKeyRow
		if err := rows.Scan(
			&k.ID, &k.Label, &k.EncryptedKey, &k.EncryptedPassphrase, &k.Fingerprint, &k.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

const getSSHKey = `-- name: GetSSHKey :one
SELECT id, label, encrypted_key, encrypted_passphrase, fingerprint, created_at
FROM ssh_keys WHERE id = ? LIMIT 1
`

// GetSSHKey loads one SSH key by id (for the WS handler to resolve + decrypt on
// connect). Returns sql.ErrNoRows if absent.
func (q *Queries) GetSSHKey(ctx context.Context, id int64) (SSHKeyRow, error) {
	row := q.db.QueryRowContext(ctx, getSSHKey, id)
	var k SSHKeyRow
	err := row.Scan(
		&k.ID, &k.Label, &k.EncryptedKey, &k.EncryptedPassphrase, &k.Fingerprint, &k.CreatedAt,
	)
	return k, err
}

const deleteSSHKey = `-- name: DeleteSSHKey :exec
DELETE FROM ssh_keys WHERE id = ?
`

// DeleteSSHKey removes a stored SSH key. No-op if the id doesn't exist.
func (q *Queries) DeleteSSHKey(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, deleteSSHKey, id)
	return err
}
