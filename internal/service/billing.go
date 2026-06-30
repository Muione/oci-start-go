// Package service — billing.go: Billing service (Phase B).
// Provides OSP Gateway subscription details and Usage/Cost API queries per tenant.
// Looks up tenant credentials, builds OCI clients via proxy, and delegates
// to oci osp_gateway and usage wrappers.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// BillingService provides billing-related OCI operations per tenant.
type BillingService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewBillingService constructs a BillingService.
func NewBillingService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *BillingService {
	return &BillingService{store: store, masterKey: masterKey, pool: pool}
}

// GetSubscriptionDetail returns OSP Gateway subscription info for a tenant.
func (s *BillingService) GetSubscriptionDetail(ctx context.Context, tenantID int64) (*oci.SubscriptionInfo, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result *oci.SubscriptionInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		result, qErr = oci.GetSubscriptionInfo(ctx, clients, creds.Tenancy)
		return qErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// QueryCost queries cost data for a tenant. queryType can be:
// - "yesterday": yesterday's daily cost
// - "today": today's daily cost
// - "current_month": current month's cost
// - "last_month": last month's cost
// - "custom": custom date range (requires startDate and endDate in "2006-01-02" format)
func (s *BillingService) QueryCost(ctx context.Context, tenantID int64, queryType, startDate, endDate string) ([]oci.CostSummary, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result []oci.CostSummary
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		switch queryType {
		case "yesterday":
			result, qErr = oci.QueryYesterdayCost(ctx, clients, creds.Tenancy)
		case "today":
			result, qErr = oci.QueryTodayCost(ctx, clients, creds.Tenancy)
		case "current_month":
			result, qErr = oci.QueryCurrentMonthCost(ctx, clients, creds.Tenancy)
		case "last_month":
			result, qErr = oci.QueryLastMonthCost(ctx, clients, creds.Tenancy)
		case "custom":
			result, qErr = oci.QueryCustomCost(ctx, clients, creds.Tenancy, startDate, endDate)
		default:
			result, qErr = oci.QueryCurrentMonthCost(ctx, clients, creds.Tenancy)
		}
		return qErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
