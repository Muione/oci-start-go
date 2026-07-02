// Package oci — usage.go: OCI Usage/Cost API wrapper (Phase B).
// Parity with Java UsageApiUtils: query cost data with various time ranges.
package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/usageapi"
)

// CostSummary holds a single cost data point from the Usage API.
type CostSummary struct {
	TimeUsageStarted *common.SDKTime `json:"timeUsageStarted"`
	TimeUsageEnded   *common.SDKTime `json:"timeUsageEnded"`
	Service          string          `json:"service"`
	ResourceName     string          `json:"resourceName"`
	ComputedAmount   float32         `json:"computedAmount"`
	ComputedQuantity float32         `json:"computedQuantity"`
	Currency         string          `json:"currency"`
	SkuName          string          `json:"skuName"`
	Region           string          `json:"region"`
}

// QueryCost queries OCI Usage/Cost API with the given parameters.
func QueryCost(ctx context.Context, c Clients, tenancyOCID string, startUTC, endUTC time.Time, groupBy []string, granularity usageapi.RequestSummarizedUsagesDetailsGranularityEnum) ([]CostSummary, error) {
	details := usageapi.RequestSummarizedUsagesDetails{
		TenantId:        common.String(tenancyOCID),
		TimeUsageStarted: &common.SDKTime{Time: startUTC},
		TimeUsageEnded:   &common.SDKTime{Time: endUTC},
		Granularity:      granularity,
	}
	if len(groupBy) > 0 {
		details.GroupBy = groupBy
	}

	resp, err := c.UsageApi.RequestSummarizedUsages(ctx, usageapi.RequestSummarizedUsagesRequest{
		RequestSummarizedUsagesDetails: details,
	})
	if err != nil {
		return nil, fmt.Errorf("request summarized usages: %w", err)
	}

	out := make([]CostSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		cs := CostSummary{
			TimeUsageStarted: item.TimeUsageStarted,
			TimeUsageEnded:   item.TimeUsageEnded,
			Service:          derefStr(item.Service),
			ResourceName:     derefStr(item.ResourceName),
			Currency:         derefStr(item.Currency),
			SkuName:          derefStr(item.SkuName),
			Region:           derefStr(item.Region),
		}
		if item.ComputedAmount != nil {
			cs.ComputedAmount = derefFloat32(item.ComputedAmount)
		}
		if item.ComputedQuantity != nil {
			cs.ComputedQuantity = derefFloat32(item.ComputedQuantity)
		}
		out = append(out, cs)
	}
	return out, nil
}

// QueryTodayCost returns DAILY cost for today (00:00 to start of tomorrow UTC).
// OCI Usage API requires all dates to have zero hours/minutes/seconds.
func QueryTodayCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityDaily)
}

// QueryYesterdayCost returns DAILY cost for yesterday (00:00 to 00:00 UTC).
func QueryYesterdayCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityDaily)
}

// QueryCurrentMonthCost returns MONTHLY cost from the 1st of the current month to the 1st of next month UTC.
// OCI Usage API requires all dates to have zero hours/minutes/seconds.
func QueryCurrentMonthCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityMonthly)
}

// QueryLastMonthCost returns MONTHLY cost for the entire previous month.
func QueryLastMonthCost(ctx context.Context, c Clients, tenancyOCID string) ([]CostSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start, end, groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityMonthly)
}

// QueryCustomCost queries cost for a custom date range. Dates should be in "2006-01-02" format.
func QueryCustomCost(ctx context.Context, c Clients, tenancyOCID string, startStr, endStr string) ([]CostSummary, error) {
	const dateFmt = "2006-01-02"
	start, err := time.Parse(dateFmt, startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start date %q: %w", startStr, err)
	}
	end, err := time.Parse(dateFmt, endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end date %q: %w", endStr, err)
	}
	groupBy := []string{"service", "skuName", "region"}
	return QueryCost(ctx, c, tenancyOCID, start.UTC(), end.UTC(), groupBy, usageapi.RequestSummarizedUsagesDetailsGranularityDaily)
}
