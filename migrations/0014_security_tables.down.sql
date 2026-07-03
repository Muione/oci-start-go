DROP INDEX IF EXISTS idx_notification_history_created_at;
DROP INDEX IF EXISTS idx_notification_history_channel;
DROP TABLE IF EXISTS notification_history;

DROP INDEX IF EXISTS idx_login_history_success;
DROP INDEX IF EXISTS idx_login_history_created_at;
DROP INDEX IF EXISTS idx_login_history_username;
DROP TABLE IF EXISTS login_history;
