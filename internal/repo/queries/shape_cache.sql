-- name: UpsertShapeCache :exec
INSERT INTO shape_cache (
    tenant_id, compartment_id, availability_domain, shape, architecture,
    ocpus, memory_in_gbs, processor_description, is_flexible,
    max_vnic_attachments, gpu_description, gpu_count,
    local_disk_description, networking_description, baseline_ocpu,
    last_synced_at, cloud_type
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?,
    ?, ?, ?,
    ?, 1
);

-- name: ListShapeCacheByTenant :many
SELECT id, tenant_id, compartment_id, availability_domain, shape, architecture,
       ocpus, memory_in_gbs, processor_description, is_flexible,
       max_vnic_attachments, gpu_description, gpu_count,
       local_disk_description, networking_description, baseline_ocpu,
       last_synced_at, cloud_type
FROM shape_cache
WHERE tenant_id = ?
ORDER BY ocpus ASC;

-- name: ListShapeCacheByTenantAndArch :many
SELECT id, tenant_id, compartment_id, availability_domain, shape, architecture,
       ocpus, memory_in_gbs, processor_description, is_flexible,
       max_vnic_attachments, gpu_description, gpu_count,
       local_disk_description, networking_description, baseline_ocpu,
       last_synced_at, cloud_type
FROM shape_cache
WHERE tenant_id = ? AND architecture = ?
ORDER BY ocpus ASC;

-- name: DeleteShapeCacheByTenant :exec
DELETE FROM shape_cache WHERE tenant_id = ?;
