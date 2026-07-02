-- Tenant queries (Phase 3). key_file_blob holds the AES-256-GCM encrypted
-- PEM (base64). List/mask queries deliberately omit key_file/key_file_blob.

-- name: ListTenants :many
SELECT
    id, tenant_id, user_name, fingerprint, tenancy, region, created_at,
    api_synced, enable_icmp, enable_all_protocol, is_home_region, paren_id,
    tenancy_name, tenancy_des, account_type, cloud_type, region_en, id_str,
    email_address, email_enable, transfer_status, transfer_amount, is_active
FROM tenant
ORDER BY id;

-- name: FindTenantByID :one
SELECT * FROM tenant WHERE id = ?;

-- name: FindTenantsByParenId :many
SELECT
    id, tenant_id, user_name, fingerprint, tenancy, region, created_at,
    api_synced, is_home_region, paren_id, tenancy_name, account_type,
    cloud_type, is_active
FROM tenant
WHERE paren_id = ?
ORDER BY id;

-- name: InsertTenant :exec
INSERT INTO tenant (
    tenant_id, user_name, fingerprint, tenancy, region, key_file_blob,
    created_at, cloud_type, is_home_region, account_type, tenancy_name,
    is_active
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateTenant :exec
UPDATE tenant
SET tenant_id = ?, user_name = ?, fingerprint = ?, tenancy = ?, region = ?,
    key_file_blob = ?, cloud_type = ?, is_home_region = ?, account_type = ?,
    tenancy_name = ?
WHERE id = ?;

-- name: DeleteTenant :exec
DELETE FROM tenant WHERE id = ?;

-- name: SetTenantApiSynced :exec
UPDATE tenant SET api_synced = ? WHERE id = ?;

-- name: ListTenantsWithCounts :many
-- Aggregates tenant + register_detail.register_time + boot_instance count
-- (status=1) + child count in a single round-trip, replacing the per-tenant
-- FindRegisterDetailByTenantId / CountBootInstancesByTenantId / CountTenantChildren
-- fan-out in service.TenantService.List. All three enrichments are correlated
-- subqueries: this avoids the row-multiplication a LEFT JOIN would introduce
-- when register_detail has duplicate tenant_id rows (UpsertRegisterDetail uses
-- INSERT OR REPLACE with no unique constraint), and matches the first-row
-- semantics of FindRegisterDetailByTenantId (:one).
-- ALL COMMENTS MUST BE ASCII-ONLY.
SELECT
    t.id, t.tenant_id, t.user_name, t.fingerprint, t.tenancy, t.region, t.created_at,
    t.api_synced, t.enable_icmp, t.enable_all_protocol, t.is_home_region, t.paren_id,
    t.tenancy_name, t.tenancy_des, t.account_type, t.cloud_type, t.region_en, t.id_str,
    t.email_address, t.email_enable, t.transfer_status, t.transfer_amount, t.is_active,
    (SELECT COUNT(*) FROM instance_detail WHERE tenant_id = t.id) AS instance_count,
    (SELECT rd.register_time FROM register_detail rd WHERE rd.tenant_id = t.tenant_id LIMIT 1) AS register_time,
    (SELECT COUNT(*) FROM boot_instance b WHERE b.tenant_id = t.id AND b.status = 1) AS boot_count,
    (SELECT COUNT(*) FROM tenant c WHERE c.paren_id = t.id) AS child_count
FROM tenant t
ORDER BY t.id;
