-- Extended tenant email config queries (Phase 12.2). OCI provisioning fields,
-- daily send tracking, and admin list endpoint.
-- ALL COMMENTS MUST BE ASCII-ONLY.

-- name: FindTenantEmailConfigById :one
SELECT id, tenant_id, domain_id, domain_name, sender_id, credential_id,
       smtp_username, smtp_password, smtp_host, smtp_port, sender_email,
       dkim_id, cname_record_value, active, created_time, daily_email_limit,
       today_sent_count, last_reset_date, dbs_record_ids_str
FROM tenant_email_config WHERE id = ?;

-- name: UpsertTenantEmailConfigFull :exec
INSERT INTO tenant_email_config (
    tenant_id, domain_id, domain_name, sender_id, credential_id,
    smtp_username, smtp_password, smtp_host, smtp_port, sender_email,
    dkim_id, cname_record_value, active, created_time, daily_email_limit,
    today_sent_count, last_reset_date, dbs_record_ids_str
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tenant_id) DO UPDATE SET
    domain_id = excluded.domain_id,
    domain_name = excluded.domain_name,
    sender_id = excluded.sender_id,
    credential_id = excluded.credential_id,
    smtp_username = excluded.smtp_username,
    smtp_password = excluded.smtp_password,
    smtp_host = excluded.smtp_host,
    smtp_port = excluded.smtp_port,
    sender_email = excluded.sender_email,
    dkim_id = excluded.dkim_id,
    cname_record_value = excluded.cname_record_value,
    active = excluded.active,
    created_time = excluded.created_time,
    daily_email_limit = excluded.daily_email_limit,
    today_sent_count = excluded.today_sent_count,
    last_reset_date = excluded.last_reset_date,
    dbs_record_ids_str = excluded.dbs_record_ids_str;

-- name: UpdateTenantEmailSentCount :exec
UPDATE tenant_email_config SET today_sent_count = ?, last_reset_date = ?
WHERE id = ?;

-- name: ListTenantEmailConfigs :many
SELECT tec.id, tec.tenant_id, tec.domain_name, tec.smtp_username,
       tec.smtp_host, tec.smtp_port, tec.sender_email, tec.active,
       tec.created_time, tec.daily_email_limit, tec.today_sent_count,
       t.user_name AS tenant_name
FROM tenant_email_config tec
LEFT JOIN tenant t ON tec.tenant_id = t.id
WHERE tec.active = 1
ORDER BY tec.id DESC
LIMIT ? OFFSET ?;

-- name: CountTenantEmailConfigs :one
SELECT COUNT(*) FROM tenant_email_config WHERE active = 1;
