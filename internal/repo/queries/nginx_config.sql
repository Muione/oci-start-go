-- NginxConfig queries (Phase 12.1). nginx_config stores versioned, compiled
-- nginx server-block content. is_current marks the version currently applied
-- to OpenResty. ALL COMMENTS MUST BE ASCII-ONLY.

-- name: InsertNginxConfig :one
INSERT INTO nginx_config (
    config_name, config_content, is_current, config_version, config_status,
    create_time, update_time
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: FindLatestNginxConfig :one
SELECT * FROM nginx_config ORDER BY config_version DESC LIMIT 1;

-- name: FindCurrentNginxConfig :one
SELECT * FROM nginx_config WHERE is_current = 1 LIMIT 1;

-- name: FindNginxConfigById :one
SELECT * FROM nginx_config WHERE id = ?;

-- name: MarkNginxConfigCurrent :exec
UPDATE nginx_config SET is_current = 1, config_status = 'APPLIED', update_time = ? WHERE id = ?;

-- name: ClearCurrentNginxConfig :exec
UPDATE nginx_config SET is_current = 0, update_time = ? WHERE is_current = 1;
