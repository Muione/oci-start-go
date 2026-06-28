// Package oci — traffic.go: OCI Monitoring client for instance traffic
// (SPEC S10.4). Port of TrafficMetricsUtils.java + InstanceTrafficTask.java.
// Uses monitoring.MonitoringClient to query oci_vcn namespace metrics
// (VnicFromNetworkBytes / VnicToNetworkBytes).
package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/monitoring"
)

// TrafficPeriod defines the aggregation window for traffic metric queries.
type TrafficPeriod string

const (
	TrafficPeriodOneHour  TrafficPeriod = "1h"
	TrafficPeriodOneDay   TrafficPeriod = "1d"
	TrafficPeriodOneMonth TrafficPeriod = "1M"
)

// VnicInfo holds the minimal VNIC data needed for traffic queries.
type VnicInfo struct {
	VnicID       string
	InstanceID   string
	InstanceName string
	PublicIP     string
}

// BuildMonitoringClient creates a MonitoringClient from a configuration provider.
// Parity with TrafficMetricsUtils.buildClient.
func BuildMonitoringClient(prov common.ConfigurationProvider) (*monitoring.MonitoringClient, error) {
	client, err := monitoring.NewMonitoringClientWithConfigurationProvider(prov)
	if err != nil {
		return nil, fmt.Errorf("monitoring client: %w", err)
	}
	return &client, nil
}

// GetInstanceTrafficTotal queries the total ingress or egress bytes for a set
// of VNICs over the given time range. Parity with TrafficMetricsUtils.
// getInstanceTrafficTotal.
func GetInstanceTrafficTotal(
	ctx context.Context,
	client *monitoring.MonitoringClient,
	compartmentID string,
	vnics []VnicInfo,
	ingress bool,
	startTime, endTime time.Time,
	period TrafficPeriod,
) (float64, error) {
	if len(vnics) == 0 {
		return 0, nil
	}

	metricName := "VnicFromNetworkBytes" // egress
	if ingress {
		metricName = "VnicToNetworkBytes"
	}

	// Map period to a MQL-compatible interval string for the range selector.
	interval := "1h"
	switch period {
	case TrafficPeriodOneHour:
		interval = "5m"
	case TrafficPeriodOneDay:
		interval = "1h"
	case TrafficPeriodOneMonth:
		interval = "1d"
	}

	var total float64
	for _, vnic := range vnics {
		if vnic.VnicID == "" {
			continue
		}
		val, err := queryVnicTraffic(ctx, client, compartmentID, vnic.VnicID, metricName, startTime, endTime, period, interval)
		if err != nil {
			continue // per-VNIC failure non-fatal
		}
		total += val
	}
	return total, nil
}

// queryVnicTraffic queries a single metric for one VNIC. Uses
// SummarizeMetricsData with the oci_vcn namespace.
func queryVnicTraffic(
	ctx context.Context,
	client *monitoring.MonitoringClient,
	compartmentID, vnicID, metricName string,
	startTime, endTime time.Time,
	period TrafficPeriod,
	interval string,
) (float64, error) {
	req := monitoring.SummarizeMetricsDataRequest{
		CompartmentId: common.String(compartmentID),
		SummarizeMetricsDataDetails: monitoring.SummarizeMetricsDataDetails{
			Namespace: common.String("oci_vcn"),
			Query:     common.String(fmt.Sprintf("%s[%s]{%s}.sum()", metricName, interval, `resourceId="`+vnicID+`"`)),
			StartTime: &common.SDKTime{Time: startTime},
			EndTime:   &common.SDKTime{Time: endTime},
			Resolution: common.String(string(period)),
		},
	}

	resp, err := client.SummarizeMetricsData(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("summarize metrics for %s: %w", vnicID, err)
	}

	var total float64
	for _, item := range resp.Items {
		if item.AggregatedDatapoints != nil {
			for _, dp := range item.AggregatedDatapoints {
				if dp.Value != nil {
					total += *dp.Value
				}
			}
		}
	}
	return total, nil
}

// BytesToGB converts bytes to gigabytes.
func BytesToGB(bytes float64) float64 {
	return bytes / (1024.0 * 1024.0 * 1024.0)
}
