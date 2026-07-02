// hand-written; regenerate via sqlc generate after adding query to queries/tenant.sql
// (sqlc unavailable in current env). Matches the style of tenant.sql.go.
// versions:
//   sqlc v1.31.1
// source: tenant.sql

package repo

import (
	"context"
	"database/sql"
)

const listTenantsWithCounts = `-- name: ListTenantsWithCounts :many
SELECT
    t.id, t.tenant_id, t.user_name, t.fingerprint, t.tenancy, t.region, t.created_at,
    t.api_synced, t.enable_icmp, t.enable_all_protocol, t.is_home_region, t.paren_id,
    t.tenancy_name, t.tenancy_des, t.account_type, t.cloud_type, t.region_en, t.id_str,
    t.email_address, t.email_enable, t.transfer_status, t.transfer_amount, t.is_active,
    (SELECT rd.register_time FROM register_detail rd WHERE rd.tenant_id = t.tenant_id LIMIT 1) AS register_time,
    (SELECT COUNT(*) FROM boot_instance b WHERE b.tenant_id = t.id AND b.status = 1) AS boot_count,
    (SELECT COUNT(*) FROM tenant c WHERE c.paren_id = t.id) AS child_count
FROM tenant t
ORDER BY t.id
`

type ListTenantsWithCountsRow struct {
	ID                int64          `json:"id"`
	TenantID          sql.NullString `json:"tenant_id"`
	UserName          sql.NullString `json:"user_name"`
	Fingerprint       sql.NullString `json:"fingerprint"`
	Tenancy           sql.NullString `json:"tenancy"`
	Region            sql.NullString `json:"region"`
	CreatedAt         sql.NullString `json:"created_at"`
	ApiSynced         sql.NullInt64  `json:"api_synced"`
	EnableIcmp        sql.NullInt64  `json:"enable_icmp"`
	EnableAllProtocol sql.NullInt64  `json:"enable_all_protocol"`
	IsHomeRegion      sql.NullInt64  `json:"is_home_region"`
	ParenID           sql.NullInt64  `json:"paren_id"`
	TenancyName       sql.NullString `json:"tenancy_name"`
	TenancyDes        sql.NullString `json:"tenancy_des"`
	AccountType       sql.NullString `json:"account_type"`
	CloudType         sql.NullInt64  `json:"cloud_type"`
	RegionEn          sql.NullString `json:"region_en"`
	IDStr             sql.NullString `json:"id_str"`
	EmailAddress      sql.NullString `json:"email_address"`
	EmailEnable       sql.NullInt64  `json:"email_enable"`
	TransferStatus    sql.NullInt64  `json:"transfer_status"`
	TransferAmount    sql.NullString `json:"transfer_amount"`
	IsActive          sql.NullInt64  `json:"is_active"`
	RegisterTime      sql.NullString `json:"register_time"`
	BootCount         int64          `json:"boot_count"`
	ChildCount        int64          `json:"child_count"`
}

// ListTenantsWithCounts returns every tenant together with its register_time,
// active boot-instance count (status=1) and child count in one round-trip,
// replacing the per-tenant fan-out in service.TenantService.List.
func (q *Queries) ListTenantsWithCounts(ctx context.Context) ([]ListTenantsWithCountsRow, error) {
	rows, err := q.db.QueryContext(ctx, listTenantsWithCounts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListTenantsWithCountsRow{}
	for rows.Next() {
		var i ListTenantsWithCountsRow
		if err := rows.Scan(
			&i.ID,
			&i.TenantID,
			&i.UserName,
			&i.Fingerprint,
			&i.Tenancy,
			&i.Region,
			&i.CreatedAt,
			&i.ApiSynced,
			&i.EnableIcmp,
			&i.EnableAllProtocol,
			&i.IsHomeRegion,
			&i.ParenID,
			&i.TenancyName,
			&i.TenancyDes,
			&i.AccountType,
			&i.CloudType,
			&i.RegionEn,
			&i.IDStr,
			&i.EmailAddress,
			&i.EmailEnable,
			&i.TransferStatus,
			&i.TransferAmount,
			&i.IsActive,
			&i.RegisterTime,
			&i.BootCount,
			&i.ChildCount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
