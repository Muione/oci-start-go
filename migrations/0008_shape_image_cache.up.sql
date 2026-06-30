-- Shape and Image cache tables for Phase 2 instance management.
-- Caches OCI Shape and Image data per tenant to avoid repeated API calls.

CREATE TABLE shape_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT NOT NULL,
    compartment_id TEXT NOT NULL,
    availability_domain TEXT,
    shape TEXT NOT NULL,
    architecture TEXT NOT NULL DEFAULT 'AMD',
    ocpus REAL,
    memory_in_gbs REAL,
    processor_description TEXT,
    is_flexible INTEGER NOT NULL DEFAULT 0,
    max_vnic_attachments INTEGER,
    gpu_description TEXT,
    gpu_count INTEGER,
    local_disk_description TEXT,
    networking_description TEXT,
    baseline_ocpu REAL,
    last_synced_at TEXT NOT NULL,
    cloud_type INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_shape_cache_tenant ON shape_cache(tenant_id);
CREATE INDEX idx_shape_cache_arch ON shape_cache(tenant_id, architecture);

CREATE TABLE image_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT NOT NULL,
    compartment_id TEXT NOT NULL,
    image_id TEXT NOT NULL,
    display_name TEXT,
    operating_system TEXT,
    operating_system_version TEXT,
    architecture TEXT NOT NULL DEFAULT 'AMD',
    size_in_gbs BIGINT,
    launch_mode TEXT,
    time_created TEXT,
    last_synced_at TEXT NOT NULL,
    cloud_type INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_image_cache_tenant ON image_cache(tenant_id);
CREATE INDEX idx_image_cache_arch ON image_cache(tenant_id, architecture);
CREATE INDEX idx_image_cache_os ON image_cache(tenant_id, operating_system);
