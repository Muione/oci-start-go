-- Tenant email config queries (Phase 9). tenant_email_config stores SES/SMTP
-- configuration per tenant for email service.

-- name: FindTenantEmailConfigByTenantId :one
SELECT id, tenant_id, domain_id, domain_name, sender_id, credential_id,
       smtp_username, smtp_password, smtp_host, smtp_port, sender_email,
       dkim_id, cname_record_value, active, created_time, daily_email_limit,
       today_sent_count, last_reset_date, dbs_record_ids_str
FROM tenant_email_config WHERE tenant_id = ?;

-- name: UpsertTenantEmailConfig :exec
INSERT INTO tenant_email_config (
    tenant_id, domain_name, smtp_username, smtp_password, smtp_host, smtp_port,
    sender_email, active, created_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tenant_id) DO UPDATE SET
    domain_name = excluded.domain_name,
    smtp_username = excluded.smtp_username,
    smtp_password = excluded.smtp_password,
    smtp_host = excluded.smtp_host,
    smtp_port = excluded.smtp_port,
    sender_email = excluded.sender_email,
    active = excluded.active;

-- name: UpdateTenantEmailActive :exec
UPDATE tenant_email_config SET active = ? WHERE tenant_id = ?;

-- name: DeleteTenantEmailConfig :exec
DELETE FROM tenant_email_config WHERE tenant_id = ?;
