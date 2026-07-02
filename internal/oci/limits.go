// Package oci — limits.go: OCI Limits/Quota SDK wrapper (Phase 11.4).
// Parity with Java OciLimitsUtils: two-pass approach to collect limit names
// then get resource availability per limit, with AD-level aggregation.
package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/limits"
	"github.com/rs/zerolog/log"
)

const (
	ARMCoreFreeQuotaName = "standard-a1-core-count"
	ARMFreeQuotaName     = "standard-a1-memory-count"
	AMDCoreFreeQuotaName = "standard-e2-micro-core-count"
	AMDVMFreeCountName   = "vm-standard-e2-1-micro-count"

	bytesToGB = 1073741824
)

// QuotaItem represents a single resource quota entry.
type QuotaItem struct {
	Name      string `json:"name"`
	Total     int64  `json:"total"`
	Used      int64  `json:"used"`
	Available int64  `json:"available"`
}

// QuotaPage is a paginated response of quota items.
type QuotaPage struct {
	Items      []QuotaItem `json:"items"`
	Region     string      `json:"region"`
	RegionEn   string      `json:"regionEn"`
	Service    string      `json:"service"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	HasNextPage bool       `json:"hasNextPage"`
}

// ServiceInfo is a simplified service summary for the API.
type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// limitNameWithAD tracks whether a limit is AD-level and the ADs it appeared in.
type limitNameWithAD struct {
	name      string
	isADLevel bool
	adNames   map[string]bool
}

// GetServiceQuotasPaged returns paginated quota data for a single service.
// Two-pass approach (matches Java OciLimitsUtils.getSingleServiceQuotasPaged):
//   Pass 1: ListLimitValues (no AD filter) to collect unique non-zero limit names.
//   Pass 2: GetResourceAvailability per limit name for the current page slice.
// AD-level limits are aggregated across all ADs; regional limits queried directly.
// Values ending in "-bytes" are converted to GB (div 1073741824) and renamed "-gb".
func GetServiceQuotasPaged(
	ctx context.Context,
	c Clients,
	compartmentID, serviceName string,
	regionName string,
	page, pageSize int,
) (*QuotaPage, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	// Pass 1: collect unique limit names with AD info.
	needed := (page+1)*pageSize + 1 // +1 to determine hasNextPage
	allLimits := collectLimitNames(ctx, c, compartmentID, serviceName, needed)

	hasNextPage := len(allLimits) > (page+1)*pageSize

	// Slice for current page.
	from := page * pageSize
	to := from + pageSize
	if from > len(allLimits) {
		from = len(allLimits)
	}
	if to > len(allLimits) {
		to = len(allLimits)
	}
	pageLimits := allLimits[from:to]

	// Pass 2: get availability for each limit in the page.
	items := make([]QuotaItem, 0, len(pageLimits))
	for _, lim := range pageLimits {
		name := lim.name
		var total, used, available int64
		var err error

		if lim.isADLevel && len(lim.adNames) > 0 {
			// Aggregate across all ADs.
			total, used, available, err = getAggregatedAvailability(ctx, c, compartmentID, serviceName, name)
		} else {
			// Regional limit — query directly.
			u, a, e := getResourceAvailability(ctx, c, compartmentID, serviceName, name, nil)
			err = e
			used = u
			available = a
			total = used + available
		}
		if err != nil {
			log.Warn().Err(err).Str("limitName", name).Msg("get resource availability failed, treating as 0")
			// Treat as zero on error.
			items = append(items, QuotaItem{Name: name, Total: 0, Used: 0, Available: 0})
			continue
		}

		// Unit conversion: -bytes → -gb.
		if strings.HasSuffix(name, "-bytes") {
			name = strings.TrimSuffix(name, "-bytes") + "-gb"
			total /= bytesToGB
			used /= bytesToGB
			available /= bytesToGB
		}

		items = append(items, QuotaItem{
			Name:      name,
			Total:     total,
			Used:      used,
			Available: available,
		})
	}

	return &QuotaPage{
		Items:      items,
		Region:     regionName,
		RegionEn:   compartmentID, // compartmentID is the tenancy OCID; region code from provider
		Service:    serviceName,
		Page:       page,
		PageSize:   pageSize,
		HasNextPage: hasNextPage,
	}, nil
}

// collectLimitNames paginates through ListLimitValues and collects unique
// non-zero limit names up to maxCount. Returns limitNameWithAD entries
// preserving insertion order.
func collectLimitNames(ctx context.Context, c Clients, compartmentID, serviceName string, maxCount int) []limitNameWithAD {
	var result []limitNameWithAD
	seen := make(map[string]int) // name → index in result
	var page *string

	for len(result) < maxCount {
		resp, err := c.Limits.ListLimitValues(ctx, limits.ListLimitValuesRequest{
			CompartmentId: common.String(compartmentID),
			ServiceName:   common.String(serviceName),
			Page:          page,
		})
		if err != nil {
			log.Warn().Err(err).Str("service", serviceName).Msg("ListLimitValues failed")
			break
		}
		for _, lv := range resp.Items {
			if lv.Name == nil || lv.Value == nil || *lv.Value <= 0 {
				continue
			}
			name := *lv.Name
			isAD := lv.AvailabilityDomain != nil && *lv.AvailabilityDomain != ""
			if idx, exists := seen[name]; exists {
				// Already seen — update AD info if this is AD-level.
				if isAD {
					result[idx].isADLevel = true
					if result[idx].adNames == nil {
						result[idx].adNames = make(map[string]bool)
					}
					result[idx].adNames[*lv.AvailabilityDomain] = true
				}
			} else {
				entry := limitNameWithAD{name: name, isADLevel: isAD}
				if isAD {
					entry.adNames = map[string]bool{*lv.AvailabilityDomain: true}
				}
				seen[name] = len(result)
				result = append(result, entry)
			}
			if len(result) >= maxCount {
				break
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return result
}

// getAggregatedAvailability sums availability across all ADs for AD-level limits.
func getAggregatedAvailability(ctx context.Context, c Clients, compartmentID, serviceName, limitName string) (total, used, available int64, err error) {
	// Get all ADs for the compartment.
	ads, err := c.Identity.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("list availability domains: %w", err)
	}

	var totalUsed, totalAvail int64
	for _, ad := range ads.Items {
		if ad.Name == nil {
			continue
		}
		u, a, e := getResourceAvailability(ctx, c, compartmentID, serviceName, limitName, ad.Name)
		if e != nil {
			log.Warn().Err(e).Str("ad", *ad.Name).Str("limit", limitName).Msg("getResourceAvailability for AD failed, skipping")
			continue
		}
		totalUsed += u
		totalAvail += a
	}
	return totalUsed + totalAvail, totalUsed, totalAvail, nil
}

// getResourceAvailability returns availability for a single limit.
func getResourceAvailability(ctx context.Context, c Clients, compartmentID, serviceName, limitName string, adName *string) (used, available int64, err error) {
	req := limits.GetResourceAvailabilityRequest{
		CompartmentId: common.String(compartmentID),
		ServiceName:   common.String(serviceName),
		LimitName:     common.String(limitName),
	}
	if adName != nil {
		req.AvailabilityDomain = common.String(*adName)
	}
	resp, err := c.Limits.GetResourceAvailability(ctx, req)
	if err != nil {
		return 0, 0, fmt.Errorf("GetResourceAvailability(%s/%s): %w", serviceName, limitName, err)
	}
	if resp.Used != nil {
		used = *resp.Used
	}
	if resp.Available != nil {
		available = *resp.Available
	}
	return used, available, nil
}

// ListLimitServices returns all services that support limits management.
func ListLimitServices(ctx context.Context, c Clients, compartmentID string) ([]limits.ServiceSummary, error) {
	resp, err := c.Limits.ListServices(ctx, limits.ListServicesRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	return resp.Items, nil
}

// HasEnoughResource checks if the tenant has enough quota for a requested amount.
func HasEnoughResource(ctx context.Context, c Clients, compartmentID, serviceName, limitName string, required int64) (bool, error) {
	used, available, err := getResourceAvailability(ctx, c, compartmentID, serviceName, limitName, nil)
	if err != nil {
		return false, err
	}
	_ = used
	return available >= required, nil
}
