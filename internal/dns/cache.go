// Package dns — cache.go: two-level (memory + database) TTL cache for
// Cloudflare API responses. Memory provides sub-millisecond reads; the
// database backing store survives server restarts so the DNS management
// page can show cached data immediately while a background API refresh
// completes.
//
// Cache strategy (stale-while-revalidate):
//   - Serve from memory if fresh.
//   - If memory expired, serve from DB (always considered fresh for read).
//   - Trigger async API refresh when memory is expired; return stale data
//     immediately while the refresh completes in background.
//   - Only invalidate cache on write operations (create/update/delete).
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
// Uses stale-while-revalidate: expired data is still served while a background
// refresh fetches fresh data from the API. Safe for concurrent use.
type CfCache struct {
	client *CfClient
	store  *CacheStore // optional persistent backing store

	mu          sync.RWMutex
	zones       *cacheEntry[[]Zone]
	zoneRecords map[string]*cacheEntry[[]DnsRecord] // zoneID → records
	zoneTTL     time.Duration
	recordTTL   time.Duration

	// Refresh guards prevent concurrent API refreshes for the same key.
	refreshingZones  sync.Mutex
	refreshingRecords map[string]*sync.Mutex
	refMu            sync.Mutex
}

// NewCfCache creates a cached Cloudflare client wrapper.
// TTLs are intentionally long — cache is only invalidated on writes.
func NewCfCache(client *CfClient) *CfCache {
	return &CfCache{
		client:            client,
		zoneRecords:       make(map[string]*cacheEntry[[]DnsRecord]),
		refreshingRecords: make(map[string]*sync.Mutex),
		zoneTTL:           30 * time.Minute,
		recordTTL:         15 * time.Minute,
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

// getOrCreateRefreshMu returns a mutex for the given zone's record refresh guard.
func (c *CfCache) getOrCreateRefreshMu(zoneID string) *sync.Mutex {
	c.refMu.Lock()
	defer c.refMu.Unlock()
	if mu, ok := c.refreshingRecords[zoneID]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	c.refreshingRecords[zoneID] = mu
	return mu
}

// ListZones returns cached zones or fetches from API if missing entirely.
// Uses stale-while-revalidate: if memory is expired but we have DB data,
// return that and refresh in background.
func (c *CfCache) ListZones(ctx context.Context) ([]Zone, error) {
	// 1. Memory — return immediately if fresh
	c.mu.RLock()
	if c.zones != nil && !c.zones.expired() {
		val := c.zones.value
		c.mu.RUnlock()
		return val, nil
	}
	// Check if we have stale memory data to return
	hasStaleMem := c.zones != nil
	var staleZones []Zone
	if hasStaleMem {
		staleZones = c.zones.value
	}
	c.mu.RUnlock()

	// 2. DB backing store — if memory is expired/missing, serve from DB
	if c.store != nil {
		if zones, ok := c.store.LoadZones(); ok && len(zones) > 0 {
			// Update memory cache
			c.mu.Lock()
			c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
			c.mu.Unlock()

			// If memory was stale, trigger background refresh
			if hasStaleMem {
				go c.refreshZonesBg()
			}
			return zones, nil
		}
	}

	// 3. If we have stale memory data, return it and refresh in background
	if hasStaleMem && len(staleZones) > 0 {
		go c.refreshZonesBg()
		return staleZones, nil
	}

	// 4. API — nothing cached anywhere, must fetch
	return c.refreshZones(ctx)
}

// refreshZonesBg performs a background zone list refresh.
func (c *CfCache) refreshZonesBg() {
	if !c.refreshingZones.TryLock() {
		return // already refreshing
	}
	defer c.refreshingZones.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	zones, err := c.client.ListZones(ctx)
	if err != nil {
		log.Printf("[dns] background zone refresh failed: %v", err)
		return
	}

	c.mu.Lock()
	c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
	c.mu.Unlock()

	if c.store != nil {
		c.store.SaveZones(zones)
	}
}

// refreshZones performs a synchronous zone list fetch from the API.
func (c *CfCache) refreshZones(ctx context.Context) ([]Zone, error) {
	zones, err := c.client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
	c.mu.Unlock()

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
// Uses stale-while-revalidate: expired data is still served while background refresh runs.
func (c *CfCache) ListRecords(ctx context.Context, zoneID, recordType, name string) ([]DnsRecord, error) {
	// 1. Memory — return immediately if fresh
	c.mu.RLock()
	entry, ok := c.zoneRecords[zoneID]
	if ok && entry != nil && !entry.expired() {
		val := entry.value
		c.mu.RUnlock()
		return val, nil
	}
	hasStaleMem := ok && entry != nil
	var staleRecords []DnsRecord
	if hasStaleMem {
		staleRecords = entry.value
	}
	c.mu.RUnlock()

	// 2. DB backing store — if memory is expired/missing, serve from DB
	if c.store != nil {
		if records, ok := c.store.LoadRecords(zoneID); ok && len(records) > 0 {
			c.mu.Lock()
			c.zoneRecords[zoneID] = &cacheEntry[[]DnsRecord]{value: records, expiresAt: time.Now().Add(c.recordTTL)}
			c.mu.Unlock()

			if hasStaleMem {
				go c.refreshRecordsBg(zoneID, recordType, name)
			}
			return records, nil
		}
	}

	// 3. If we have stale memory data, return it and refresh in background
	if hasStaleMem && len(staleRecords) > 0 {
		go c.refreshRecordsBg(zoneID, recordType, name)
		return staleRecords, nil
	}

	// 4. API — nothing cached anywhere
	return c.refreshRecords(ctx, zoneID, recordType, name)
}

// refreshRecordsBg performs a background DNS records refresh for a zone.
func (c *CfCache) refreshRecordsBg(zoneID, recordType, name string) {
	mu := c.getOrCreateRefreshMu(zoneID)
	if !mu.TryLock() {
		return // already refreshing for this zone
	}
	defer mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	records, err := c.client.ListDnsRecords(ctx, zoneID, recordType, name)
	if err != nil {
		log.Printf("[dns] background records refresh failed for zone %s: %v", zoneID, err)
		return
	}

	c.mu.Lock()
	c.zoneRecords[zoneID] = &cacheEntry[[]DnsRecord]{value: records, expiresAt: time.Now().Add(c.recordTTL)}
	c.mu.Unlock()

	if c.store != nil {
		c.store.SaveRecords(zoneID, records)
	}
}

// refreshRecords performs a synchronous DNS records fetch from the API.
func (c *CfCache) refreshRecords(ctx context.Context, zoneID, recordType, name string) ([]DnsRecord, error) {
	records, err := c.client.ListDnsRecords(ctx, zoneID, recordType, name)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.zoneRecords[zoneID] = &cacheEntry[[]DnsRecord]{value: records, expiresAt: time.Now().Add(c.recordTTL)}
	c.mu.Unlock()

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
