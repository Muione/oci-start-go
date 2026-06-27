-- name: FindUserByUsername :one
SELECT id, username, password, is_first_user, login_type, external_id, last_login_at, role
FROM login_user
WHERE username = ?
LIMIT 1;

-- name: FindUserByExternalIdAndLoginType :one
SELECT id, username, password, is_first_user, login_type, external_id, last_login_at, role
FROM login_user
WHERE external_id = ? AND login_type = ?
LIMIT 1;

-- name: CountByLoginType :one
SELECT count(*) FROM login_user WHERE login_type = ?;

-- name: ExistsByUsername :one
SELECT count(*) FROM login_user WHERE username = ?;

-- name: InsertLoginUser :exec
INSERT INTO login_user (username, password, is_first_user, login_type, external_id, last_login_at, role)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateLastLoginAt :exec
UPDATE login_user SET last_login_at = ? WHERE username = ?;

-- name: UpdateUserCredentials :exec
UPDATE login_user SET username = ?, password = ? WHERE username = ?;
