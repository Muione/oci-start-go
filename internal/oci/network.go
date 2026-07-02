// Package oci — network.go: basic VCN/VNIC domain (Phase 3) + Phase 11.2
// network configuration (NAT gateway, route tables, NLB orchestration).
// Ports OciUtils.getVnicPrimary and OciNetworkUtils.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// GetPrimaryVnic returns the primary VNIC of an instance (or the first
// attachment if none is marked primary). Parity with OciUtils.getVnicPrimary.
func GetPrimaryVnic(ctx context.Context, c Clients, instanceID, compartmentID string) (core.Vnic, error) {
	resp, err := c.Compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
	})
	if err != nil {
		return core.Vnic{}, fmt.Errorf("list vnic attachments: %w", err)
	}
	return getPrimaryVnicFromAttachments(ctx, resp.Items, instanceID, func(ctx context.Context, vnicID *string) (core.Vnic, error) {
		vr, err := c.Vcn.GetVnic(ctx, core.GetVnicRequest{VnicId: vnicID})
		if err != nil {
			return core.Vnic{}, err
		}
		return vr.Vnic, nil
	})
}

// getPrimaryVnicFromAttachments resolves the primary VNIC from a list of VNIC
// attachments. getVnic is injected so the helper is unit-testable without a
// live OCI client. Parity with OciUtils.getVnicPrimary.
//
// Single pass: each attachment's Vnic is fetched at most once. The first
// reachable Vnic is remembered so the fallback reuses it instead of re-calling
// GetVnic (P-3: previously the fallback re-iterated and re-fetched every
// attachment, making GetVnic O(2N) when no Vnic is marked primary).
func getPrimaryVnicFromAttachments(ctx context.Context, attachments []core.VnicAttachment, instanceID string, getVnic func(ctx context.Context, vnicID *string) (core.Vnic, error)) (core.Vnic, error) {
	var firstReachable *core.Vnic
	for _, a := range attachments {
		if a.VnicId == nil {
			continue
		}
		vr, err := getVnic(ctx, a.VnicId)
		if err != nil {
			continue
		}
		if firstReachable == nil {
			v := vr
			firstReachable = &v
		}
		if vr.IsPrimary != nil && *vr.IsPrimary {
			return vr, nil
		}
	}
	if firstReachable != nil {
		return *firstReachable, nil
	}
	return core.Vnic{}, fmt.Errorf("no vnic for instance %s", instanceID)
}

// ListVcns lists VCNs in a compartment (basic domain helper).
func ListVcns(ctx context.Context, c Clients, compartmentID string) ([]core.Vcn, error) {
	resp, err := c.Vcn.ListVcns(ctx, core.ListVcnsRequest{
		CompartmentId: common.String(compartmentID),
	})
	if err != nil {
		return nil, fmt.Errorf("list vcns: %w", err)
	}
	return resp.Items, nil
}

// ReassignPublicIP assigns a new ephemeral public IP to an instance by deleting
// the old reserved public IP and creating a new one on the primary VNIC's private IP.
// Returns the new public IP address. Port of OracleInstanceServiceImpl.changePublicIp.
func ReassignPublicIP(ctx context.Context, c Clients, compartmentID, instanceID string) (string, error) {
	// 1. Get the primary VNIC
	vnic, err := GetPrimaryVnic(ctx, c, instanceID, compartmentID)
	if err != nil {
		return "", fmt.Errorf("get primary vnic: %w", err)
	}

	oldPublicIP := ""
	if vnic.PublicIp != nil {
		oldPublicIP = *vnic.PublicIp
	}

	// 2. Find and delete the old reserved public IP
	if oldPublicIP != "" {
		var page *string
		for {
			resp, err := c.Vcn.ListPublicIps(ctx, core.ListPublicIpsRequest{
				CompartmentId: common.String(compartmentID),
				Scope:         core.ListPublicIpsScopeRegion,
				Page:          page,
			})
			if err != nil {
				return "", fmt.Errorf("list public ips: %w", err)
			}
			for _, ip := range resp.Items {
				if ip.IpAddress != nil && *ip.IpAddress == oldPublicIP {
					_, err := c.Vcn.DeletePublicIp(ctx, core.DeletePublicIpRequest{
						PublicIpId: ip.Id,
					})
					if err != nil {
						return "", fmt.Errorf("delete old public ip %s: %w", oldPublicIP, err)
					}
					break
				}
			}
			if resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}
	}

	// 3. Get the private IP ID
	if vnic.PrivateIp == nil || *vnic.PrivateIp == "" {
		return "", fmt.Errorf("vnic has no private ip")
	}
	privateIP := *vnic.PrivateIp

	// Find the private IP OCID
	var privateIPID string
	{
		var page *string
		for {
			resp, err := c.Vcn.ListPrivateIps(ctx, core.ListPrivateIpsRequest{
				VnicId: vnic.Id,
				Page:   page,
			})
			if err != nil {
				return "", fmt.Errorf("list private ips: %w", err)
			}
			for _, pip := range resp.Items {
				if pip.IpAddress != nil && *pip.IpAddress == privateIP {
					privateIPID = *pip.Id
					break
				}
			}
			if privateIPID != "" || resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}
	}
	if privateIPID == "" {
		return "", fmt.Errorf("private ip ocid not found for %s", privateIP)
	}

	// 4. Create a new reserved public IP
	createResp, err := c.Vcn.CreatePublicIp(ctx, core.CreatePublicIpRequest{
		CreatePublicIpDetails: core.CreatePublicIpDetails{
			CompartmentId: common.String(compartmentID),
			Lifetime:      core.CreatePublicIpDetailsLifetimeReserved,
			PrivateIpId:   common.String(privateIPID),
		},
	})
	if err != nil {
		return "", fmt.Errorf("create public ip: %w", err)
	}
	if createResp.PublicIp.IpAddress == nil {
		return "", fmt.Errorf("created public ip has nil address")
	}
	return *createResp.PublicIp.IpAddress, nil
}

// ---------------------------------------------------------------------------
// Phase 11.2: Network configuration (NAT gateway, route tables, NLB setup)
// ---------------------------------------------------------------------------

// NetworkConfigResult mirrors Java OciNetworkUtils.NetworkConfigResult.
type NetworkConfigResult struct {
	Success                 bool     `json:"success"`
	Message                 string   `json:"message,omitempty"`
	ErrorMessage            string   `json:"errorMessage,omitempty"`
	NatGatewayID            string   `json:"natGatewayId,omitempty"`
	NatGatewayName          string   `json:"natGatewayName,omitempty"`
	RouteTableID            string   `json:"routeTableId,omitempty"`
	RouteTableName          string   `json:"routeTableName,omitempty"`
	RouteTableUpdated       bool     `json:"routeTableUpdated"`
	LoadBalancerCreated     bool     `json:"loadBalancerCreated"`
	NetworkLoadBalancerID   string   `json:"networkLoadBalancerId,omitempty"`
	NetworkLoadBalancerName string   `json:"networkLoadBalancerName,omitempty"`
	NlbIPAddress            string   `json:"nlpIpAddress,omitempty"`
	IPAddresses             []string `json:"ipAddresses,omitempty"`
}

// CreateOrGetNatGateway finds or creates a NAT gateway by display name.
func CreateOrGetNatGateway(ctx context.Context, vcnClient *core.VirtualNetworkClient, compartmentID, vcnID, displayName string) (*core.NatGateway, error) {
	// Try to find existing.
	var page *string
	for {
		resp, err := vcnClient.ListNatGateways(ctx, core.ListNatGatewaysRequest{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list nat gateways: %w", err)
		}
		for _, gw := range resp.Items {
			if gw.DisplayName != nil && *gw.DisplayName == displayName {
				gwCopy := gw
				return &gwCopy, nil
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	// Create new.
	createResp, err := vcnClient.CreateNatGateway(ctx, core.CreateNatGatewayRequest{
		CreateNatGatewayDetails: core.CreateNatGatewayDetails{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			DisplayName:   common.String(displayName),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create nat gateway: %w", err)
	}
	return &createResp.NatGateway, nil
}

// CreateOrGetNatRouteTable finds or creates a route table with NAT gateway route.
func CreateOrGetNatRouteTable(ctx context.Context, vcnClient *core.VirtualNetworkClient, compartmentID, vcnID, natGatewayID, displayName string) (*core.RouteTable, error) {
	// Try to find existing.
	var page *string
	for {
		resp, err := vcnClient.ListRouteTables(ctx, core.ListRouteTablesRequest{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list route tables: %w", err)
		}
		for _, rt := range resp.Items {
			if rt.DisplayName != nil && *rt.DisplayName == displayName {
				rtCopy := rt
				return &rtCopy, nil
			}
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	// Create new route table with NAT gateway route.
	createResp, err := vcnClient.CreateRouteTable(ctx, core.CreateRouteTableRequest{
		CreateRouteTableDetails: core.CreateRouteTableDetails{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			DisplayName:   common.String(displayName),
			RouteRules: []core.RouteRule{
				{
					Destination:     common.String("0.0.0.0/0"),
					DestinationType: core.RouteRuleDestinationTypeCidrBlock,
					NetworkEntityId: common.String(natGatewayID),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create route table: %w", err)
	}
	return &createResp.RouteTable, nil
}

// UpdateInstanceVnicRouteTable updates the primary VNIC's route table.
func UpdateInstanceVnicRouteTable(ctx context.Context, c Clients, instanceID, compartmentID, routeTableID string) error {
	vnic, err := GetPrimaryVnic(ctx, c, instanceID, compartmentID)
	if err != nil {
		return fmt.Errorf("get primary vnic: %w", err)
	}
	if vnic.Id == nil {
		return fmt.Errorf("primary vnic has nil id")
	}
	_, err = c.Vcn.UpdateVnic(ctx, core.UpdateVnicRequest{
		VnicId: vnic.Id,
		UpdateVnicDetails: core.UpdateVnicDetails{
			RouteTableId: common.String(routeTableID),
		},
	})
	if err != nil {
		return fmt.Errorf("update vnic route table: %w", err)
	}
	return nil
}

// ResetVnicToDefaultRouteTable resets a VNIC's route table to the VCN default.
func ResetVnicToDefaultRouteTable(ctx context.Context, c Clients, instanceID, compartmentID string) error {
	// Find the VCN default route table.
	vnic, err := GetPrimaryVnic(ctx, c, instanceID, compartmentID)
	if err != nil {
		return fmt.Errorf("get primary vnic: %w", err)
	}
	if vnic.SubnetId == nil {
		return fmt.Errorf("primary vnic has no subnet")
	}

	// Get the subnet to find the VCN.
	subnetResp, err := c.Vcn.GetSubnet(ctx, core.GetSubnetRequest{
		SubnetId: vnic.SubnetId,
	})
	if err != nil {
		return fmt.Errorf("get subnet: %w", err)
	}
	vcnID := *subnetResp.Subnet.VcnId

	// List route tables and find the default one (not named "amd").
	var page *string
	var defaultRT *core.RouteTable
	for {
		resp, err := c.Vcn.ListRouteTables(ctx, core.ListRouteTablesRequest{
			CompartmentId: common.String(compartmentID),
			VcnId:         common.String(vcnID),
			Page:          page,
		})
		if err != nil {
			return fmt.Errorf("list route tables: %w", err)
		}
		for _, rt := range resp.Items {
			if rt.DisplayName == nil || *rt.DisplayName != "amd" {
				rtCopy := rt
				defaultRT = &rtCopy
				break
			}
		}
		if defaultRT != nil || resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	if defaultRT == nil {
		return fmt.Errorf("no default route table found")
	}

	// Update the VNIC's route table.
	if vnic.Id == nil {
		return fmt.Errorf("primary vnic has nil id")
	}
	_, err = c.Vcn.UpdateVnic(ctx, core.UpdateVnicRequest{
		VnicId: vnic.Id,
		UpdateVnicDetails: core.UpdateVnicDetails{
			RouteTableId: defaultRT.Id,
		},
	})
	if err != nil {
		return fmt.Errorf("update vnic route table: %w", err)
	}
	return nil
}

// DeleteNatGateway deletes a NAT gateway by ID.
func DeleteNatGateway(ctx context.Context, vcnClient *core.VirtualNetworkClient, natGatewayID string) error {
	_, err := vcnClient.DeleteNatGateway(ctx, core.DeleteNatGatewayRequest{
		NatGatewayId: common.String(natGatewayID),
	})
	if err != nil {
		return fmt.Errorf("delete nat gateway %s: %w", natGatewayID, err)
	}
	return nil
}

// DeleteRouteTable deletes a route table by ID.
func DeleteRouteTable(ctx context.Context, vcnClient *core.VirtualNetworkClient, routeTableID string) error {
	_, err := vcnClient.DeleteRouteTable(ctx, core.DeleteRouteTableRequest{
		RtId: common.String(routeTableID),
	})
	if err != nil {
		return fmt.Errorf("delete route table %s: %w", routeTableID, err)
	}
	return nil
}
