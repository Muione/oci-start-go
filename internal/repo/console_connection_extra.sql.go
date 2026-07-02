// hand-written; regenerate via sqlc generate after adding queries to
// queries/console_connections.sql (sqlc unavailable in current env). Matches
// the style of instance_detail_extra.sql.go.
// source: console_connections.sql
package repo

import (
	"context"
	"database/sql"
)

// ConsoleConnectionRow is the post-0009 shape returned by GetConsoleConnection
// byInstance: the connection OCID + the AES-encrypted private key PEM + the
// SSH public key, for resume of an app-created console connection.
type ConsoleConnectionRow struct {
	ID                  int64
	InstanceID          string
	TenantID            int64
	ConnectionID        string
	EncryptedPrivateKey sql.NullString
	PublicKeySSH        sql.NullString
	CreatedAt           sql.NullTime
}

const upsertConsoleConnection = `-- name: UpsertConsoleConnection :exec
INSERT INTO console_connections (instance_id, tenant_id, connection_id, encrypted_private_key, public_key_ssh)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(instance_id) DO UPDATE SET
    tenant_id = excluded.tenant_id,
    connection_id = excluded.connection_id,
    encrypted_private_key = excluded.encrypted_private_key,
    public_key_ssh = excluded.public_key_ssh
`

// UpsertConsoleConnection persists (or replaces) the app-created console
// connection for an instance: its OCI connection OCID + AES-encrypted private
// key PEM + SSH public key. One row per instance (unique index) so the latest
// creation wins, enabling resume across app restarts.
func (q *Queries) UpsertConsoleConnection(ctx context.Context, instanceID string, tenantID int64, connectionID, encryptedPrivateKey, publicKeySSH string) error {
	_, err := q.db.ExecContext(ctx, upsertConsoleConnection, instanceID, tenantID, connectionID, encryptedPrivateKey, publicKeySSH)
	return err
}

const getConsoleConnectionByInstance = `-- name: GetConsoleConnectionByInstance :one
SELECT id, instance_id, tenant_id, connection_id, encrypted_private_key, public_key_ssh, created_at
FROM console_connections WHERE instance_id = ? LIMIT 1
`

// GetConsoleConnectionByInstance loads the app-created console connection row
// for an instance (for resume). Returns sql.ErrNoRows if none.
func (q *Queries) GetConsoleConnectionByInstance(ctx context.Context, instanceID string) (ConsoleConnectionRow, error) {
	row := q.db.QueryRowContext(ctx, getConsoleConnectionByInstance, instanceID)
	var c ConsoleConnectionRow
	err := row.Scan(
		&c.ID,
		&c.InstanceID,
		&c.TenantID,
		&c.ConnectionID,
		&c.EncryptedPrivateKey,
		&c.PublicKeySSH,
		&c.CreatedAt,
	)
	return c, err
}

const deleteConsoleConnectionByInstance = `-- name: DeleteConsoleConnectionByInstance :exec
DELETE FROM console_connections WHERE instance_id = ?
`

// DeleteConsoleConnectionByInstance removes the app's tracked console
// connection row for an instance (after the OCI connection is deleted, or to
// drop a stale row). No-op if no row exists.
func (q *Queries) DeleteConsoleConnectionByInstance(ctx context.Context, instanceID string) error {
	_, err := q.db.ExecContext(ctx, deleteConsoleConnectionByInstance, instanceID)
	return err
}
