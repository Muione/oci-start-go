-- Remove the UNIQUE constraint on tenant_id by rebuilding the table without it.
CREATE TABLE IF NOT EXISTS register_detail_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_prv_id BIGINT,
    tenant_id TEXT,
    account_type INTEGER,
    plan_type INTEGER,
    register_time TEXT,
    city TEXT,
    country TEXT,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
    line1 TEXT,
    postal_code TEXT,
    subscription_plan_number TEXT,
    upgrade_state TEXT,
    currency_code TEXT,
    is_intent_to_pay INTEGER,
    created_time TEXT,
    updated_time TEXT,
    cloud_type INTEGER DEFAULT 1
);

INSERT INTO register_detail_old (
    id, tenant_prv_id, tenant_id, account_type, plan_type, register_time,
    city, country, email_address, first_name, last_name, line1, postal_code,
    subscription_plan_number, upgrade_state, currency_code, is_intent_to_pay,
    created_time, updated_time, cloud_type
)
SELECT
    id, tenant_prv_id, tenant_id, account_type, plan_type, register_time,
    city, country, email_address, first_name, last_name, line1, postal_code,
    subscription_plan_number, upgrade_state, currency_code, is_intent_to_pay,
    created_time, updated_time, cloud_type
FROM register_detail;

DROP TABLE register_detail;
ALTER TABLE register_detail_old RENAME TO register_detail;
