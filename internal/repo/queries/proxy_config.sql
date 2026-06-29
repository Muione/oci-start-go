-- ProxyConfig queries (Phase 12.1). proxy_config stores per-domain reverse
-- proxy rules with target host/port, SSL, WebSocket, rate limiting, caching,
-- and custom config blocks. ALL COMMENTS MUST BE ASCII-ONLY.

-- name: InsertProxyConfig :exec
INSERT INTO proxy_config (
    domain, target_host, target_port, protocol, enable_ssl, enable_websocket,
    ssl_certificate_id, config_status, ssl_status, custom_config, remark,
    load_balance_type, enable_health_check, health_check_path, health_check_interval,
    enable_rate_limit, rate_limit, enable_cache, cache_time, create_time, update_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateProxyConfig :exec
UPDATE proxy_config
SET domain = ?, target_host = ?, target_port = ?, protocol = ?, enable_ssl = ?,
    enable_websocket = ?, ssl_certificate_id = ?, config_status = ?, ssl_status = ?,
    custom_config = ?, remark = ?, load_balance_type = ?, enable_health_check = ?,
    health_check_path = ?, health_check_interval = ?, enable_rate_limit = ?,
    rate_limit = ?, enable_cache = ?, cache_time = ?, update_time = ?
WHERE id = ?;

-- name: DeleteProxyConfig :exec
DELETE FROM proxy_config WHERE id = ?;

-- name: FindProxyConfigById :one
SELECT * FROM proxy_config WHERE id = ?;

-- name: ListProxyConfigs :many
SELECT * FROM proxy_config ORDER BY create_time DESC LIMIT ? OFFSET ?;

-- name: CountProxyConfigs :one
SELECT COUNT(*) FROM proxy_config;

-- name: ListActiveProxyConfigs :many
SELECT * FROM proxy_config WHERE config_status != 'DISABLED' ORDER BY create_time DESC;

-- name: ExistsProxyConfigByDomain :one
SELECT COUNT(*) FROM proxy_config WHERE domain = ?;

-- name: UpdateProxyConfigStatus :exec
UPDATE proxy_config SET config_status = ?, update_time = ? WHERE id = ?;

-- name: UpdateProxyConfigSslFields :exec
UPDATE proxy_config SET ssl_certificate_id = ?, enable_ssl = ?, ssl_status = ?, update_time = ? WHERE id = ?;

-- name: FindProxyConfigsBySslCertId :many
SELECT * FROM proxy_config WHERE ssl_certificate_id = ?;
