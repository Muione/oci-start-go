// Package service — quota.go: Quota service (Phase 11.4).
// Looks up tenant credentials, builds OCI clients via proxy, and delegates
// to oci.GetServiceQuotasPaged for paginated quota data.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/repo"
)

// QuotaService provides paginated OCI resource quota data per tenant.
type QuotaService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewQuotaService constructs a QuotaService.
func NewQuotaService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *QuotaService {
	return &QuotaService{store: store, masterKey: masterKey, pool: pool}
}

// GetQuota returns paginated quota for a tenant+service.
func (s *QuotaService) GetQuota(ctx context.Context, tenantID int64, serviceName string, page, pageSize int) (*oci.QuotaPage, error) {
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
	return result, nil
}
