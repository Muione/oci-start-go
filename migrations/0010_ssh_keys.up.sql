CREATE TABLE ssh_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT NOT NULL,
    encrypted_key TEXT NOT NULL,
    encrypted_passphrase TEXT,
    fingerprint TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
