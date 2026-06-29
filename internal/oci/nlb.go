// Package oci — nlb.go: Network Load Balancer operations (Phase 11.2).
// NLB create/delete/list/wait for the VNIC management load balancer feature.
// Uses github.com/oracle/oci-go-sdk/v65/networkloadbalancer.
package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

// CreateOrGetNetworkLoadBalancer finds or creates an NLB with backend set + listener.
// Parity with Java OciNetworkUtils.createNetworkLoadBalancer.
//
// Configuration:
//   - Backend set "amd-1": targetId = instance OCID, port 22
//   - Health check: TCP port 22, interval 10s, timeout 3s, retries 3
//   - Listener "amd": TCP+UDP port 22
//   - Policy: FiveTuple
func CreateOrGetNetworkLoadBalancer(ctx context.Context,
	nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
	compartmentID, subnetID, instanceID, displayName, privateIP string,
) (*networkloadbalancer.NetworkLoadBalancer, error) {
	// Try to find existing NLB with this display name.
	existing, err := findNLBByDisplayName(ctx, nlbClient, compartmentID, displayName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// Create NLB with backend set and listener inline.
	port22 := 22
	createResp, err := nlbClient.CreateNetworkLoadBalancer(ctx, networkloadbalancer.CreateNetworkLoadBalancerRequest{
		CreateNetworkLoadBalancerDetails: networkloadbalancer.CreateNetworkLoadBalancerDetails{
			CompartmentId: common.String(compartmentID),
			DisplayName:   common.String(displayName),
			SubnetId:      common.String(subnetID),
			IsPrivate:     common.Bool(false),
			BackendSets: map[string]networkloadbalancer.BackendSetDetails{
				"amd-1": {
					Policy: networkloadbalancer.NetworkLoadBalancingPolicyFiveTuple,
					HealthChecker: &networkloadbalancer.HealthChecker{
						Protocol:        networkloadbalancer.HealthCheckProtocolsTcp,
						Port:            common.Int(port22),
						IntervalInMillis: common.Int(10000),
						TimeoutInMillis: common.Int(3000),
						Retries:         common.Int(3),
					},
					Backends: []networkloadbalancer.Backend{
						{
							TargetId: common.String(instanceID),
							Port:     common.Int(port22),
							Weight:   common.Int(1),
						},
					},
				},
			},
			Listeners: map[string]networkloadbalancer.ListenerDetails{
				"amd": {
					Name:                 common.String("amd"),
					DefaultBackendSetName: common.String("amd-1"),
					Port:                 common.Int(port22),
					Protocol:             networkloadbalancer.ListenerProtocolsTcpAndUdp,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create NLB: %w", err)
	}

	// Wait for NLB to become ACTIVE.
	nlb, err := WaitForNLBCreation(ctx, nlbClient, *createResp.Id, 300*time.Second, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wait for NLB creation: %w", err)
	}
	return nlb, nil
}

// DeleteNetworkLoadBalancer deletes an NLB by OCID.
func DeleteNetworkLoadBalancer(ctx context.Context,
	nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
	nlbID string,
) error {
	_, err := nlbClient.DeleteNetworkLoadBalancer(ctx, networkloadbalancer.DeleteNetworkLoadBalancerRequest{
		NetworkLoadBalancerId: common.String(nlbID),
	})
	if err != nil {
		return fmt.Errorf("delete NLB %s: %w", nlbID, err)
	}
	return nil
}

// NetworkLoadBalancerSummary is a lightweight wrapper for list results.
type NetworkLoadBalancerSummary struct {
	ID          string
	DisplayName string
	SubnetID    string
}

// ListNetworkLoadBalancers lists all NLBs in a compartment (returns summaries).
func ListNetworkLoadBalancers(ctx context.Context,
	nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
	compartmentID string,
) ([]NetworkLoadBalancerSummary, error) {
	var out []NetworkLoadBalancerSummary
	var page *string
	for {
		resp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
			CompartmentId: common.String(compartmentID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list NLBs: %w", err)
		}
		for _, item := range resp.Items {
			s := NetworkLoadBalancerSummary{}
			if item.Id != nil {
				s.ID = *item.Id
			}
			if item.DisplayName != nil {
				s.DisplayName = *item.DisplayName
			}
			if item.SubnetId != nil {
				s.SubnetID = *item.SubnetId
			}
			out = append(out, s)
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// WaitForNLBCreation polls until the NLB is ACTIVE.
func WaitForNLBCreation(ctx context.Context,
	nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
	nlbID string, timeout, interval time.Duration,
) (*networkloadbalancer.NetworkLoadBalancer, error) {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
			NetworkLoadBalancerId: common.String(nlbID),
		})
		if err != nil {
			return nil, fmt.Errorf("get NLB %s: %w", nlbID, err)
		}
		if resp.LifecycleState == networkloadbalancer.LifecycleStateActive {
			return &resp.NetworkLoadBalancer, nil
		}
		if resp.LifecycleState == networkloadbalancer.LifecycleStateFailed {
			return nil, fmt.Errorf("NLB %s entered FAILED state", nlbID)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for NLB %s (state: %s)", nlbID, resp.LifecycleState)
		}
		time.Sleep(interval)
	}
}

// findNLBByDisplayName searches for an NLB with the given display name in a compartment
// and returns the full object via GetNetworkLoadBalancer.
func findNLBByDisplayName(ctx context.Context,
	nlbClient *networkloadbalancer.NetworkLoadBalancerClient,
	compartmentID, displayName string,
) (*networkloadbalancer.NetworkLoadBalancer, error) {
	var page *string
	for {
		resp, err := nlbClient.ListNetworkLoadBalancers(ctx, networkloadbalancer.ListNetworkLoadBalancersRequest{
			CompartmentId: common.String(compartmentID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list NLBs: %w", err)
		}
		for _, nlb := range resp.Items {
			if nlb.DisplayName != nil && *nlb.DisplayName == displayName && nlb.Id != nil {
				// Fetch full NLB object.
				getResp, err := nlbClient.GetNetworkLoadBalancer(ctx, networkloadbalancer.GetNetworkLoadBalancerRequest{
					NetworkLoadBalancerId: nlb.Id,
				})
				if err != nil {
					return nil, fmt.Errorf("get NLB %s: %w", *nlb.Id, err)
				}
				return &getResp.NetworkLoadBalancer, nil
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return nil, nil
}
