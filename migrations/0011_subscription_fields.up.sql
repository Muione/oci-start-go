-- Persist OSP Gateway subscription fields not already in register_detail.
-- register_time already holds subscription timeStart; tenant.account_type holds planType.
ALTER TABLE register_detail ADD COLUMN currency_code TEXT;
ALTER TABLE register_detail ADD COLUMN is_intent_to_pay INTEGER;
