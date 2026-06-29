-- SslCertificate queries (Phase 12.1). ssl_certificate stores SSL cert
-- lifecycle data: domain, type, status, paths to PEM files, auto-renew.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: InsertSslCertificate :one
INSERT INTO ssl_certificate (
    domain, certificate_type, email, validation_method, auto_renew,
    certificate_status, issue_date, expire_date, certificate_path,
    private_key_path, create_time, update_time, dns_provider
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateSslCertificate :exec
UPDATE ssl_certificate
SET domain = ?, certificate_type = ?, email = ?, validation_method = ?,
    auto_renew = ?, certificate_status = ?, issue_date = ?, expire_date = ?,
    certificate_path = ?, private_key_path = ?, update_time = ?, dns_provider = ?
WHERE id = ?;

-- name: DeleteSslCertificate :exec
DELETE FROM ssl_certificate WHERE id = ?;

-- name: FindSslCertificateById :one
SELECT * FROM ssl_certificate WHERE id = ?;

-- name: FindSslCertificateByDomain :one
SELECT * FROM ssl_certificate WHERE domain = ? LIMIT 1;

-- name: ListSslCertificates :many
SELECT * FROM ssl_certificate ORDER BY create_time DESC LIMIT ? OFFSET ?;

-- name: CountSslCertificates :one
SELECT COUNT(*) FROM ssl_certificate;

-- name: ListExpiringCertificates :many
SELECT * FROM ssl_certificate WHERE expire_date <= ? AND auto_renew = 1 AND certificate_type = 'LETS_ENCRYPT';

-- name: FindAllActiveSslCertificates :many
SELECT * FROM ssl_certificate WHERE certificate_status = 'VALID';

-- name: ExistsSslCertificateById :one
SELECT COUNT(*) FROM ssl_certificate WHERE id = ?;

-- name: ExistsProxyConfigBySslCertId :one
SELECT COUNT(*) FROM proxy_config WHERE ssl_certificate_id = ?;
