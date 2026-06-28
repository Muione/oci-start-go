// Package dns — cache.go: two-level (memory + database) TTL cache for
// Cloudflare API responses. Memory provides sub-millisecond reads; the
// database backing store survives server restarts so the DNS management
// page can show cached data immediately while a background API refresh
// completes.
package dns

import (
	"context"
	"log"
	"sync"
	"time"
)

// cacheEntry holds a cached value with an expiration time.
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func (e cacheEntry[T]) expired() bool { return time.Now().After(e.expiresAt) }

// CfCache wraps a CfClient with a two-level TTL cache (memory + optional DB).
// Safe for concurrent use.
type CfCache struct {
	client *CfClient
	store  *CacheStore // optional persistent backing store

	mu          sync.RWMutex
	zones       *cacheEntry[[]Zone]
	zoneRecords map[string]*cacheEntry[[]DnsRecord] // zoneID → records
	zoneTTL     time.Duration
	recordTTL   time.Duration
}

// NewCfCache creates a cached Cloudflare client wrapper.
func NewCfCache(client *CfClient) *CfCache {
	return &CfCache{
		client:      client,
		zoneRecords: make(map[string]*cacheEntry[[]DnsRecord]),
		zoneTTL:     5 * time.Minute,
		recordTTL:   2 * time.Minute,
	}
}

// SetStore attaches a persistent database backing store. When set, cache
// reads fall through to the DB on memory miss, and writes are persisted.
func (c *CfCache) SetStore(store *CacheStore) {
	c.store = store
	c.PreloadFromDB()
}

// PreloadFromDB populates the in-memory cache from the database backing
// store. Called at startup and when the backing store is attached.
func (c *CfCache) PreloadFromDB() {
	if c.store == nil {
		return
	}
	// Preload zones
	if zones, ok := c.store.LoadZones(); ok && len(zones) > 0 {
		c.mu.Lock()
		c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
		c.mu.Unlock()
	}
}

// ListZones returns cached zones or fetches from API if expired/missing.
// Reads from memory → DB → API, writing back at each level on miss.
func (c *CfCache) ListZones(ctx context.Context) ([]Zone, error) {
	// 1. Memory
	c.mu.RLock()
	if c.zones != nil && !c.zones.expired() {
		val := c.zones.value
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	// 2. DB backing store
	if c.store != nil {
		if zones, ok := c.store.LoadZones(); ok && len(zones) > 0 {
			c.mu.Lock()
			c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
			c.mu.Unlock()
			return zones, nil
		}
	}

	// 3. API
	zones, err := c.client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
	c.mu.Unlock()

	// Persist to DB
	if c.store != nil {
		c.store.SaveZones(zones)
	}
	return zones, nil
}

// InvalidateZones clears the zone list cache (memory + DB).
func (c *CfCache) InvalidateZones() {
	c.mu.Lock()
	c.zones = nil
	c.mu.Unlock()
	if c.store != nil {
		c.store.InvalidateAll()
	}
}

// ListRecords returns cached DNS records for a zone or fetches from API.
func (c *CfCache) ListRecords(ctx context.Context, zoneID, recordType, name string) ([]DnsRecord, error) {
	// 1. Memory
	c.mu.RLock()
	entry, ok := c.zoneRecords[zoneID]
	if ok && entry != nil && !entry.expired() {
		val := entry.value
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	// 2. DB backing store
	if c.store != nil {
		if records, ok := c.store.LoadRecords(zoneID); ok && len(records) > 0 {
			c.mu.Lock()
			c.zoneRecords[zoneID] = &cacheEntry[[]DnsRecord]{value: records, expiresAt: time.Now().Add(c.recordTTL)}
			c.mu.Unlock()
			return records, nil
		}
	}

	// 3. API
	records, err := c.client.ListDnsRecords(ctx, zoneID, recordType, name)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.zoneRecords[zoneID] = &cacheEntry[[]DnsRecord]{value: records, expiresAt: time.Now().Add(c.recordTTL)}
	c.mu.Unlock()

	// Persist to DB
	if c.store != nil {
		c.store.SaveRecords(zoneID, records)
	}
	return records, nil
}

// ListRecordsPage always hits the API — paginated results are too volatile
// to cache usefully.
func (c *CfCache) ListRecordsPage(ctx context.Context, zoneID string, page, perPage int, recordType, name, content string) ([]DnsRecord, *CfListRecordsResponse, error) {
	return c.client.ListDnsRecordsPage(ctx, zoneID, page, perPage, recordType, name, content)
}

// InvalidateZone clears cache for a specific zone (memory + DB).
func (c *CfCache) InvalidateZone(zoneID string) {
	c.mu.Lock()
	delete(c.zoneRecords, zoneID)
	c.zones = nil
	c.mu.Unlock()

	if c.store != nil {
		c.store.InvalidateRecords(zoneID)
	}
}

// CreateRecord creates a DNS record and invalidates the zone cache.
func (c *CfCache) CreateRecord(ctx context.Context, zoneID string, record DnsRecord) (*DnsRecord, error) {
	result, err := c.client.CreateDnsRecord(ctx, zoneID, record)
	if err == nil {
		c.InvalidateZone(zoneID)
	}
	return result, err
}

// UpdateRecord updates a DNS record and invalidates the zone cache.
func (c *CfCache) UpdateRecord(ctx context.Context, zoneID, recordID string, record DnsRecord) (*DnsRecord, error) {
	result, err := c.client.UpdateDnsRecord(ctx, zoneID, recordID, record)
	if err == nil {
		c.InvalidateZone(zoneID)
	}
	return result, err
}

// DeleteRecord deletes a DNS record and invalidates the zone cache.
func (c *CfCache) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	err := c.client.DeleteDnsRecord(ctx, zoneID, recordID)
	if err == nil {
		c.InvalidateZone(zoneID)
	}
	return err
}

// RawClient returns the underlying CfClient for operations that should
// bypass the cache.
func (c *CfCache) RawClient() *CfClient { return c.client }

// ─── Global cache singleton ───────────────────────────────────────────

var (
	globalCache      *CfCache
	globalCacheToken string
	globalCacheStore *CacheStore
	globalCacheMu    sync.Mutex
)

// SetGlobalCacheStore sets the database backing store for the global
// CfCache singleton. Call once at startup (before any DNS requests).
func SetGlobalCacheStore(store *CacheStore) {
	globalCacheMu.Lock()
	defer globalCacheMu.Unlock()
	globalCacheStore = store
	if globalCache != nil {
		globalCache.SetStore(store)
	}
}

// GetOrCreateCache returns a cached Cloudflare client for the given API
// token. If the token matches the currently-cached client, it is returned
// immediately. Otherwise a new client + cache is created. The global DB
// backing store is attached automatically if SetGlobalCacheStore was called.
func GetOrCreateCache(apiToken string) *CfCache {
	globalCacheMu.Lock()
	defer globalCacheMu.Unlock()
	if globalCache != nil && globalCacheToken == apiToken {
		return globalCache
	}
	globalCache = NewCfCache(NewCfClient(apiToken))
	globalCacheToken = apiToken
	if globalCacheStore != nil {
		globalCache.SetStore(globalCacheStore)
		log.Println("[dns] cache store attached, preloaded from DB")
	}
	return globalCache
}
