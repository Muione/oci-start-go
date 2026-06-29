-- Email body (batch) queries (Phase 12.2). email_body tracks email batch
-- send operations with aggregated success/fail counts.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: InsertEmailBody :exec
INSERT INTO email_body (email_body_id, current_version, tenant_name,
    tenant_email_config_id, sender_email, title, content,
    receive_total, receive_success_total, receive_fail_total, create_time)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindEmailBodyById :one
SELECT id, email_body_id, current_version, tenant_name,
    tenant_email_config_id, sender_email, title, content,
    receive_total, receive_success_total, receive_fail_total, create_time
FROM email_body WHERE email_body_id = ?;

-- name: ListEmailBodies :many
SELECT id, email_body_id, current_version, tenant_name,
    tenant_email_config_id, sender_email, title, content,
    receive_total, receive_success_total, receive_fail_total, create_time
FROM email_body
WHERE (COALESCE(NULLIF(?, ''), email_body_id) = email_body_id)
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountEmailBodies :one
SELECT COUNT(*) FROM email_body
WHERE (COALESCE(NULLIF(?, ''), email_body_id) = email_body_id);

-- name: UpdateEmailBodyTotals :exec
UPDATE email_body SET receive_success_total = ?, receive_fail_total = ?
WHERE email_body_id = ?;

-- name: DeleteEmailBody :exec
DELETE FROM email_body WHERE email_body_id = ?;

-- name: DeleteEmailBodiesByConfigId :exec
DELETE FROM email_body WHERE tenant_email_config_id = ?;
