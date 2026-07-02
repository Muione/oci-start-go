// Package service — quota.go: Quota service (Phase 11.4).
// Looks up tenant credentials, builds OCI clients via proxy, and delegates
// to oci.GetServiceQuotasPaged for paginated quota data.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/limits"
)

// QuotaService provides paginated OCI resource quota data per tenant.
type QuotaService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool

	mu         sync.Mutex
	svcCache   map[int64]*svcCacheEntry          // tenantID → services-with-quota list
	quotaCache map[quotaCacheKey]*quotaCacheEntry
}

type svcCacheEntry struct {
	services []oci.ServiceInfo
	fetched  time.Time
}

type quotaCacheKey struct {
	tenantID int64
	service  string
}

type quotaCacheEntry struct {
	page    *oci.QuotaPage
	fetched time.Time
}

// NewQuotaService constructs a QuotaService.
func NewQuotaService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *QuotaService {
	return &QuotaService{
		store:      store,
		masterKey:  masterKey,
		pool:       pool,
		svcCache:   make(map[int64]*svcCacheEntry),
		quotaCache: make(map[quotaCacheKey]*quotaCacheEntry),
	}
}

// GetQuota returns paginated quota for a tenant+service.
func (s *QuotaService) GetQuota(ctx context.Context, tenantID int64, serviceName string, page, pageSize int) (*oci.QuotaPage, error) {
	// 2-min cache (page 0 only — cached page is always page 0 of the common case).
	if page == 0 {
		s.mu.Lock()
		if e, ok := s.quotaCache[quotaCacheKey{tenantID, serviceName}]; ok && time.Since(e.fetched) < 2*time.Minute {
			s.mu.Unlock()
			return e.page, nil
		}
		s.mu.Unlock()
	}

	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)
	regionName := region.NameByCode(creds.Region)

	var result *oci.QuotaPage
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		result, qErr = oci.GetServiceQuotasPaged(ctx, clients, creds.Tenancy, serviceName, regionName, page, pageSize)
		return qErr
	})
	if err != nil {
		return nil, err
	}

	if page == 0 {
		s.mu.Lock()
		s.quotaCache[quotaCacheKey{tenantID, serviceName}] = &quotaCacheEntry{page: result, fetched: time.Now()}
		s.mu.Unlock()
	}
	return result, nil
}

// ListServices returns all available OCI limit services for a tenant.
func (s *QuotaService) ListServices(ctx context.Context, tenantID int64) ([]oci.ServiceInfo, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result []oci.ServiceInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		services, qErr := oci.ListLimitServices(ctx, clients, creds.Tenancy)
		if qErr != nil {
			return qErr
		}
		result = make([]oci.ServiceInfo, 0, len(services))
		for _, svc := range services {
			if svc.Name != nil {
				desc := ""
				if svc.Description != nil {
					desc = *svc.Description
				}
				result = append(result, oci.ServiceInfo{
					Name:        *svc.Name,
					Description: desc,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListServicesWithQuota returns only services that have at least one non-zero
// limit value for the tenant. Probes each service concurrently (semaphore-limited
// to 5) with a single ListLimitValues call. Cached for 5 minutes per tenant.
func (s *QuotaService) ListServicesWithQuota(ctx context.Context, tenantID int64) ([]oci.ServiceInfo, error) {
	// 5-min cache.
	s.mu.Lock()
	if e, ok := s.svcCache[tenantID]; ok && time.Since(e.fetched) < 5*time.Minute {
		s.mu.Unlock()
		return e.services, nil
	}
	s.mu.Unlock()

	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var allServices []limits.ServiceSummary
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		allServices, qErr = oci.ListLimitServices(ctx, clients, creds.Tenancy)
		return qErr
	})
	if err != nil {
		return nil, err
	}

	// Concurrent probe (semaphore = 5). stdlib only.
	type probeResult struct {
		idx int
		has bool
		ok  bool
	}
	results := make([]probeResult, len(allServices))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for i, svc := range allServices {
		if svc.Name == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, name string) {
			defer wg.Done()
			defer func() { <-sem }()
			var has bool
			pErr := oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
				var qErr error
				has, qErr = oci.ServiceHasLimits(ctx, clients, creds.Tenancy, name)
				return qErr
			})
			results[i] = probeResult{idx: i, has: has, ok: pErr == nil && has}
		}(i, *svc.Name)
	}
	wg.Wait()

	out := make([]oci.ServiceInfo, 0, len(allServices))
	for i, svc := range allServices {
		if svc.Name == nil {
			continue
		}
		if i < len(results) && results[i].ok {
			desc := ""
			if svc.Description != nil {
				desc = *svc.Description
			}
			out = append(out, oci.ServiceInfo{Name: *svc.Name, Description: desc})
		}
	}

	s.mu.Lock()
	s.svcCache[tenantID] = &svcCacheEntry{services: out, fetched: time.Now()}
	s.mu.Unlock()
	return out, nil
}