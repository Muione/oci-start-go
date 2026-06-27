-- Grabber queries (Phase 4). boot_instance, open_boot_lock, oci_computer_info,
-- tem_instance. See SPEC S8; parity with BootInstanceRepository +
-- OpenBootLockRepository + BootTotalInstanceService queries.
-- ALL COMMENTS MUST BE ASCII-ONLY (sqlc v1.31.1 sqlite parser corrupts queries
-- when a -- comment contains non-ASCII characters).

-- name: FindDistinctTasksToExecute :many
SELECT b.id, b.boot_id, b.tenant_id, b.ocpu, b.memory, b.disk, b.loop_time,
       b.instance_count, b.status, b.architecture, b.root_password, b.public_ip,
       b.next_execution_time, b.add_count, b.success_count, b.remark,
       b.created_at, b.updated_at, b.cloud_type, b.current_attempt_count,
       b.yesterday_attempt_count, b.reset_today_flag, b.last_reset_date,
       b.fail_count, b.total_count, b.image_id, b.operating_system,
       b.operating_system_version, b.data_gap, b.notify_flag
FROM boot_instance b
WHERE b.status = 1 AND b.next_execution_time <= ?1
  AND b.next_execution_time = (
      SELECT MIN(b2.next_execution_time)
      FROM boot_instance b2
      WHERE b2.status = 1
        AND b2.tenant_id = b.tenant_id
        AND COALESCE(b2.architecture, '') = COALESCE(b.architecture, '')
        AND b2.next_execution_time <= ?1
  )
ORDER BY b.next_execution_time ASC
LIMIT ?2;

-- name: FindBootInstanceByID :one
SELECT id, version, boot_id, tenant_id, ocpu, memory, disk, loop_time, instance_count, status, architecture, root_password, public_ip, next_execution_time, add_count, success_count, remark, created_at, updated_at, cloud_type, current_attempt_count, yesterday_attempt_count, reset_today_flag, last_reset_date, fail_count, total_count, image_id, operating_system, operating_system_version, data_gap, notify_flag FROM boot_instance WHERE id = ?;

-- name: FindBootInstanceByBootID :one
SELECT id, version, boot_id, tenant_id, ocpu, memory, disk, loop_time, instance_count, status, architecture, root_password, public_ip, next_execution_time, add_count, success_count, remark, created_at, updated_at, cloud_type, current_attempt_count, yesterday_attempt_count, reset_today_flag, last_reset_date, fail_count, total_count, image_id, operating_system, operating_system_version, data_gap, notify_flag FROM boot_instance WHERE boot_id = ?;

-- name: ListBootInstances :many
SELECT id, version, boot_id, tenant_id, ocpu, memory, disk, loop_time, instance_count, status, architecture, root_password, public_ip, next_execution_time, add_count, success_count, remark, created_at, updated_at, cloud_type, current_attempt_count, yesterday_attempt_count, reset_today_flag, last_reset_date, fail_count, total_count, image_id, operating_system, operating_system_version, data_gap, notify_flag FROM boot_instance ORDER BY created_at DESC;

-- name: InsertBootInstance :exec
INSERT INTO boot_instance (
    boot_id, tenant_id, ocpu, memory, disk, loop_time, instance_count, status, architecture,
    root_password, image_id, operating_system, operating_system_version,
    next_execution_time, cloud_type, data_gap, notify_flag, remark, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateBootInstance :exec
UPDATE boot_instance SET
    ocpu = ?, memory = ?, disk = ?, loop_time = ?, instance_count = ?,
    architecture = ?, image_id = ?, operating_system = ?,
    operating_system_version = ?, data_gap = ?, notify_flag = ?,
    remark = ?, updated_at = ?
WHERE boot_id = ?;

-- name: DisableBootInstance :exec
UPDATE boot_instance SET status = 0, updated_at = ? WHERE boot_id = ?;

-- name: EnableBootInstance :exec
UPDATE boot_instance SET status = 1, next_execution_time = ?, updated_at = ? WHERE boot_id = ?;

-- name: CountRunningTasks :one
SELECT COUNT(*) FROM boot_instance WHERE status = 1;

-- name: CountTotalTasks :one
SELECT COUNT(*) FROM boot_instance;

-- name: ResetDailyCounts :exec
UPDATE boot_instance SET current_attempt_count = 0 WHERE reset_today_flag = 0 OR last_reset_date <> ?;

-- name: MarkDailyReset :exec
UPDATE boot_instance SET reset_today_flag = 1, last_reset_date = ? WHERE status = 1;

-- name: AdvanceNextExecutionTime :exec
UPDATE boot_instance SET next_execution_time = ? WHERE id = ?;

-- name: IncrementFailCount :exec
UPDATE boot_instance SET fail_count = fail_count + 1 WHERE id = ?;

-- name: UpdateBootInstanceStatus :exec
UPDATE boot_instance SET status = ?, public_ip = ?, updated_at = ? WHERE id = ?;

-- name: MarkNotificationAsSent :exec
UPDATE boot_instance SET notify_flag = 'YES' WHERE id = ? AND notify_flag = 'NO';

-- name: IncrementSuccessCount :exec
UPDATE boot_instance SET success_count = success_count + 1, current_attempt_count = current_attempt_count + 1, updated_at = ? WHERE id = ?;

-- open_boot_lock queries

-- name: FindLockByTaskID :one
SELECT task_id, cloud_type, status, ins_id, create_time FROM open_boot_lock WHERE task_id = ?;

-- name: InsertLockIgnore :exec
INSERT OR IGNORE INTO open_boot_lock (task_id, cloud_type, status, create_time)
VALUES (?, 1, 'PROCESSING', ?);

-- name: UpdateLockSuccess :exec
UPDATE open_boot_lock SET status = 'SUCCESS', ins_id = ? WHERE task_id = ?;

-- name: DeleteLock :exec
DELETE FROM open_boot_lock WHERE task_id = ?;

-- name: DeleteProcessingLocks :exec
DELETE FROM open_boot_lock WHERE status = 'PROCESSING';

-- oci_computer_info queries

-- name: FindComputerInfoByBootIDStr :one
SELECT id, boot_id_str, computer_create_json, tenant_id, architecture, cloud_type, computer_region FROM oci_computer_info WHERE boot_id_str = ?;

-- name: InsertComputerInfo :exec
INSERT INTO oci_computer_info (boot_id_str, computer_create_json, tenant_id, architecture, cloud_type, computer_region)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateComputerInfo :exec
UPDATE oci_computer_info SET computer_create_json = ?, architecture = ?, computer_region = ? WHERE boot_id_str = ?;

-- name: DeleteComputerInfo :exec
DELETE FROM oci_computer_info WHERE boot_id_str = ?;

-- tem_instance queries (temp instance tracking during grab)

-- name: InsertTemInstance :exec
INSERT INTO tem_instance (tenancy, instance_id, public_ip, region, architecture, root_password, clone_boot_volume_id, cloud_type)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteTemInstancesByTenancy :exec
DELETE FROM tem_instance WHERE tenancy = ?;
