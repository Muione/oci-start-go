-- migration 0004: API response cache for DNS providers.
-- Persists Cloudflare zone list and per-zone DNS records so the
-- DNS management page can serve cached data immediately after restart
-- while refreshing from the provider API in the background.

CREATE TABLE IF NOT EXISTS api_cache (
    cache_key   TEXT PRIMARY KEY,
    cache_value TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_cache_expires ON api_cache(expires_at);
