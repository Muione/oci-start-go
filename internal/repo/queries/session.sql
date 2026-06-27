-- name: FindSessionByToken :one
SELECT token, username, ip, user_agent, created_at, expires_at, last_active_at
FROM login_session
WHERE token = ?;

-- name: InsertSession :exec
INSERT INTO login_session (token, username, ip, user_agent, created_at, expires_at, last_active_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteSessionsByUsername :exec
DELETE FROM login_session WHERE username = ?;

-- name: DeleteSession :exec
DELETE FROM login_session WHERE token = ?;

-- name: TouchSessionActive :exec
UPDATE login_session SET last_active_at = ? WHERE token = ?;
