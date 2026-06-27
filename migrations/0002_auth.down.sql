DROP INDEX IF EXISTS idx_login_session_username;
DROP TABLE IF EXISTS login_session;
-- NOTE: the login_user.role column is intentionally NOT reversed.
-- SQLite ALTER TABLE DROP COLUMN is limited and the column is harmless.
