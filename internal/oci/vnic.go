// Package oci — vnic.go: VNIC management operations (SPEC S10.5).
// Port of VnicManagementUtils.java. Lists VNIC attachments, gets VNIC
// details (public/private IP, IPv6), enumerates VNICs for a tenant.
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// VnicAttachmentInfo holds the parsed VNIC attachment result.
type VnicAttachmentInfo struct {
	VnicID       string
	InstanceID   string
	InstanceName string
	PublicIP     string
	PrivateIP    string
	Ipv6Addresses []string
	SubnetID     string
	VlanTag      *int
}

// ListVnicAttachmentsForInstance lists all VNIC attachments for an instance.
func ListVnicAttachmentsForInstance(ctx context.Context, computeClient *core.ComputeClient, compartmentID, instanceID string) ([]core.VnicAttachment, error) {
	var out []core.VnicAttachment
	var page *string
	for {
		resp, err := computeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
			CompartmentId: common.String(compartmentID),
			InstanceId:    common.String(instanceID),
			Limit:         common.Int(100),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("list vnic attachments for %s: %w", instanceID, err)
		}
		out = append(out, resp.Items...)
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return out, nil
}

// GetVnicInfo resolves a VNIC OCID to its IP details (public, private, IPv6).
func GetVnicInfo(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string) (*VnicAttachmentInfo, error) {
	resp, err := vcnClient.GetVnic(ctx, core.GetVnicRequest{
		VnicId: common.String(vnicID),
	})
	if err != nil {
		return nil, fmt.Errorf("get vnic %s: %w", vnicID, err)
	}
	v := resp.Vnic
	info := &VnicAttachmentInfo{
		VnicID: vnicID,
	}
	if v.PublicIp != nil {
		info.PublicIP = *v.PublicIp
	}
	if v.PrivateIp != nil {
		info.PrivateIP = *v.PrivateIp
	}
	if v.SubnetId != nil {
		info.SubnetID = *v.SubnetId
	}
	info.Ipv6Addresses = v.Ipv6Addresses
	return info, nil
}

// ListAllVnicsForInstance resolves all VNIC attachments for an instance and
// returns them with full VNIC info (IPs, IPv6). Parity with
// VnicManagementUtils.listAllVnicsForInstance.
func ListAllVnicsForInstance(ctx context.Context, computeClient *core.ComputeClient, vcnClient *core.VirtualNetworkClient, compartmentID, instanceID string) ([]VnicAttachmentInfo, error) {
	attachments, err := ListVnicAttachmentsForInstance(ctx, computeClient, compartmentID, instanceID)
	if err != nil {
		return nil, err
	}
	var out []VnicAttachmentInfo
	for _, att := range attachments {
		if att.VnicId == nil {
			continue
		}
		info, err := GetVnicInfo(ctx, vcnClient, *att.VnicId)
		if err != nil {
			continue // per-VNIC error non-fatal
		}
		info.InstanceID = instanceID
		if att.DisplayName != nil {
			info.InstanceName = *att.DisplayName
		}
		out = append(out, *info)
	}
	return out, nil
}

// AssignIpv6ToVnic assigns an IPv6 address to a VNIC. If forceNew is true, it
// unassigns all existing IPv6 addresses first. Returns the new/current IPv6 address.
// Port of OciIpv6Utils.enableOrRefreshVnicIpv6.
func AssignIpv6ToVnic(ctx context.Context, vcnClient *core.VirtualNetworkClient, vnicID string, forceNew bool) (string, error) {
	if forceNew {
		// List and detach existing IPv6 addresses from the VNIC
		var page *string
		for {
			resp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
				VnicId: common.String(vnicID),
				Page:   page,
			})
			if err != nil {
				return "", fmt.Errorf("list ipv6s for vnic %s: %w", vnicID, err)
			}
			for _, ipv6 := range resp.Items {
				if ipv6.Id != nil {
					_, err := vcnClient.Ipv6VnicDetach(ctx, core.Ipv6VnicDetachRequest{
						Ipv6Id: ipv6.Id,
					})
					if err != nil {
						// Non-fatal per-address error
						continue
					}
				}
			}
			if resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}
	}

	// Assign a new IPv6
	resp, err := vcnClient.CreateIpv6(ctx, core.CreateIpv6Request{
		CreateIpv6Details: core.CreateIpv6Details{
			VnicId: common.String(vnicID),
		},
	})
	if err != nil {
		return "", fmt.Errorf("assign ipv6 to vnic %s: %w", vnicID, err)
	}
	if resp.IpAddress != nil {
		return *resp.IpAddress, nil
	}
	// Fallback: list to find assigned IPv6
	listResp, err := vcnClient.ListIpv6s(ctx, core.ListIpv6sRequest{
		VnicId: common.String(vnicID),
	})
	if err != nil {
		return "", fmt.Errorf("list ipv6s after assign: %w", err)
	}
	if len(listResp.Items) > 0 && listResp.Items[0].IpAddress != nil {
		return *listResp.Items[0].IpAddress, nil
	}
	return "", fmt.Errorf("no ipv6 address assigned")
}
