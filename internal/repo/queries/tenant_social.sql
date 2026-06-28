-- Tenant social config queries (Phase 9). tenant_social stores third-party OAuth
-- configuration per tenant (Google, GitHub, Microsoft, etc.).

-- name: ListTenantSocialByTenantId :many
SELECT id, tenant_id, tenancy, cloud_type, client_id, client_secret,
       social_type_str, third_login_address, redirect_url, social_status
FROM tenant_social WHERE tenant_id = ? ORDER BY id;

-- name: FindTenantSocialByType :one
SELECT id, tenant_id, tenancy, cloud_type, client_id, client_secret,
       social_type_str, third_login_address, redirect_url, social_status
FROM tenant_social WHERE tenant_id = ? AND social_type_str = ?;

-- name: InsertTenantSocial :exec
INSERT INTO tenant_social (
    tenant_id, tenancy, cloud_type, client_id, client_secret,
    social_type_str, third_login_address, redirect_url, social_status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateTenantSocial :exec
UPDATE tenant_social
SET client_id = ?, client_secret = ?, redirect_url = ?, third_login_address = ?
WHERE id = ?;

-- name: UpdateTenantSocialStatus :exec
UPDATE tenant_social SET social_status = ? WHERE id = ?;

-- name: DeleteTenantSocial :exec
DELETE FROM tenant_social WHERE id = ?;
