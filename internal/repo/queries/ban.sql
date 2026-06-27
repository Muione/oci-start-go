-- name: FindTopBanByIpAddress :one
SELECT id, ip_address, source, operator_name, reason, status, create_time, unban_time, remark
FROM ban_record
WHERE ip_address = ?
ORDER BY id DESC
LIMIT 1;

-- name: InsertBan :exec
INSERT INTO ban_record (ip_address, source, operator_name, reason, status, create_time)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateBanStatus :exec
UPDATE ban_record
SET status = ?, reason = ?, unban_time = ?, operator_name = ?
WHERE id = ?;
