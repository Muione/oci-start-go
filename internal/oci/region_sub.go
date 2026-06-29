// Package oci — region_sub.go: OCI Region Subscription SDK wrapper (Phase 11.4).
// Parity with Java OciRegionSubscriptionUtils: list subscribed/all/unsubscribed
// regions, subscribe to a region, check subscription status.
package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// RegionSubInfo represents a subscribed region.
type RegionSubInfo struct {
	RegionKey    string `json:"regionKey"`
	RegionName   string `json:"regionName"`
	Status       string `json:"status"`
	IsHomeRegion bool   `json:"isHomeRegion"`
}

// RegionInfo represents an unsubscribed region.
type RegionInfo struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	CnName string `json:"cnName"`
}

// RegionSummary holds counts for the summary endpoint.
type RegionSummary struct {
	TotalRegions       int `json:"totalRegions"`
	SubscribedRegions   int `json:"subscribedRegions"`
	UnsubscribedRegions int `json:"unsubscribedRegions"`
}

// RegionSubscribeResult is the per-region result in a batch subscribe.
type RegionSubscribeResult struct {
	RegionKey string `json:"regionKey"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
}

// RegionSubscribeResponse is the batch subscribe response.
type RegionSubscribeResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Details []RegionSubscribeResult `json:"details"`
}

// ListSubscribedRegions returns all regions the tenancy is subscribed to.
// Parity with OciRegionSubscriptionUtils.getSubscribedRegions.
func ListSubscribedRegions(ctx context.Context, c Clients, tenancyOCID string) ([]RegionSubInfo, error) {
	resp, err := c.Identity.ListRegionSubscriptions(ctx, identity.ListRegionSubscriptionsRequest{
		TenancyId: common.String(tenancyOCID),
	})
	if err != nil {
		return nil, fmt.Errorf("list region subscriptions: %w", err)
	}
	out := make([]RegionSubInfo, 0, len(resp.Items))
	for _, r := range resp.Items {
		info := RegionSubInfo{}
		if r.RegionKey != nil {
			info.RegionKey = *r.RegionKey
		}
		if r.RegionName != nil {
			info.RegionName = *r.RegionName
		}
		info.Status = string(r.Status)
		if r.IsHomeRegion != nil {
			info.IsHomeRegion = *r.IsHomeRegion
		}
		out = append(out, info)
	}
	return out, nil
}

// ListAllRegions returns all OCI regions available for subscription.
// Parity with OciRegionSubscriptionUtils.getAllAvailableRegions.
func ListAllRegions(ctx context.Context, c Clients) ([]RegionInfo, error) {
	resp, err := c.Identity.ListRegions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	out := make([]RegionInfo, 0, len(resp.Items))
	for _, r := range resp.Items {
		info := RegionInfo{}
		if r.Key != nil {
			info.Key = *r.Key
		}
		if r.Name != nil {
			info.Name = *r.Name
		}
		info.CnName = region.NameByCode(info.Key)
		out = append(out, info)
	}
	return out, nil
}

// ListUnsubscribedRegions returns regions not yet subscribed.
// Parity with OciRegionSubscriptionUtils.getUnsubscribedRegions.
func ListUnsubscribedRegions(ctx context.Context, c Clients, tenancyOCID string) ([]RegionInfo, error) {
	allRegions, err := ListAllRegions(ctx, c)
	if err != nil {
		return nil, err
	}
	subs, err := ListSubscribedRegions(ctx, c, tenancyOCID)
	if err != nil {
		return nil, err
	}
	subscribed := make(map[string]bool, len(subs))
	for _, s := range subs {
		subscribed[s.RegionKey] = true
	}
	out := make([]RegionInfo, 0)
	for _, r := range allRegions {
		if !subscribed[r.Key] {
			out = append(out, r)
		}
	}
	return out, nil
}

// GetRegionSummary returns total/subscribed/unsubscribed counts.
func GetRegionSummary(ctx context.Context, c Clients, tenancyOCID string) (*RegionSummary, error) {
	allRegions, err := ListAllRegions(ctx, c)
	if err != nil {
		return nil, err
	}
	subs, err := ListSubscribedRegions(ctx, c, tenancyOCID)
	if err != nil {
		return nil, err
	}
	total := len(allRegions)
	subscribed := len(subs)
	return &RegionSummary{
		TotalRegions:       total,
		SubscribedRegions:   subscribed,
		UnsubscribedRegions: total - subscribed,
	}, nil
}

// SubscribeToRegion subscribes the tenancy to a single region.
// Does NOT wait for activation (v1 — frontend polls status).
// Returns success/failure status.
func SubscribeToRegion(ctx context.Context, c Clients, tenancyOCID, regionKey string) (bool, string, error) {
	// Check if already subscribed.
	subs, err := ListSubscribedRegions(ctx, c, tenancyOCID)
	if err != nil {
		return false, "", fmt.Errorf("check subscriptions: %w", err)
	}
	for _, s := range subs {
		if s.RegionKey == regionKey {
			return true, "Already subscribed", nil
		}
	}

	// Validate region key exists.
	allRegions, err := ListAllRegions(ctx, c)
	if err != nil {
		return false, "", fmt.Errorf("list regions: %w", err)
	}
	found := false
	for _, r := range allRegions {
		if r.Key == regionKey {
			found = true
			break
		}
	}
	if !found {
		return false, "Region does not exist", nil
	}

	// Create subscription.
	_, err = c.Identity.CreateRegionSubscription(ctx, identity.CreateRegionSubscriptionRequest{
		TenancyId: common.String(tenancyOCID),
		CreateRegionSubscriptionDetails: identity.CreateRegionSubscriptionDetails{
			RegionKey: common.String(regionKey),
		},
	})
	if err != nil {
		return false, fmt.Sprintf("Subscription failed: %v", err), nil
	}
	return true, "Region subscribed successfully", nil
}

// GetRegionSubscriptionStatus returns "READY", "NOT_SUBSCRIBED", etc.
func GetRegionSubscriptionStatus(ctx context.Context, c Clients, tenancyOCID, regionKey string) (string, error) {
	subs, err := ListSubscribedRegions(ctx, c, tenancyOCID)
	if err != nil {
		return "", fmt.Errorf("list subscriptions: %w", err)
	}
	for _, s := range subs {
		if s.RegionKey == regionKey {
			return s.Status, nil
		}
	}
	return "NOT_SUBSCRIBED", nil
}

// WaitRegionActivation polls ListRegionSubscriptions every 30s until the
// region's status becomes "READY" or "FAILED", or timeout (maxWaitMinutes).
// Optional — v1 can skip the blocking wait.
func WaitRegionActivation(ctx context.Context, c Clients, tenancyOCID, regionKey string, maxWaitMinutes int) (bool, string, error) {
	if maxWaitMinutes <= 0 {
		maxWaitMinutes = 30
	}
	maxAttempts := maxWaitMinutes * 2 // poll every 30s

	for i := 0; i < maxAttempts; i++ {
		status, err := GetRegionSubscriptionStatus(ctx, c, tenancyOCID, regionKey)
		if err != nil {
			return false, "", err
		}
		switch status {
		case "READY":
			return true, "Region activated successfully", nil
		case "FAILED":
			return false, "Region subscription failed", nil
		}
		time.Sleep(30 * time.Second)
	}
	return false, "Timeout waiting for activation", nil
}
