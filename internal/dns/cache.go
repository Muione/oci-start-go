// Package dns — cache.go: server-side TTL cache for Cloudflare API responses.
// Wraps CfClient to avoid redundant API calls for zone lists and DNS records.
// Mutations (create/update/delete) invalidate the affected zone's cache.
package dns

import (
	"context"
	"sync"
	"time"
)

// cacheEntry holds a cached value with an expiration time.
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

func (e cacheEntry[T]) expired() bool { return time.Now().After(e.expiresAt) }

// CfCache wraps a CfClient with a TTL-based in-memory cache.
// Safe for concurrent use.
type CfCache struct {
	client *CfClient

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

// NewCfCacheWithTTL creates a cached client with custom TTLs.
func NewCfCacheWithTTL(client *CfClient, zoneTTL, recordTTL time.Duration) *CfCache {
	return &CfCache{
		client:      client,
		zoneRecords: make(map[string]*cacheEntry[[]DnsRecord]),
		zoneTTL:     zoneTTL,
		recordTTL:   recordTTL,
	}
}

// ListZones returns cached zones or fetches from API if expired/missing.
func (c *CfCache) ListZones(ctx context.Context) ([]Zone, error) {
	c.mu.RLock()
	if c.zones != nil && !c.zones.expired() {
		val := c.zones.value
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	zones, err := c.client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.zones = &cacheEntry[[]Zone]{value: zones, expiresAt: time.Now().Add(c.zoneTTL)}
	c.mu.Unlock()
	return zones, nil
}

// InvalidateZones clears the zone list cache.
func (c *CfCache) InvalidateZones() {
	c.mu.Lock()
	c.zones = nil
	c.mu.Unlock()
}

// ListRecords returns cached DNS records for a zone or fetches from API.
func (c *CfCache) ListRecords(ctx context.Context, zoneID, recordType, name string) ([]DnsRecord, error) {
	cacheKey := zoneID

	c.mu.RLock()
	entry, ok := c.zoneRecords[cacheKey]
	if ok && entry != nil && !entry.expired() {
		val := entry.value
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	records, err := c.client.ListDnsRecords(ctx, zoneID, recordType, name)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.zoneRecords[cacheKey] = &cacheEntry[[]DnsRecord]{value: records, expiresAt: time.Now().Add(c.recordTTL)}
	c.mu.Unlock()
	return records, nil
}

// ListRecordsPage is like ListRecords but with pagination. Paginated results
// are cached with a shorter TTL since they represent a specific page view.
func (c *CfCache) ListRecordsPage(ctx context.Context, zoneID string, page, perPage int, recordType, name, content string) ([]DnsRecord, *CfListRecordsResponse, error) {
	// For paginated results, always hit the API — pages change too often
	// and caching individual pages adds complexity without much benefit.
	return c.client.ListDnsRecordsPage(ctx, zoneID, page, perPage, recordType, name, content)
}

// InvalidateZone clears the cache for a specific zone (call after mutations).
func (c *CfCache) InvalidateZone(zoneID string) {
	c.mu.Lock()
	delete(c.zoneRecords, zoneID)
	// Also invalidate zone list since a new zone might have been created
	c.zones = nil
	c.mu.Unlock()
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
//
// GetOrCreateCache returns a shared, token-scoped CfCache. When the token
// changes (reconfigured via system settings), the old cache is discarded
// and a new one is created. This allows the cache to persist across HTTP
// requests while reacting to credential changes.

var (
	globalCache      *CfCache
	globalCacheToken string
	globalCacheMu    sync.Mutex
)

// GetOrCreateCache returns a cached Cloudflare client for the given API
// token. If the token matches the currently-cached client, it is returned
// immediately. Otherwise a new client + cache is created.
func GetOrCreateCache(apiToken string) *CfCache {
	globalCacheMu.Lock()
	defer globalCacheMu.Unlock()
	if globalCache != nil && globalCacheToken == apiToken {
		return globalCache
	}
	globalCache = NewCfCache(NewCfClient(apiToken))
	globalCacheToken = apiToken
	return globalCache
}
