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
