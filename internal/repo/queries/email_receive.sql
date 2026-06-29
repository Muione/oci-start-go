-- Email receive (recipient) queries (Phase 12.2). email_receive stores
-- email recipients for batch sending.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: FindEmailReceiveById :one
SELECT id, email, name, create_time, update_time
FROM email_receive WHERE id = ?;

-- name: ListEmailReceives :many
SELECT id, email, name, create_time, update_time
FROM email_receive
WHERE (COALESCE(NULLIF(?, ''), email) = email)
  AND (COALESCE(NULLIF(?, ''), name) = name)
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountEmailReceives :one
SELECT COUNT(*) FROM email_receive
WHERE (COALESCE(NULLIF(?, ''), email) = email)
  AND (COALESCE(NULLIF(?, ''), name) = name);

-- name: InsertEmailReceive :exec
INSERT INTO email_receive (email, name, create_time, update_time)
VALUES (?, ?, ?, ?);

-- name: DeleteEmailReceive :exec
DELETE FROM email_receive WHERE id = ?;

-- NOTE: FindEmailReceivesByIds is implemented manually in Go using
-- dynamic IN clause construction (not a sqlc-generated query).
