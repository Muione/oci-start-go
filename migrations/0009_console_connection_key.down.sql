DROP INDEX IF EXISTS idx_console_connections_instance;
ALTER TABLE console_connections DROP COLUMN public_key_ssh;
ALTER TABLE console_connections DROP COLUMN encrypted_private_key;
