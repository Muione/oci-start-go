// Package dns — store.go: database-backed persistent cache for DNS API
// responses. Survives server restarts so the DNS management page can
// show cached data immediately while refreshing from the provider.
package dns

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheStore persists DNS API responses to SQLite.
// Safe for concurrent use.
type CacheStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewCacheStore creates a new CacheStore backed by the given DB.
func NewCacheStore(db *sql.DB) *CacheStore {
	return &CacheStore{db: db}
}

// ─── Zone cache ────────────────────────────────────────────────────────

// LoadZones returns cached zones from the DB, or nil if expired/missing.
func (s *CacheStore) LoadZones() ([]Zone, bool) {
	return loadFromDB[[]Zone](s, "dns.zones", 5*time.Minute)
}

// SaveZones writes zones to the DB cache.
func (s *CacheStore) SaveZones(zones []Zone) {
	saveToDB(s, "dns.zones", zones, 5*time.Minute)
}

// ─── Records cache ─────────────────────────────────────────────────────

// LoadRecords returns cached DNS records for a zone from the DB,
// or nil if expired/missing.
func (s *CacheStore) LoadRecords(zoneID string) ([]DnsRecord, bool) {
	return loadFromDB[[]DnsRecord](s, "dns.records."+zoneID, 2*time.Minute)
}

// SaveRecords writes DNS records for a zone to the DB cache.
func (s *CacheStore) SaveRecords(zoneID string, records []DnsRecord) {
	saveToDB(s, "dns.records."+zoneID, records, 2*time.Minute)
}

// InvalidateRecords removes cached records for a zone.
func (s *CacheStore) InvalidateRecords(zoneID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`DELETE FROM api_cache WHERE cache_key = ?`, "dns.records."+zoneID)
}

// InvalidateAll clears all DNS cache entries.
func (s *CacheStore) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`DELETE FROM api_cache WHERE cache_key LIKE 'dns.%'`)
}

// ─── Generic helpers ───────────────────────────────────────────────────

func loadFromDB[T any](s *CacheStore, key string, ttl time.Duration) (T, bool) {
	var zero T
	s.mu.Lock()
	defer s.mu.Unlock()

	var value string
	var expiresAt string
	err := s.db.QueryRow(
		`SELECT cache_value, expires_at FROM api_cache WHERE cache_key = ?`, key,
	).Scan(&value, &expiresAt)
	if err != nil {
		return zero, false
	}

	exp, parseErr := time.Parse(time.RFC3339, expiresAt)
	if parseErr != nil || time.Now().After(exp) {
		// Expired — clean up
		_, _ = s.db.Exec(`DELETE FROM api_cache WHERE cache_key = ?`, key)
		return zero, false
	}

	var result T
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return zero, false
	}
	return result, true
}

func saveToDB[T any](s *CacheStore, key string, value T, ttl time.Duration) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(ttl)

	_, _ = s.db.Exec(
		`INSERT OR REPLACE INTO api_cache (cache_key, cache_value, expires_at, updated_at)
		 VALUES (?, ?, ?, ?)`,
		key, string(data), expiresAt.Format(time.RFC3339), now.Format(time.RFC3339),
	)
}

// EnsureTable creates the api_cache table if it doesn't exist.
// Safe to call at startup — uses IF NOT EXISTS.
func (s *CacheStore) EnsureTable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS api_cache (
			cache_key   TEXT PRIMARY KEY,
			cache_value TEXT NOT NULL,
			expires_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure api_cache table: %w", err)
	}
	return nil
}
