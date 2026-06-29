// Package service — region_sub.go: Region Subscription service (Phase 11.4).
// Looks up tenant credentials, builds OCI clients via proxy, and delegates
// to oci region subscription wrappers.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// RegionSubService provides region subscription management per tenant.
type RegionSubService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
}

// NewRegionSubService constructs a RegionSubService.
func NewRegionSubService(store *db.Store, masterKey []byte, pool *oci.ProxyPool) *RegionSubService {
	return &RegionSubService{store: store, masterKey: masterKey, pool: pool}
}

// Summary returns region counts for a tenant.
func (s *RegionSubService) Summary(ctx context.Context, tenantID int64) (*oci.RegionSummary, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result *oci.RegionSummary
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		result, qErr = oci.GetRegionSummary(ctx, clients, creds.Tenancy)
		return qErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Subscribed returns subscribed regions for a tenant.
func (s *RegionSubService) Subscribed(ctx context.Context, tenantID int64) ([]oci.RegionSubInfo, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result []oci.RegionSubInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		result, qErr = oci.ListSubscribedRegions(ctx, clients, creds.Tenancy)
		return qErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Unsubscribed returns unsubscribed regions for a tenant.
func (s *RegionSubService) Unsubscribed(ctx context.Context, tenantID int64) ([]oci.RegionInfo, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result []oci.RegionInfo
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		var qErr error
		result, qErr = oci.ListUnsubscribedRegions(ctx, clients, creds.Tenancy)
		return qErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Subscribe subscribes a tenant to one or more regions (batch).
// v1: calls CreateRegionSubscription and returns immediately (no blocking wait).
func (s *RegionSubService) Subscribe(ctx context.Context, tenantID int64, regionKeys []string) (*oci.RegionSubscribeResponse, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var response *oci.RegionSubscribeResponse
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		details := make([]oci.RegionSubscribeResult, 0, len(regionKeys))
		allSuccess := true
		for i, key := range regionKeys {
			if i > 0 {
				time.Sleep(1 * time.Second) // rate-limit between requests
			}
			success, message, subErr := oci.SubscribeToRegion(ctx, clients, creds.Tenancy, key)
			if subErr != nil {
				return subErr
			}
			if !success {
				allSuccess = false
			}
			details = append(details, oci.RegionSubscribeResult{
				RegionKey: key,
				Success:   success,
				Message:   message,
			})
		}
		msg := "All regions subscribed successfully"
		if !allSuccess {
			msg = "Some regions failed to subscribe"
		}
		response = &oci.RegionSubscribeResponse{
			Success: allSuccess,
			Message: msg,
			Details: details,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// SubscriptionStatus returns the subscription status for a single region.
func (s *RegionSubService) SubscriptionStatus(ctx context.Context, tenantID int64, regionKey string) (map[string]interface{}, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %w", err)
	}
	creds := tenantToCreds(t)

	var result map[string]interface{}
	err = oci.WithProxy(ctx, s.pool, creds, s.masterKey, func(clients oci.Clients) error {
		status, sErr := oci.GetRegionSubscriptionStatus(ctx, clients, creds.Tenancy, regionKey)
		if sErr != nil {
			return sErr
		}
		result = map[string]interface{}{
			"regionKey":  regionKey,
			"status":     status,
			"subscribed": status != "NOT_SUBSCRIBED",
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
