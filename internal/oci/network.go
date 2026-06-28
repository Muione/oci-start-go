// Package oci — network.go: basic VCN/VNIC domain (Phase 3). Ports
// OciUtils.getVnicPrimary (primary VNIC → public/private IP + IPv6) and
// listVcns. VNIC batch-create, IPv6 enable, public-IP reassignment are later
// phases (SPEC §10.5).
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
	// First pass: the primary VNIC.
	for _, a := range resp.Items {
		if a.VnicId == nil {
			continue
		}
		vr, err := c.Vcn.GetVnic(ctx, core.GetVnicRequest{VnicId: a.VnicId})
		if err != nil {
			continue
		}
		if vr.Vnic.IsPrimary != nil && *vr.Vnic.IsPrimary {
			return vr.Vnic, nil
		}
	}
	// Fallback: first attachment with a reachable Vnic.
	for _, a := range resp.Items {
		if a.VnicId == nil {
			continue
		}
		vr, err := c.Vcn.GetVnic(ctx, core.GetVnicRequest{VnicId: a.VnicId})
		if err == nil {
			return vr.Vnic, nil
		}
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
