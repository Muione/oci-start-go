-- name: GetAppVersion :one
SELECT id, current_version, latest_version, deploy_type, create_time, update_time
FROM app_version
ORDER BY id ASC
LIMIT 1;

-- name: CountAppVersion :one
SELECT count(*) FROM app_version;

-- name: SeedAppVersion :exec
INSERT OR IGNORE INTO app_version (id, current_version, latest_version, deploy_type, create_time, update_time)
VALUES (1, ?, ?, ?, ?, ?);

-- name: UpdateAppVersion :exec
UPDATE app_version
SET current_version = ?, latest_version = ?, update_time = ?
WHERE id = 1;
