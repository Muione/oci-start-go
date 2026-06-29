-- Email send record queries (Phase 12.2). email_send_record tracks per-recipient
-- send results for each email body batch.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: InsertEmailSendRecord :exec
INSERT INTO email_send_record (email_send_record_id, email_body_id,
    email_send_address, current_version, tenant_name,
    email_receive_id, receive_email_address, send_state, create_time)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListEmailSendRecords :many
SELECT id, email_send_record_id, email_body_id, email_send_address,
    current_version, tenant_name, email_receive_id,
    receive_email_address, send_state, create_time
FROM email_send_record
WHERE email_body_id = ?
ORDER BY id ASC
LIMIT ? OFFSET ?;

-- name: CountEmailSendRecords :one
SELECT COUNT(*) FROM email_send_record WHERE email_body_id = ?;

-- name: UpdateEmailSendRecordState :exec
UPDATE email_send_record SET send_state = ? WHERE email_send_record_id = ?;

-- name: DeleteEmailSendRecordsByBodyId :exec
DELETE FROM email_send_record WHERE email_body_id = ?;

-- name: DeleteEmailSendRecordsByConfigId :exec
DELETE FROM email_send_record
WHERE email_body_id IN (SELECT email_body_id FROM email_body WHERE tenant_email_config_id = ?);
