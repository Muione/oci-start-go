-- name: UpsertImageCache :exec
INSERT INTO image_cache (
    tenant_id, compartment_id, image_id, display_name,
    operating_system, operating_system_version, architecture,
    size_in_gbs, launch_mode, time_created,
    last_synced_at, cloud_type
) VALUES (
    ?, ?, ?, ?,
    ?, ?, ?,
    ?, ?, ?,
    ?, 1
);

-- name: ListImageCacheByTenant :many
SELECT id, tenant_id, compartment_id, image_id, display_name,
       operating_system, operating_system_version, architecture,
       size_in_gbs, launch_mode, time_created,
       last_synced_at, cloud_type
FROM image_cache
WHERE tenant_id = ?
ORDER BY operating_system, operating_system_version;

-- name: ListImageCacheByTenantAndArch :many
SELECT id, tenant_id, compartment_id, image_id, display_name,
       operating_system, operating_system_version, architecture,
       size_in_gbs, launch_mode, time_created,
       last_synced_at, cloud_type
FROM image_cache
WHERE tenant_id = ? AND architecture = ?
ORDER BY operating_system, operating_system_version;

-- name: ListImageCacheByTenantAndOS :many
SELECT id, tenant_id, compartment_id, image_id, display_name,
       operating_system, operating_system_version, architecture,
       size_in_gbs, launch_mode, time_created,
       last_synced_at, cloud_type
FROM image_cache
WHERE tenant_id = ? AND operating_system LIKE ?
ORDER BY operating_system_version;

-- name: DeleteImageCacheByTenant :exec
DELETE FROM image_cache WHERE tenant_id = ?;
