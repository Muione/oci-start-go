// hand-written; regenerate via sqlc generate after adding query to queries/instance_detail.sql
// (sqlc unavailable in current env). Matches the style of instance_detail.sql.go.
// versions:
//   sqlc v1.31.1
// source: instance_detail.sql

package repo

import (
	"context"
	"database/sql"
)

const findConsoleInstanceInfo = `-- name: FindConsoleInstanceInfo :one
SELECT instance_id, display_name, public_ips, private_ips, shape,
       username, port, password, tenant_id, compartment_id, availability_domain
FROM instance_detail WHERE instance_id = ? LIMIT 1
`

type FindConsoleInstanceInfoRow struct {
	InstanceID         sql.NullString `json:"instance_id"`
	DisplayName        sql.NullString `json:"display_name"`
	PublicIps          sql.NullString `json:"public_ips"`
	PrivateIps         sql.NullString `json:"private_ips"`
	Shape              sql.NullString `json:"shape"`
	Username           sql.NullString `json:"username"`
	Port               sql.NullInt64  `json:"port"`
	Password           sql.NullString `json:"password"`
	TenantID           sql.NullInt64  `json:"tenant_id"`
	CompartmentID      sql.NullString `json:"compartment_id"`
	AvailabilityDomain sql.NullString `json:"availability_domain"`
}

// FindConsoleInstanceInfo replaces the inline SELECT in
// cmd/oci-start/main.go wsHub.Console.SetDeps InstanceLookup.
func (q *Queries) FindConsoleInstanceInfo(ctx context.Context, instanceID string) (FindConsoleInstanceInfoRow, error) {
	row := q.db.QueryRowContext(ctx, findConsoleInstanceInfo, instanceID)
	var i FindConsoleInstanceInfoRow
	err := row.Scan(
		&i.InstanceID,
		&i.DisplayName,
		&i.PublicIps,
		&i.PrivateIps,
		&i.Shape,
		&i.Username,
		&i.Port,
		&i.Password,
		&i.TenantID,
		&i.CompartmentID,
		&i.AvailabilityDomain,
	)
	return i, err
}

const findRescueInstanceInfo = `-- name: FindRescueInstanceInfo :one
SELECT instance_id, display_name, state, boot_volume_id, shape,
       availability_domain, compartment_id, public_ips, username, password
FROM instance_detail WHERE instance_id = ? LIMIT 1
`

type FindRescueInstanceInfoRow struct {
	InstanceID         sql.NullString `json:"instance_id"`
	DisplayName        sql.NullString `json:"display_name"`
	State              sql.NullString `json:"state"`
	BootVolumeID       sql.NullString `json:"boot_volume_id"`
	Shape              sql.NullString `json:"shape"`
	AvailabilityDomain sql.NullString `json:"availability_domain"`
	CompartmentID      sql.NullString `json:"compartment_id"`
	PublicIps          sql.NullString `json:"public_ips"`
	Username           sql.NullString `json:"username"`
	Password           sql.NullString `json:"password"`
}

// FindRescueInstanceInfo replaces the inline SELECT in
// cmd/oci-start/main.go wsHub.Rescue.SetDeps GetInstance.
func (q *Queries) FindRescueInstanceInfo(ctx context.Context, instanceID string) (FindRescueInstanceInfoRow, error) {
	row := q.db.QueryRowContext(ctx, findRescueInstanceInfo, instanceID)
	var i FindRescueInstanceInfoRow
	err := row.Scan(
		&i.InstanceID,
		&i.DisplayName,
		&i.State,
		&i.BootVolumeID,
		&i.Shape,
		&i.AvailabilityDomain,
		&i.CompartmentID,
		&i.PublicIps,
		&i.Username,
		&i.Password,
	)
	return i, err
}

const findCompartmentID = `-- name: FindCompartmentID :one
SELECT compartment_id FROM instance_detail WHERE instance_id = ? LIMIT 1
`

// FindCompartmentID replaces the inline compartment_id lookup in
// cmd/oci-start/main.go wsHub.Console.SetDeps GetCompartmentID. Returns
// sql.NullString (Valid=false when the column is NULL/empty); sql.ErrNoRows
// when the instance row itself is absent.
func (q *Queries) FindCompartmentID(ctx context.Context, instanceID string) (sql.NullString, error) {
	row := q.db.QueryRowContext(ctx, findCompartmentID, instanceID)
	var compID sql.NullString
	err := row.Scan(&compID)
	return compID, err
}
