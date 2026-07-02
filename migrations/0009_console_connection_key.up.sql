ALTER TABLE console_connections ADD COLUMN encrypted_private_key TEXT;
ALTER TABLE console_connections ADD COLUMN public_key_ssh TEXT;

-- One row per instance (the latest app-created connection) so upsert can use
-- ON CONFLICT(instance_id). The table was unused before this migration, so no
-- duplicate-instance_id rows exist to block the index.
CREATE UNIQUE INDEX IF NOT EXISTS idx_console_connections_instance
    ON console_connections(instance_id);
