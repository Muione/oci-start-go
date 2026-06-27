-- system_config is the runtime KV store (config_key UNIQUE).
-- Boolean configs use the config_enabled column (INTEGER 0/1);
-- string configs use config_value. Mirrors Java SystemConfigService.

-- name: FindConfigByKey :one
SELECT id, config_key, config_value, config_enabled, last_modified
FROM system_config
WHERE config_key = ?
LIMIT 1;

-- name: UpsertConfig :exec
INSERT INTO system_config (config_key, config_value, config_enabled, last_modified)
VALUES (?, ?, ?, ?)
ON CONFLICT(config_key) DO UPDATE SET
    config_value = excluded.config_value,
    config_enabled = excluded.config_enabled,
    last_modified = excluded.last_modified;

-- name: UpsertConfigValue :exec
INSERT INTO system_config (config_key, config_value, config_enabled, last_modified)
VALUES (?, ?, ?, ?)
ON CONFLICT(config_key) DO UPDATE SET
    config_value = excluded.config_value,
    last_modified = excluded.last_modified;

-- name: UpsertConfigEnabled :exec
INSERT INTO system_config (config_key, config_value, config_enabled, last_modified)
VALUES (?, ?, ?, ?)
ON CONFLICT(config_key) DO UPDATE SET
    config_enabled = excluded.config_enabled,
    last_modified = excluded.last_modified;
