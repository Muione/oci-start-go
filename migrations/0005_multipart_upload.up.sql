-- Multipart upload tracking table (parity with Java oci_multipart_upload_record)
CREATE TABLE IF NOT EXISTS oci_multipart_upload_record (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id       INTEGER NOT NULL,
    cloud_type      INTEGER DEFAULT 1,          -- 1=OCI, 2=GCP, 3=Azure, 4=AWS
    tenancy_ocid    TEXT,                        -- redundant: survives tenant delete+reimport
    namespace       TEXT,
    bucket_name     TEXT NOT NULL,
    object_name     TEXT NOT NULL,
    upload_id       TEXT NOT NULL,
    total_size      INTEGER,
    chunk_size      INTEGER,
    total_parts     INTEGER,
    completed_parts TEXT DEFAULT '[]',           -- JSON: [{"partNum":1,"etag":"abc"},...]
    status          TEXT DEFAULT 'uploading',    -- uploading | completed | aborted
    create_time     TEXT,
    update_time     TEXT
);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_tenant_bucket
    ON oci_multipart_upload_record(tenant_id, bucket_name, status);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_upload_id
    ON oci_multipart_upload_record(upload_id);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_stale
    ON oci_multipart_upload_record(status, update_time);
