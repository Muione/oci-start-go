-- instance_detail queries (Phase 3 instance sync). Sync is delete-by-tenant
-- then insert-all (parity with Java syncOci: deleteByTenantId then saveAll).

-- name: DeleteInstanceDetailsByTenantId :exec
DELETE FROM instance_detail WHERE tenant_id = ?;

-- name: InsertInstanceDetail :exec
INSERT INTO instance_detail (
    tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
    boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
    compartment_id, boot_volume_id, boot_volume_name, vpus_per_gb,
    ipv6_addresses, vnic_ids, username, port, password,
    processor_description, architecture, cloud_type, create_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListInstanceDetailsByTenantId :many
SELECT
    id, tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
    boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
    compartment_id, boot_volume_id, boot_volume_name, vpus_per_gb,
    ipv6_addresses, vnic_ids, username, port, password,
    processor_description, architecture, cloud_type, sys_image_backup,
    conn_time, enable_ping, on_line_enable, last_on_line_enable,
    offline_notify, resume_notify, monitor_installed, last_heartbeat, create_time
FROM instance_detail
WHERE tenant_id = ?
ORDER BY id;
