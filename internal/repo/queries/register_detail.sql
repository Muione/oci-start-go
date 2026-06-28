-- RegisterDetail queries (Phase 9). register_detail stores OCI subscription
-- information including register_time (timeStart) used for active-days calculation.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: FindRegisterDetailByTenantId :one
SELECT id, tenant_prv_id, tenant_id, account_type, plan_type, register_time,
       city, country, email_address, first_name, last_name, line1, postal_code,
       subscription_plan_number, upgrade_state, created_time, updated_time, cloud_type
FROM register_detail
WHERE tenant_id = ?;

-- name: UpsertRegisterDetail :exec
INSERT OR REPLACE INTO register_detail (
    tenant_prv_id, tenant_id, account_type, plan_type, register_time,
    city, country, email_address, first_name, last_name, line1, postal_code,
    subscription_plan_number, upgrade_state, created_time, updated_time, cloud_type
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
