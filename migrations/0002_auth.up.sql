-- oci-start Go rewrite: migration 0002 — auth (login_session table + login_user.role)
-- See SPEC §7.1 (role column reserved for flat-USER model + future RBAC).

CREATE TABLE login_session (
    token          TEXT PRIMARY KEY,
    username       TEXT NOT NULL,
    ip             TEXT,
    user_agent     TEXT,
    created_at     TEXT NOT NULL,
    expires_at     TEXT NOT NULL,
    last_active_at TEXT NOT NULL
);
CREATE INDEX idx_login_session_username ON login_session(username);

-- Reserve the flat USER role (SPEC §7.1). NOT NULL DEFAULT keeps existing rows 'USER'.
ALTER TABLE login_user ADD COLUMN role TEXT NOT NULL DEFAULT 'USER';
