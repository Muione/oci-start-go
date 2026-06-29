-- Multipart upload tracking queries (Phase 11.1). Resumable upload sessions
-- with per-part completion tracking via JSON column.

-- name: CreateMultipartUploadRecord :exec
INSERT INTO oci_multipart_upload_record
    (tenant_id, cloud_type, tenancy_ocid, namespace, bucket_name, object_name,
     upload_id, total_size, chunk_size, total_parts, completed_parts, status,
     create_time, update_time)
VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, '[]', 'uploading', ?, ?);

-- name: FindActiveUploads :many
SELECT * FROM oci_multipart_upload_record
WHERE tenant_id = ? AND bucket_name = ? AND object_name = ? AND status = 'uploading';

-- name: FindByUploadId :one
SELECT * FROM oci_multipart_upload_record WHERE upload_id = ?;

-- name: ListResumableUploads :many
SELECT * FROM oci_multipart_upload_record
WHERE tenant_id = ? AND bucket_name = ? AND status = 'uploading'
ORDER BY create_time DESC;

-- name: UpdateMultipartUploadParts :exec
UPDATE oci_multipart_upload_record
SET completed_parts = ?, update_time = ?
WHERE upload_id = ?;

-- name: UpdateMultipartUploadStatus :exec
UPDATE oci_multipart_upload_record
SET status = ?, update_time = ?
WHERE upload_id = ?;

-- name: FindStaleUploads :many
SELECT * FROM oci_multipart_upload_record
WHERE status = 'uploading' AND update_time < ?;

-- name: FixMultipartUploadTenantId :exec
UPDATE oci_multipart_upload_record
SET tenant_id = ?, update_time = ?
WHERE tenancy_ocid = ? AND tenant_id != ?;

-- name: DeleteMultipartUploadsByTenant :exec
DELETE FROM oci_multipart_upload_record WHERE tenant_id = ?;
