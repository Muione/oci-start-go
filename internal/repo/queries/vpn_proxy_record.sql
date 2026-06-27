-- VpnProxyRecord queries (Phase 3 SOCKS proxy pool). Random available pick
-- parity with VpnProxyRecordRepository.findRandomAvailableRecord.

-- name: FindRandomAvailableProxy :one
SELECT * FROM vpn_proxy_record
WHERE available_status = 1
ORDER BY RANDOM()
LIMIT 1;

-- name: ListVpnProxyRecords :many
SELECT * FROM vpn_proxy_record ORDER BY id;

-- name: InsertVpnProxyRecord :exec
INSERT INTO vpn_proxy_record (
    proxy_type, proxy_host, proxy_port, proxy_username, proxy_password,
    available_status, update_time, create_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateVpnProxyRecord :exec
UPDATE vpn_proxy_record
SET proxy_type = ?, proxy_host = ?, proxy_port = ?, proxy_username = ?,
    proxy_password = ?, available_status = ?, update_time = ?
WHERE id = ?;

-- name: DeleteVpnProxyRecord :exec
DELETE FROM vpn_proxy_record WHERE id = ?;
