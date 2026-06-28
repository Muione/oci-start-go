-- Extended tenant queries (Phase 9). Tenant detail, custom name, active days.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: UpdateTenantFields :exec
UPDATE tenant
SET tenancy_name = ?, tenancy_des = ?, account_type = ?, email_address = ?, is_active = ?
WHERE id = ?;

-- name: FindTenantFullByID :one
SELECT id, tenant_id, user_name, fingerprint, tenancy, region, created_at,
       api_synced, enable_icmp, enable_all_protocol, is_home_region, paren_id,
       tenancy_name, tenancy_des, account_type, cloud_type, region_en, id_str,
       email_address, email_enable, transfer_status, transfer_amount, is_active,
       key_file_blob
FROM tenant WHERE id = ?;
