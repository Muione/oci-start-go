-- Extended instance_detail queries (Phase 5). Instance management, traffic,
-- backup records. ALL COMMENTS MUST BE ASCII-ONLY.

-- name: FindInstanceDetailByID :one
SELECT id, tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
       boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
       compartment_id, boot_volume_id, boot_volume_name, vpus_per_gb,
       ipv6_addresses, vnic_ids, username, port, password,
       processor_description, architecture, cloud_type, sys_image_backup,
       conn_time, enable_ping, on_line_enable, last_on_line_enable,
       offline_notify, resume_notify, monitor_installed, last_heartbeat, create_time
FROM instance_detail WHERE id = ?;

-- name: ListAllInstanceDetails :many
SELECT id, tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
       boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
       compartment_id, boot_volume_id, boot_volume_name, vpus_per_gb,
       ipv6_addresses, vnic_ids, username, port, password,
       processor_description, architecture, cloud_type, sys_image_backup,
       conn_time, enable_ping, on_line_enable, last_on_line_enable,
       offline_notify, resume_notify, monitor_installed, last_heartbeat, create_time
FROM instance_detail
ORDER BY create_time DESC
LIMIT ? OFFSET ?;

-- name: CountInstanceDetails :one
SELECT COUNT(*) FROM instance_detail;

-- name: UpdateInstanceDetailRemark :exec
UPDATE instance_detail SET remark = ? WHERE id = ?;

-- name: UpdateInstanceConnTime :exec
UPDATE instance_detail SET conn_time = ?, last_heartbeat = ? WHERE id = ?;

-- name: UpdateInstanceOffline :exec
UPDATE instance_detail SET on_line_enable = ?, offline_notify = ? WHERE id = ?;

-- name: UpdateInstanceResumeNotify :exec
UPDATE instance_detail SET resume_notify = 0, on_line_enable = 1 WHERE id = ?;

-- name: FindOfflineInstances :many
SELECT id, tenant_id, instance_id, display_name, public_ips, last_heartbeat, on_line_enable
FROM instance_detail
WHERE on_line_enable = 1 AND last_heartbeat IS NOT NULL AND last_heartbeat <> '' AND last_heartbeat < ?;

-- name: FindInstancesByTenantId :many
SELECT id, tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
       boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
       compartment_id, boot_volume_id, boot_volume_name, vpus_per_gb,
       ipv6_addresses, vnic_ids, username, port, password,
       processor_description, architecture, cloud_type, sys_image_backup,
       conn_time, enable_ping, on_line_enable, last_on_line_enable,
       offline_notify, resume_notify, monitor_installed, last_heartbeat, create_time
FROM instance_detail WHERE tenant_id = ?
ORDER BY create_time DESC;

-- instance_backup_detail queries

-- name: FindInstanceBackupsByTenantId :many
SELECT id, tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
       boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
       compartment_id, boot_volume_id, remark, boot_volume_name, ipv6_addresses,
       username, port, password, processor_description, architecture, cloud_type,
       sys_image_backup
FROM instance_backup_detail WHERE tenant_id = ? ORDER BY id DESC;

-- name: InsertInstanceBackupDetail :exec
INSERT INTO instance_backup_detail (
    tenant_id, instance_id, display_name, shape, state, ocpus, memory_in_gbs,
    boot_volume_size_in_gbs, public_ips, private_ips, availability_domain,
    compartment_id, boot_volume_id, remark, boot_volume_name, ipv6_addresses,
    username, port, password, processor_description, architecture, cloud_type,
    sys_image_backup
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteInstanceBackupDetail :exec
DELETE FROM instance_backup_detail WHERE id = ?;

-- traffic_alert queries

-- name: FindTrafficAlertByTenantId :one
SELECT id, tenant_id, tenancy, threshold, auto_shutdown, notification_email,
       enabled, last_notification, created_at, updated_at, statistics_enabled, cloud_type
FROM traffic_alert WHERE tenant_id = ?;

-- name: UpsertTrafficAlert :exec
INSERT INTO traffic_alert (
    tenant_id, tenancy, threshold, auto_shutdown, notification_email, enabled, cloud_type,
    statistics_enabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tenant_id) DO UPDATE SET
    threshold = excluded.threshold,
    auto_shutdown = excluded.auto_shutdown,
    notification_email = excluded.notification_email,
    enabled = excluded.enabled,
    statistics_enabled = excluded.statistics_enabled,
    updated_at = excluded.updated_at;

-- name: ListTrafficAlerts :many
SELECT id, tenant_id, tenancy, threshold, auto_shutdown, notification_email,
       enabled, last_notification, created_at, updated_at, statistics_enabled, cloud_type
FROM traffic_alert ORDER BY tenant_id;

-- instance_traffic queries

-- name: FindInstanceTrafficByInstanceId :many
SELECT id, instance_id, tenant_id, tenancy, ingress_bytes, egress_bytes,
       stats_date, last_updated, region, threshold, auto_shutdown, created_at, cloud_type, alert_sent
FROM instance_traffic WHERE instance_id = ? ORDER BY stats_date DESC LIMIT ?;

-- name: InsertInstanceTraffic :exec
INSERT INTO instance_traffic (
    instance_id, tenant_id, tenancy, ingress_bytes, egress_bytes, stats_date,
    region, threshold, auto_shutdown, cloud_type, alert_sent, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteInstanceDetail :exec
DELETE FROM instance_detail WHERE id = ?;

-- name: UpdateInstanceDetailPublicIp :exec
UPDATE instance_detail SET public_ips = ? WHERE id = ?;

-- name: UpdateInstanceDetailIpv6 :exec
UPDATE instance_detail SET ipv6_addresses = ? WHERE id = ?;

-- name: UpdateInstanceSSHConfig :exec
UPDATE instance_detail SET username = ?, port = ?, password = ? WHERE id = ?;

-- name: UpdateInstanceDetailVpusPerGb :exec
UPDATE instance_detail SET vpus_per_gb = ? WHERE id = ?;

-- name: UpdateInstanceDetailBootVolumeSize :exec
UPDATE instance_detail SET boot_volume_size_in_gbs = ? WHERE id = ?;

-- name: DeleteInstanceTrafficByTenantId :exec
DELETE FROM instance_traffic WHERE tenant_id = ?;
